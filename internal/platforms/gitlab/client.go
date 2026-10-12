package gitlab

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gl "gitlab.com/gitlab-org/api/client-go"
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
	return &Client{api: api}, nil
}

// CreateRelease uploads each file to the project's Generic Packages registry, then creates the
// release for tag with one package link per file. Uploads come first so a failed upload leaves
// no asset-less release behind. POST /uploads is not used: it rejects CI_JOB_TOKEN, the
// Generic Packages API accepts it. Each link's direct_asset_path keeps the public download URL
// (.../releases/<tag>/downloads/<file>) independent of the package version.
func (c *Client) CreateRelease(project, tag, notes string, files []string) error {
	if err := checkAssets(files); err != nil {
		return err
	}

	version := packageVersion(tag)
	links := make([]*gl.ReleaseAssetLinkOptions, 0, len(files))
	for _, file := range files {
		name := filepath.Base(file)
		if err := c.upload(project, version, name, file); err != nil {
			return err
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
	if _, _, err := c.api.Releases.CreateRelease(project, opts); err != nil {
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

func (c *Client) upload(project, version, name, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("opening asset %s: %w", name, err)
	}
	defer func() { _ = f.Close() }()
	if _, _, err := c.api.GenericPackages.PublishPackageFile(project, assetPackageName, version, name, f, nil); err != nil {
		return fmt.Errorf("uploading asset %s: %w", name, err)
	}
	return nil
}

// checkAssets fails before any request when a file is missing or two files share a basename
// (they would overwrite each other in the package and collide as release links).
func checkAssets(files []string) error {
	seen := make(map[string]string, len(files))
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			return fmt.Errorf("asset %s: %w", file, err)
		}
		name := filepath.Base(file)
		if prev, dup := seen[name]; dup {
			return fmt.Errorf("assets %s and %s share the file name %q", prev, file, name)
		}
		seen[name] = file
	}
	return nil
}

// packageVersion maps a release tag to a Generic Packages version. GitLab rejects "/" in a
// package version (400 package_version is invalid); every other character a tag format can
// produce ("+", ".", "-", "_") is accepted, so only "/" — per-env tags — is replaced.
func packageVersion(tag string) string {
	return strings.ReplaceAll(tag, "/", "-")
}
