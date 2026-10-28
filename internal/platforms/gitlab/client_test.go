package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gl "gitlab.com/gitlab-org/api/client-go"

	"github.com/adaouat/heraut/internal/platforms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
	method  string
	path    string // decoded
	escaped string
	header  http.Header
	body    []byte
}

// fakeGitLab records every request and answers with a per-method status (default 201).
type fakeGitLab struct {
	srv      *httptest.Server
	requests []recordedRequest
	status   map[string]int      // "PUT", "POST" -> status override
	bodies   map[string]string   // method -> success body override (default "{}")
	queries  []string            // raw query of each request, parallel to requests
	hook     func(*http.Request) // runs on each request before the answer, when set
}

func newFakeGitLab(t *testing.T) *fakeGitLab {
	t.Helper()
	f := &fakeGitLab{status: map[string]int{}, bodies: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.requests = append(f.requests, recordedRequest{
			method: r.Method, path: r.URL.Path, escaped: r.URL.EscapedPath(), header: r.Header.Clone(), body: body,
		})
		f.queries = append(f.queries, r.URL.RawQuery)
		if f.hook != nil {
			f.hook(r)
		}
		code := http.StatusCreated
		if r.Method == http.MethodGet {
			code = http.StatusOK
		}
		if c, ok := f.status[r.Method]; ok {
			code = c
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if code >= 400 {
			_, _ = w.Write([]byte(`{"message":"nope"}`))
			return
		}
		if b, ok := f.bodies[r.Method]; ok {
			_, _ = w.Write([]byte(b))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func writeAsset(t *testing.T, dir, name, content string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

// opened opens each path as an asset the way platforms.OpenAssets hands it over.
func opened(t *testing.T, paths ...string) []platforms.Asset {
	t.Helper()
	assets := make([]platforms.Asset, 0, len(paths))
	for _, p := range paths {
		f, err := os.Open(p)
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		info, err := f.Stat()
		require.NoError(t, err)
		assets = append(assets, platforms.Asset{Path: p, Name: filepath.Base(p), Size: info.Size(), File: f})
	}
	return assets
}

// unescaped decodes a link URL: client-go percent-encodes "." and the project path's "/", which
// GitLab decodes back to the same route.
func unescaped(t *testing.T, v any) string {
	t.Helper()
	s, err := url.PathUnescape(v.(string))
	require.NoError(t, err)
	return s
}

func decodeRelease(t *testing.T, req recordedRequest) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(req.body, &m))
	return m
}

func TestClient_CreateRelease_UploadsAssetsThenCreatesReleaseWithLinks(t *testing.T) {
	f := newFakeGitLab(t)
	dir := t.TempDir()
	a := writeAsset(t, dir, "a.txt", "alpha")
	b := writeAsset(t, dir, "b.bin", "beta")

	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)
	require.NoError(t, c.CreateRelease(t.Context(), "acme/widget", "v1.2.3", "the notes", opened(t, a, b)))

	require.Len(t, f.requests, 3, "two uploads then one release")
	up1, up2, rel := f.requests[0], f.requests[1], f.requests[2]

	assert.Equal(t, http.MethodPut, up1.method)
	assert.Equal(t, "/api/v4/projects/acme/widget/packages/generic/release-assets/v1.2.3/a.txt", up1.path)
	assert.Contains(t, up1.escaped, "projects/acme%2Fwidget/", "the project path must stay one escaped segment")
	assert.Equal(t, "alpha", string(up1.body))
	assert.Equal(t, "/api/v4/projects/acme/widget/packages/generic/release-assets/v1.2.3/b.bin", up2.path)
	assert.Equal(t, "beta", string(up2.body))

	assert.Equal(t, http.MethodPost, rel.method)
	assert.Equal(t, "/api/v4/projects/acme/widget/releases", rel.path)
	body := decodeRelease(t, rel)
	assert.Equal(t, "v1.2.3", body["tag_name"])
	assert.Equal(t, "v1.2.3", body["name"])
	assert.Equal(t, "the notes", body["description"])

	links := body["assets"].(map[string]any)["links"].([]any)
	require.Len(t, links, 2)
	l0 := links[0].(map[string]any)
	assert.Equal(t, "a.txt", l0["name"])
	assert.Equal(t, "/a.txt", l0["direct_asset_path"])
	assert.Equal(t, "package", l0["link_type"])
	assert.Equal(t, f.srv.URL+"/api/v4/projects/acme/widget/packages/generic/release-assets/v1.2.3/a.txt", unescaped(t, l0["url"]))
	assert.Equal(t, "b.bin", links[1].(map[string]any)["name"])
}

func TestClient_CreateRelease_AuthHeader(t *testing.T) {
	tests := []struct {
		name     string
		jobToken bool
		header   string
		other    string
	}{
		{"personal token", false, "Private-Token", "Job-Token"},
		{"job token", true, "Job-Token", "Private-Token"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitLab(t)
			a := writeAsset(t, t.TempDir(), "a.txt", "x")
			c, err := NewClient(f.srv.URL, "secret", tc.jobToken)
			require.NoError(t, err)
			require.NoError(t, c.CreateRelease(t.Context(), "acme/widget", "v1.0.0", "n", opened(t, a)))

			require.NotEmpty(t, f.requests)
			for _, r := range f.requests {
				assert.Equal(t, "secret", r.header.Get(tc.header), "%s %s", r.method, r.path)
				assert.Empty(t, r.header.Get(tc.other))
			}
		})
	}
}

func TestClient_CreateRelease_NoAssets_OnlyCreatesTheRelease(t *testing.T) {
	f := newFakeGitLab(t)
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	require.NoError(t, c.CreateRelease(t.Context(), "acme/widget", "v1.0.0", "n", nil))

	require.Len(t, f.requests, 1)
	assert.Equal(t, http.MethodPost, f.requests[0].method)
	assert.NotContains(t, decodeRelease(t, f.requests[0]), "assets")
}

func TestClient_CreateRelease_PerEnvTag_SanitizesOnlyThePackageVersion(t *testing.T) {
	f := newFakeGitLab(t)
	a := writeAsset(t, t.TempDir(), "a.txt", "x")
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	require.NoError(t, c.CreateRelease(t.Context(), "acme/widget", "uat/0.0.0+smoke", "n", opened(t, a)))

	require.Len(t, f.requests, 2)
	assert.Equal(t, "/api/v4/projects/acme/widget/packages/generic/release-assets/uat-0.0.0+smoke/a.txt", f.requests[0].path)
	body := decodeRelease(t, f.requests[1])
	assert.Equal(t, "uat/0.0.0+smoke", body["tag_name"], "the git tag and release tag are never sanitized")
	link := body["assets"].(map[string]any)["links"].([]any)[0].(map[string]any)
	assert.Contains(t, unescaped(t, link["url"]), "/release-assets/uat-0.0.0+smoke/a.txt")
	assert.Equal(t, "/a.txt", link["direct_asset_path"])
}

func TestClient_CreateRelease_UploadFailure_CreatesNoRelease(t *testing.T) {
	f := newFakeGitLab(t)
	f.status["PUT"] = http.StatusForbidden
	a := writeAsset(t, t.TempDir(), "a.txt", "x")
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	err = c.CreateRelease(t.Context(), "acme/widget", "v1.0.0", "n", opened(t, a))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.txt")
	var apiErr *gl.ErrorResponse
	require.True(t, errors.As(err, &apiErr), "the API error must stay reachable with errors.As")
	assert.Equal(t, http.StatusForbidden, apiErr.Response.StatusCode)
	for _, r := range f.requests {
		assert.NotEqual(t, http.MethodPost, r.method, "no release may be created after a failed upload")
	}
}

func TestClient_CreateRelease_ReleaseFailure_ReturnsTheError(t *testing.T) {
	f := newFakeGitLab(t)
	f.status["POST"] = http.StatusForbidden
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	err = c.CreateRelease(t.Context(), "acme/widget", "v1.0.0", "n", nil)

	require.Error(t, err)
	var apiErr *gl.ErrorResponse
	assert.True(t, errors.As(err, &apiErr))
}

// The upload reads the file that was opened and checked, not whatever its path names by the time
// the client runs (NEW-002): a symlink swapped in after the open must not be followed.
func TestClient_CreateRelease_UploadsTheOpenedFileNotWhatThePathNamesLater(t *testing.T) {
	f := newFakeGitLab(t)
	repo := t.TempDir()
	t.Chdir(repo)
	writeAsset(t, repo, "app.zip", "checked")
	secret := writeAsset(t, t.TempDir(), "secret.txt", "secret")
	assets, err := platforms.OpenAssets([]string{"app.zip"}, func(p string) { t.Errorf("unexpected warning for %q", p) })
	require.NoError(t, err)
	t.Cleanup(func() { platforms.CloseAssets(assets) })

	require.NoError(t, os.Remove(filepath.Join(repo, "app.zip")))
	require.NoError(t, os.Symlink(secret, filepath.Join(repo, "app.zip")))

	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)
	require.NoError(t, c.CreateRelease(t.Context(), "acme/widget", "v1.0.0", "n", assets))

	require.Len(t, f.requests, 2)
	assert.Equal(t, "checked", string(f.requests[0].body))
}

// Ctrl-C during an upload stops the publish before the release is created, so no release links a
// package file that was never completed.
func TestClient_CreateRelease_CancelledDuringAnUploadCreatesNoRelease(t *testing.T) {
	f := newFakeGitLab(t)
	ctx, cancel := context.WithCancel(t.Context())
	f.hook = func(r *http.Request) {
		if r.Method == http.MethodPut {
			cancel()
			<-r.Context().Done()
		}
	}
	a := writeAsset(t, t.TempDir(), "a.txt", "x")
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- c.CreateRelease(ctx, "acme/widget", "v1.0.0", "n", opened(t, a)) }()
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the publish did not return")
	}

	require.ErrorIs(t, err, context.Canceled)
	for _, r := range f.requests {
		assert.NotEqual(t, http.MethodPost, r.method, "no release may be created after a cancelled upload")
	}
}

func TestClient_CreateRelease_LargeNotesArriveIntact(t *testing.T) {
	f := newFakeGitLab(t)
	notes := strings.Repeat("line of release notes\n", 50_000)
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	require.NoError(t, c.CreateRelease(t.Context(), "acme/widget", "v1.0.0", notes, nil))

	assert.Equal(t, notes, decodeRelease(t, f.requests[0])["description"])
}

func TestClient_CreateRelease_SelfHostedWithRelativeURLRoot(t *testing.T) {
	f := newFakeGitLab(t)
	c, err := NewClient(f.srv.URL+"/gitlab", "tok", false)
	require.NoError(t, err)

	require.NoError(t, c.CreateRelease(t.Context(), "acme/widget", "v1.0.0", "n", nil))

	assert.Equal(t, "/gitlab/api/v4/projects/acme/widget/releases", f.requests[0].path)
}

func TestNewClient_InvalidBaseURL(t *testing.T) {
	_, err := NewClient("://bad", "tok", false)
	require.Error(t, err)
}

func TestPackageVersion(t *testing.T) {
	tests := []struct{ tag, want string }{
		{"v1.2.3", "v1.2.3"},
		{"1.0.0+build.5", "1.0.0+build.5"},
		{"2026.10.1", "2026.10.1"},
		{"uat/0.0.0", "uat-0.0.0"},
		{"uat/0.0.0+smoke", "uat-0.0.0+smoke"},
		{"release/qa/1.0.0", "release-qa-1.0.0"},
	}
	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			assert.Equal(t, tc.want, packageVersion(tc.tag))
		})
	}
}

func TestClient_VerifyUser(t *testing.T) {
	f := newFakeGitLab(t)
	c, err := NewClient(f.srv.URL, "secret", false)
	require.NoError(t, err)

	require.NoError(t, c.VerifyUser())

	require.Len(t, f.requests, 1)
	assert.Equal(t, http.MethodGet, f.requests[0].method)
	assert.Equal(t, "/api/v4/user", f.requests[0].path)
	assert.Equal(t, "secret", f.requests[0].header.Get("Private-Token"))
}

func TestClient_VerifyUser_Rejected(t *testing.T) {
	f := newFakeGitLab(t)
	f.status["GET"] = http.StatusUnauthorized
	c, err := NewClient(f.srv.URL, "bad", false)
	require.NoError(t, err)

	err = c.VerifyUser()

	var apiErr *gl.ErrorResponse
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusUnauthorized, apiErr.Response.StatusCode)
}

func TestClient_VerifyReleasesAccess_UsesTheReleasesListWithTheJobToken(t *testing.T) {
	f := newFakeGitLab(t)
	f.bodies["GET"] = `[]`
	c, err := NewClient(f.srv.URL, "jobtok", true)
	require.NoError(t, err)

	require.NoError(t, c.VerifyReleasesAccess("42"))

	require.Len(t, f.requests, 1)
	assert.Equal(t, "/api/v4/projects/42/releases", f.requests[0].path)
	assert.Equal(t, "per_page=1", f.queries[0])
	assert.Equal(t, "jobtok", f.requests[0].header.Get("Job-Token"))
}

func TestClient_VerifyReleasesAccess_Rejected(t *testing.T) {
	f := newFakeGitLab(t)
	f.status["GET"] = http.StatusForbidden
	c, err := NewClient(f.srv.URL, "jobtok", true)
	require.NoError(t, err)

	err = c.VerifyReleasesAccess("acme/widget")

	var apiErr *gl.ErrorResponse
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusForbidden, apiErr.Response.StatusCode)
}

// TestClient_RedirectDropsTokenHeadersOnHostChange pins SEC-003 for the publish client: a redirect
// to another host must not carry the job or private token (net/http strips only Authorization).
func TestClient_RedirectDropsTokenHeadersOnHostChange(t *testing.T) {
	tests := []struct {
		name     string
		jobToken bool
		header   string
	}{
		{"job token", true, "Job-Token"},
		{"private token", false, "Private-Token"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var seen http.Header
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			defer other.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, other.URL+"/elsewhere", http.StatusFound)
			}))
			defer origin.Close()

			c, err := NewClient(origin.URL, "secret-token", tc.jobToken)
			require.NoError(t, err)
			require.NoError(t, c.VerifyReleasesAccess("group/project"))

			require.NotNil(t, seen, "the redirect target must have been reached")
			assert.Empty(t, seen.Get(tc.header), "the token must not follow a redirect to another host")
		})
	}
}

func TestClient_RedirectKeepsTokenOnSameHost(t *testing.T) {
	var gotToken string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/moved", http.StatusFound)
	})
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("Private-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewClient(srv.URL, "secret-token", false)
	require.NoError(t, err)
	require.NoError(t, c.VerifyReleasesAccess("group/project"))
	assert.Equal(t, "secret-token", gotToken, "a same-host redirect keeps the token")
}
