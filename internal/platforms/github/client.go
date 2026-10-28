package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	gh "github.com/google/go-github/v93/github"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
)

// Client publishes GitHub releases over the REST API through go-github (ADR-0077).
type Client struct {
	api         *gh.Client
	baseURL     string
	apiURL      string
	token       string
	httpClient  *http.Client
	owner, repo string
}

// responseHeaderTimeout bounds the wait for GitHub's answer once a request is sent: a connection that
// stalls after the tag push must not hang the release. It does not bound the request itself, so a
// large upload still completes. cleanupTimeout bounds the delete of a draft left by a failed publish.
var (
	responseHeaderTimeout = 2 * time.Minute
	cleanupTimeout        = 30 * time.Second
)

// NewClient builds a client for repository ("owner/repo") on the GitHub instance whose web root is
// baseURL (empty means github.com). The API root comes from config.GitHubAPIURL, which reads
// GITHUB_API_URL through getenv in GitHub Actions.
func NewClient(baseURL, repository, token string, getenv func(string) string) (*Client, error) {
	return newClient(baseURL, repository, token, getenv, nil)
}

func newClient(baseURL, repository, token string, getenv func(string) string, httpClient *http.Client) (*Client, error) {
	owner, repo, err := splitRepository(repository)
	if err != nil {
		return nil, err
	}
	if baseURL == "" {
		baseURL = config.DefaultGitHubBaseURL
	}
	apiURL, err := config.GitHubAPIURL(baseURL, getenv)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.ResponseHeaderTimeout = responseHeaderTimeout
		httpClient = &http.Client{Transport: tr}
	}
	c := &Client{baseURL: baseURL, apiURL: apiURL + "/", token: token, httpClient: httpClient, owner: owner, repo: repo}
	if c.api, err = c.newAPI(nil); err != nil {
		return nil, err
	}
	return c, nil
}

// newAPI builds a go-github client that sends the token only to the API origin and uploadOrigin
// (go-github's default, uploads.github.com, when nil).
func (c *Client) newAPI(uploadOrigin *string) (*gh.Client, error) {
	api, err := gh.NewClient(gh.WithHTTPClient(c.httpClient), gh.WithAuthToken(c.token), gh.WithURLs(&c.apiURL, uploadOrigin))
	if err != nil {
		return nil, fmt.Errorf("creating the GitHub client for %s: %w", c.apiURL, err)
	}
	return api, nil
}

// CreateRelease publishes the release for tag and returns the names GitHub gave the uploaded
// assets, which can differ from the file names (GitHub rewrites special characters). Without
// assets it is one create call. With assets the release is created as a draft, each file is
// uploaded, then the draft is published unless draft is set: uploading to a published release
// fails when immutable releases are on. If an upload or the publish fails, the draft is deleted so
// a re-run starts clean; a failed delete is appended to the original error. ctx stops the requests
// (Ctrl-C); the delete runs on its own short context, so it still happens once ctx is cancelled.
func (c *Client) CreateRelease(ctx context.Context, tag, notes string, draft, prerelease bool, assets []platforms.Asset) ([]string, error) {
	release, _, err := c.api.Repositories.CreateRelease(ctx, c.owner, c.repo, gh.CreateReleaseRequest{
		TagName:    tag,
		Name:       new(tag),
		Body:       new(notes),
		Draft:      new(draft || len(assets) > 0),
		Prerelease: new(prerelease),
	})
	if err != nil {
		return nil, fmt.Errorf("creating release %s: %w", tag, readable(err))
	}
	if len(assets) == 0 {
		return nil, nil
	}

	names, err := c.uploadAndPublish(ctx, release, tag, draft, assets)
	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		if _, delErr := c.api.Repositories.DeleteRelease(cleanupCtx, c.owner, c.repo, release.ID); delErr != nil {
			return nil, fmt.Errorf("%w; deleting the draft release %d also failed: %w", err, release.ID, readable(delErr))
		}
		return nil, err
	}
	return names, nil
}

func (c *Client) uploadAndPublish(ctx context.Context, release *gh.RepositoryRelease, tag string, draft bool, assets []platforms.Asset) ([]string, error) {
	origin, err := checkUploadURL(c.baseURL, release.UploadURL)
	if err != nil {
		return nil, err
	}
	uploader, err := c.newAPI(&origin)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(assets))
	for _, a := range assets {
		uploaded, _, err := uploader.Repositories.UploadReleaseAssetFromRelease(ctx, release, &gh.UploadOptions{Name: a.Name}, a.File, a.Size)
		if err != nil {
			return nil, fmt.Errorf("uploading asset %s: %w", a.Name, readable(err))
		}
		names = append(names, uploaded.GetName())
	}
	if draft {
		return names, nil
	}
	if _, _, err := c.api.Repositories.UpdateRelease(ctx, c.owner, c.repo, release.ID, gh.UpdateReleaseRequest{Draft: new(false)}); err != nil {
		return nil, fmt.Errorf("publishing release %s: %w", tag, readable(err))
	}
	return names, nil
}

// VerifyReleasesAccess checks that the token can read the repository's releases.
func (c *Client) VerifyReleasesAccess() error {
	if _, _, err := c.api.Repositories.ListReleases(context.Background(), c.owner, c.repo, &gh.ListOptions{PerPage: 1}); err != nil {
		return fmt.Errorf("listing releases of %s/%s: %w", c.owner, c.repo, readable(err))
	}
	return nil
}

// checkUploadURL accepts the upload_url GitHub returned only on an origin expected for baseURL:
// uploads.github.com for github.com, otherwise baseURL's own origin or uploads.<host> on its scheme
// and port. It returns that origin, with the trailing slash go-github wants, so the upload client
// is configured for it and nothing else.
func checkUploadURL(baseURL, uploadURL string) (string, error) {
	refused := fmt.Errorf("upload URL %q returned by GitHub is not on a host expected for %s: refusing to send the asset there", uploadURL, baseURL)
	u, err := url.Parse(uploadURL)
	if err != nil || u.Host == "" || u.User != nil {
		return "", refused
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", refused
	}
	allowed := []*url.URL{{Scheme: base.Scheme, Host: base.Host}, {Scheme: base.Scheme, Host: "uploads." + base.Host}}
	if strings.EqualFold(base.Hostname(), "github.com") {
		allowed = []*url.URL{{Scheme: "https", Host: "uploads.github.com"}}
	}
	for _, origin := range allowed {
		if config.SameOrigin(u, origin) {
			return u.Scheme + "://" + u.Host + "/", nil
		}
	}
	return "", refused
}

// apiError gives a go-github ErrorResponse a readable message: its own Error() prints the error
// list as Go structs. Unwrap keeps the original reachable through errors.As.
type apiError struct{ resp *gh.ErrorResponse }

func (e *apiError) Error() string {
	msg := fmt.Sprintf("%d %s", e.resp.Response.StatusCode, e.resp.Message)
	var details []string
	for _, d := range e.resp.Errors {
		switch {
		case d.Message != "":
			details = append(details, d.Message)
		case d.Field != "":
			details = append(details, d.Field+" "+d.Code)
		case d.Code != "":
			details = append(details, d.Code)
		}
	}
	if len(details) > 0 {
		msg += ": " + strings.Join(details, "; ")
	}
	return msg
}

func (e *apiError) Unwrap() error { return e.resp }

func readable(err error) error {
	var resp *gh.ErrorResponse
	if errors.As(err, &resp) && resp.Response != nil {
		return &apiError{resp: resp}
	}
	return err
}

// githubName matches a GitHub owner or repository name; it also keeps "/", "?" and "#" out of the
// request paths go-github builds from them.
var githubName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func splitRepository(repository string) (owner, repo string, err error) {
	if repository == "" {
		return "", "", fmt.Errorf("repository not set: configure repository: in .heraut.yml or set $%s", config.EnvGitHubRepository)
	}
	owner, repo, _ = strings.Cut(repository, "/")
	for _, part := range []string{owner, repo} {
		if !githubName.MatchString(part) || part == "." || part == ".." {
			return "", "", fmt.Errorf("repository %q is not of the form owner/repo", repository)
		}
	}
	return owner, repo, nil
}
