package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	gl "gitlab.com/gitlab-org/api/client-go"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
)

// assetPackageName is the Generic Packages package that holds every release asset.
const assetPackageName = "release-assets"

// Client publishes GitLab releases over the REST API (ADR-0069).
type Client struct {
	api *gl.Client
}

// NewClient builds a client for baseURL (instance root, optionally with a relative URL root).
// With jobToken the token is sent as JOB-TOKEN (CI_JOB_TOKEN), otherwise as PRIVATE-TOKEN.
func NewClient(baseURL, token string, jobToken bool) (*Client, error) {
	if !config.IsSecureURL(baseURL) {
		return nil, fmt.Errorf("GitLab base URL %q must use https (plain http is accepted only for localhost and loopback addresses)", baseURL)
	}
	var (
		api *gl.Client
		err error
	)
	if jobToken {
		api, err = gl.NewJobClient(token, gl.WithBaseURL(baseURL))
	} else {
		api, err = gl.NewClient(token, gl.WithBaseURL(baseURL))
	}
	if err != nil {
		return nil, fmt.Errorf("creating GitLab client for %q: %w", baseURL, err)
	}
	dropTokenOnCrossHostRedirect(api.HTTPClient())
	return &Client{api: api}, nil
}

// maxRedirects mirrors net/http's default limit, which a custom CheckRedirect replaces.
const maxRedirects = 10

// dropTokenOnCrossHostRedirect keeps the GitLab token on the host it was meant for. net/http
// strips only Authorization and Cookie across hosts, so the Job-Token and Private-Token headers
// client-go sets would otherwise follow a redirect to another host.
func dropTokenOnCrossHostRedirect(c *http.Client) {
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		// Another host, or a drop from https to plain http where the token would travel in the clear.
		if req.URL.Host != via[0].URL.Host || (via[0].URL.Scheme == "https" && req.URL.Scheme != "https") {
			req.Header.Del("Job-Token")
			req.Header.Del("Private-Token")
		}
		return nil
	}
}

// CreateRelease uploads each asset to the project's Generic Packages registry, then creates the
// release for tag with one package link per asset. Uploads come first so a failed upload leaves
// no asset-less release behind. Each asset is read from its open file, never reopened by path
// (platforms.OpenAssets). POST /uploads is not used: it rejects CI_JOB_TOKEN, the
// Generic Packages API accepts it. Each link's direct_asset_path keeps the public download URL
// (.../releases/<tag>/downloads/<file>) independent of the package version. ctx stops the requests
// (Ctrl-C); an upload cut short creates no release.
func (c *Client) CreateRelease(ctx context.Context, project, tag, notes string, assets []platforms.Asset) error {
	version := packageVersion(tag)
	links := make([]*gl.ReleaseAssetLinkOptions, 0, len(assets))
	for _, a := range assets {
		name := a.Name
		if _, _, err := c.api.GenericPackages.PublishPackageFile(project, assetPackageName, version, name, a.File, nil, gl.WithContext(ctx)); err != nil {
			return fmt.Errorf("uploading asset %s: %w", name, err)
		}
		path, err := c.api.GenericPackages.FormatPackageURL(project, assetPackageName, version, name)
		if err != nil {
			return fmt.Errorf("building package URL for %s: %w", name, err)
		}
		links = append(links, &gl.ReleaseAssetLinkOptions{
			Name:            gl.Ptr(name),
			URL:             gl.Ptr(c.api.BaseURL().String() + path),
			DirectAssetPath: gl.Ptr("/" + name),
			LinkType:        gl.Ptr(gl.PackageLinkType),
		})
	}

	opts := &gl.CreateReleaseOptions{
		Name:        gl.Ptr(tag),
		TagName:     gl.Ptr(tag),
		Description: gl.Ptr(notes),
	}
	if len(links) > 0 {
		opts.Assets = &gl.ReleaseAssetsOptions{Links: links}
	}
	if _, _, err := c.api.Releases.CreateRelease(project, opts, gl.WithContext(ctx)); err != nil {
		return fmt.Errorf("creating release %s: %w", tag, err)
	}
	return nil
}

// VerifyUser checks a personal token with GET /user. A job token cannot call it (401), so a CI
// run uses VerifyReleasesAccess instead.
func (c *Client) VerifyUser() error {
	if _, _, err := c.api.Users.CurrentUser(); err != nil {
		return fmt.Errorf("fetching the current user: %w", err)
	}
	return nil
}

// VerifyReleasesAccess checks that the token can read the project's releases, the API
// CI_JOB_TOKEN is documented to reach. project is a path or a numeric ID.
func (c *Client) VerifyReleasesAccess(project string) error {
	opts := &gl.ListReleasesOptions{ListOptions: gl.ListOptions{PerPage: 1}}
	if _, _, err := c.api.Releases.ListReleases(project, opts); err != nil {
		return fmt.Errorf("listing releases of %s: %w", project, err)
	}
	return nil
}

// packageVersion maps a release tag to a Generic Packages version. GitLab rejects "/" in a
// package version (400 package_version is invalid); every other character a tag format can
// produce ("+", ".", "-", "_") is accepted, so only "/" — per-env tags — is replaced.
func packageVersion(tag string) string {
	return strings.ReplaceAll(tag, "/", "-")
}
