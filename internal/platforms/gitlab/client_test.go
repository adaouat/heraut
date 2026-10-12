package gitlab

import (
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

	gl "gitlab.com/gitlab-org/api/client-go"

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
	status   map[string]int // "PUT", "POST" -> status override
}

func newFakeGitLab(t *testing.T) *fakeGitLab {
	t.Helper()
	f := &fakeGitLab{status: map[string]int{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.requests = append(f.requests, recordedRequest{
			method: r.Method, path: r.URL.Path, escaped: r.URL.EscapedPath(), header: r.Header.Clone(), body: body,
		})
		code := http.StatusCreated
		if c, ok := f.status[r.Method]; ok {
			code = c
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if code >= 400 {
			_, _ = w.Write([]byte(`{"message":"nope"}`))
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
	require.NoError(t, c.CreateRelease("acme/widget", "v1.2.3", "the notes", []string{a, b}))

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
			require.NoError(t, c.CreateRelease("acme/widget", "v1.0.0", "n", []string{a}))

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

	require.NoError(t, c.CreateRelease("acme/widget", "v1.0.0", "n", nil))

	require.Len(t, f.requests, 1)
	assert.Equal(t, http.MethodPost, f.requests[0].method)
	assert.NotContains(t, decodeRelease(t, f.requests[0]), "assets")
}

func TestClient_CreateRelease_PerEnvTag_SanitizesOnlyThePackageVersion(t *testing.T) {
	f := newFakeGitLab(t)
	a := writeAsset(t, t.TempDir(), "a.txt", "x")
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	require.NoError(t, c.CreateRelease("acme/widget", "uat/0.0.0+smoke", "n", []string{a}))

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

	err = c.CreateRelease("acme/widget", "v1.0.0", "n", []string{a})

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

	err = c.CreateRelease("acme/widget", "v1.0.0", "n", nil)

	require.Error(t, err)
	var apiErr *gl.ErrorResponse
	assert.True(t, errors.As(err, &apiErr))
}

func TestClient_CreateRelease_MissingAsset_FailsBeforeAnyRequest(t *testing.T) {
	f := newFakeGitLab(t)
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	err = c.CreateRelease("acme/widget", "v1.0.0", "n", []string{filepath.Join(t.TempDir(), "gone.txt")})

	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.Empty(t, f.requests)
}

func TestClient_CreateRelease_DuplicateBasenames_FailsBeforeAnyRequest(t *testing.T) {
	f := newFakeGitLab(t)
	dir := t.TempDir()
	one := writeAsset(t, filepath.Join(dir, "x"), "app.tgz", "1")
	two := writeAsset(t, filepath.Join(dir, "y"), "app.tgz", "2")
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	err = c.CreateRelease("acme/widget", "v1.0.0", "n", []string{one, two})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "app.tgz")
	assert.Empty(t, f.requests)
}

func TestClient_CreateRelease_LargeNotesArriveIntact(t *testing.T) {
	f := newFakeGitLab(t)
	notes := strings.Repeat("line of release notes\n", 50_000)
	c, err := NewClient(f.srv.URL, "tok", false)
	require.NoError(t, err)

	require.NoError(t, c.CreateRelease("acme/widget", "v1.0.0", notes, nil))

	assert.Equal(t, notes, decodeRelease(t, f.requests[0])["description"])
}

func TestClient_CreateRelease_SelfHostedWithRelativeURLRoot(t *testing.T) {
	f := newFakeGitLab(t)
	c, err := NewClient(f.srv.URL+"/gitlab", "tok", false)
	require.NoError(t, err)

	require.NoError(t, c.CreateRelease("acme/widget", "v1.0.0", "n", nil))

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
