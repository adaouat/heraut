package github

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	gh "github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms"
)

const (
	testRepository = "acme/widget"
	testToken      = "test-token"
	releasesPath   = "/repos/acme/widget/releases"
	draftID        = "42"
)

type recorded struct {
	Method, Host, Path, Auth, ContentType string
	Query                                 url.Values
	ContentLength                         int64
	Body                                  []byte
}

func (r recorded) String() string { return r.Method + " " + r.Host + r.Path }

// fakeGitHub answers as GitHub does, for every host name: the test client dials every connection to
// it (TLS for port 443 and other ports, plain HTTP for port 80), so the client is exercised with real
// host names while nothing leaves the machine.
type fakeGitHub struct {
	tls, plain *httptest.Server
	uploadURL  string
	override   map[string]http.HandlerFunc // keyed by recorded.String()

	mu    sync.Mutex
	calls []recorded
}

func newFakeGitHub(t *testing.T, uploadURL string) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{uploadURL: uploadURL, override: map[string]http.HandlerFunc{}}
	f.tls = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	f.plain = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.tls.Close)
	t.Cleanup(f.plain.Close)
	return f
}

func (f *fakeGitHub) httpClient() *http.Client {
	tr := f.tls.Client().Transport.(*http.Transport).Clone()
	tr.TLSClientConfig.ServerName = "example.com" // the name httptest's certificate is issued for
	tlsAddr, plainAddr := f.tls.Listener.Addr().String(), f.plain.Listener.Addr().String()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		target := tlsAddr
		if strings.HasSuffix(addr, ":80") {
			target = plainAddr
		}
		return (&net.Dialer{}).DialContext(ctx, network, target)
	}
	return &http.Client{Transport: tr}
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rec := recorded{
		Method: r.Method, Host: r.Host, Path: r.URL.Path, Query: r.URL.Query(),
		Auth: r.Header.Get("Authorization"), ContentType: r.Header.Get("Content-Type"),
		ContentLength: r.ContentLength, Body: body,
	}
	f.mu.Lock()
	f.calls = append(f.calls, rec)
	f.mu.Unlock()

	if h, ok := f.override[rec.String()]; ok {
		h(w, r)
		return
	}
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/releases"):
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": 42, "tag_name": req["tag_name"], "draft": req["draft"], "prerelease": req["prerelease"],
			"upload_url": f.uploadURL,
		})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/assets"):
		// GitHub rewrites special characters in asset names.
		writeJSON(w, http.StatusCreated, map[string]any{"name": strings.ReplaceAll(r.URL.Query().Get("name"), "#", ".")})
	case r.Method == http.MethodPatch:
		writeJSON(w, http.StatusOK, map[string]any{"id": 42, "draft": false})
	case r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/releases"):
		writeJSON(w, http.StatusOK, []any{})
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
	}
}

func (f *fakeGitHub) recorded() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.calls...)
}

func (f *fakeGitHub) requests() []string {
	var out []string
	for _, c := range f.recorded() {
		out = append(out, c.String())
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func failWith(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func jsonBody(t *testing.T, r recorded) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(r.Body, &m), "body: %s", r.Body)
	return m
}

func testAsset(t *testing.T, name, content string) platforms.Asset {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return platforms.Asset{Path: path, Name: name, Size: int64(len(content)), File: f}
}

func noEnv(string) string { return "" }

const githubUploadURL = "https://uploads.github.com/repos/acme/widget/releases/42/assets{?name,label}"

func githubClient(t *testing.T, f *fakeGitHub) *Client {
	t.Helper()
	c, err := newClient("https://github.com", testRepository, testToken, noEnv, f.httpClient())
	require.NoError(t, err)
	return c
}

// ---- No assets: one call ----------------------------------------------------------------------

func TestCreateRelease_WithoutAssetsIsOneCreateCall(t *testing.T) {
	tests := []struct {
		name              string
		tag               string
		draft, prerelease bool
	}{
		{"published", "v1.2.3", false, false},
		{"draft", "v1.2.3", true, false},
		{"prerelease", "v2.0.0-rc.1", false, true},
		{"slash tag", "uat/0.0.1", false, false},
		{"build metadata tag", "v0.0.2+a.1", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitHub(t, githubUploadURL)

			names, err := githubClient(t, f).CreateRelease(tc.tag, "the notes", tc.draft, tc.prerelease, nil)
			require.NoError(t, err)
			assert.Empty(t, names)

			calls := f.recorded()
			require.Equal(t, []string{"POST api.github.com" + releasesPath}, f.requests())
			assert.Equal(t, "Bearer "+testToken, calls[0].Auth)
			assert.Equal(t, map[string]any{
				"tag_name": tc.tag, "name": tc.tag, "body": "the notes", "draft": tc.draft, "prerelease": tc.prerelease,
			}, jsonBody(t, calls[0]))
		})
	}
}

func TestCreateRelease_LargeNotesTravelInTheBody(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	notes := strings.Repeat("- a changelog line\n", (1<<20)/19+1)

	_, err := githubClient(t, f).CreateRelease("v1.2.3", notes, false, false, nil)
	require.NoError(t, err)
	assert.Equal(t, notes, jsonBody(t, f.recorded()[0])["body"])
}

// ---- With assets: draft, upload, publish ------------------------------------------------------

func TestCreateRelease_WithAssetsCreatesADraftUploadsThenPublishes(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	assets := []platforms.Asset{testAsset(t, "app.zip", "zip bytes"), testAsset(t, "notes.txt", "hello")}

	names, err := githubClient(t, f).CreateRelease("v1.2.3", "the notes", false, true, assets)
	require.NoError(t, err)
	assert.Equal(t, []string{"app.zip", "notes.txt"}, names)

	assert.Equal(t, []string{
		"POST api.github.com" + releasesPath,
		"POST uploads.github.com" + releasesPath + "/42/assets",
		"POST uploads.github.com" + releasesPath + "/42/assets",
		"PATCH api.github.com" + releasesPath + "/42",
	}, f.requests())

	calls := f.recorded()
	create := jsonBody(t, calls[0])
	assert.Equal(t, true, create["draft"], "created as a draft so the uploads land before it is published")
	assert.Equal(t, true, create["prerelease"])

	for i, want := range []struct{ name, body, contentType string }{
		{"app.zip", "zip bytes", "application/zip"},
		{"notes.txt", "hello", "text/plain; charset=utf-8"},
	} {
		up := calls[1+i]
		assert.Equal(t, want.name, up.Query.Get("name"))
		assert.Equal(t, want.body, string(up.Body))
		assert.Equal(t, int64(len(want.body)), up.ContentLength)
		assert.Equal(t, want.contentType, up.ContentType)
		assert.Equal(t, "Bearer "+testToken, up.Auth)
	}

	assert.Equal(t, map[string]any{"draft": false}, jsonBody(t, calls[3]))
}

func TestCreateRelease_DraftTargetWithAssetsIsNotPublished(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)

	_, err := githubClient(t, f).CreateRelease("v1.2.3", "n", true, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"POST api.github.com" + releasesPath,
		"POST uploads.github.com" + releasesPath + "/42/assets",
	}, f.requests())
}

func TestCreateRelease_ReturnsTheNamesGitHubPublished(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)

	names, err := githubClient(t, f).CreateRelease("v1.2.3", "n", false, false, []platforms.Asset{testAsset(t, "meta#1.json", "{}")})
	require.NoError(t, err)
	assert.Equal(t, "meta#1.json", f.recorded()[1].Query.Get("name"), "the basename is sent as is")
	assert.Equal(t, []string{"meta.1.json"}, names)
}

// ---- Hosts ------------------------------------------------------------------------------------

func TestCreateRelease_Hosts(t *testing.T) {
	tests := []struct {
		name, baseURL, uploadURL string
		env                      map[string]string
		wantCreate, wantUpload   string
	}{
		{
			name: "GHES", baseURL: "https://ghe.example.com",
			uploadURL:  "https://ghe.example.com/api/uploads/repos/acme/widget/releases/42/assets{?name,label}",
			wantCreate: "POST ghe.example.com/api/v3" + releasesPath,
			wantUpload: "POST ghe.example.com/api/uploads" + releasesPath + "/42/assets",
		},
		{
			name: "GHES with subdomain isolation", baseURL: "https://ghe.example.com",
			uploadURL:  "https://uploads.ghe.example.com/repos/acme/widget/releases/42/assets{?name,label}",
			wantCreate: "POST ghe.example.com/api/v3" + releasesPath,
			wantUpload: "POST uploads.ghe.example.com" + releasesPath + "/42/assets",
		},
		{
			name: "GHE.com tenant", baseURL: "https://octo.ghe.com",
			uploadURL:  "https://uploads.octo.ghe.com/repos/acme/widget/releases/42/assets{?name,label}",
			wantCreate: "POST api.octo.ghe.com" + releasesPath,
			wantUpload: "POST uploads.octo.ghe.com" + releasesPath + "/42/assets",
		},
		{
			name: "GITHUB_API_URL in Actions on the same instance", baseURL: "https://ghe.example.com",
			env: map[string]string{
				config.EnvGitHubActions:   "true",
				config.EnvGitHubServerURL: "https://ghe.example.com",
				config.EnvGitHubAPIURL:    "https://ghe.example.com/custom/api",
			},
			uploadURL:  "https://ghe.example.com/api/uploads/repos/acme/widget/releases/42/assets{?name,label}",
			wantCreate: "POST ghe.example.com/custom/api" + releasesPath,
			wantUpload: "POST ghe.example.com/api/uploads" + releasesPath + "/42/assets",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitHub(t, tc.uploadURL)
			getenv := func(k string) string { return tc.env[k] }
			c, err := newClient(tc.baseURL, testRepository, testToken, getenv, f.httpClient())
			require.NoError(t, err)

			_, err = c.CreateRelease("v1.2.3", "n", true, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
			require.NoError(t, err)
			assert.Equal(t, []string{tc.wantCreate, tc.wantUpload}, f.requests())
			for _, call := range f.recorded() {
				assert.Equal(t, "Bearer "+testToken, call.Auth, call.String())
			}
		})
	}
}

// ---- Upload URL allowlist ---------------------------------------------------------------------

func TestCreateRelease_RefusesAnUnexpectedUploadOriginAndDeletesTheDraft(t *testing.T) {
	for _, uploadURL := range []string{
		"https://evil.example/repos/acme/widget/releases/42/assets{?name,label}",
		"https://uploads.github.com.evil.example/repos/acme/widget/releases/42/assets",
		"https://api.github.com/repos/acme/widget/releases/42/assets",
		"http://uploads.github.com/repos/acme/widget/releases/42/assets",
		"https://uploads.github.com:8443/repos/acme/widget/releases/42/assets",
	} {
		t.Run(uploadURL, func(t *testing.T) {
			f := newFakeGitHub(t, uploadURL)

			_, err := githubClient(t, f).CreateRelease("v1.2.3", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "upload URL")
			assert.Equal(t, []string{
				"POST api.github.com" + releasesPath,
				"DELETE api.github.com" + releasesPath + "/42",
			}, f.requests(), "no byte of the asset is sent")
		})
	}
}

func TestCheckUploadURL(t *testing.T) {
	tests := []struct {
		baseURL, uploadURL, want string
	}{
		{"https://github.com", "https://uploads.github.com/repos/a/b/releases/1/assets{?name,label}", "https://uploads.github.com/"},
		{"https://ghe.example.com", "https://ghe.example.com/api/uploads/repos/a/b/releases/1/assets", "https://ghe.example.com/"},
		{"https://ghe.example.com", "https://UPLOADS.ghe.example.com/repos/a/b/releases/1/assets", "https://UPLOADS.ghe.example.com/"},
		{"https://ghe.example.com:8443", "https://uploads.ghe.example.com:8443/repos/a/b/releases/1/assets", "https://uploads.ghe.example.com:8443/"},
		{"https://ghe.example.com:443", "https://ghe.example.com/api/uploads/x", "https://ghe.example.com/"},
		{"https://example.com/github", "https://example.com/github/api/uploads/x", "https://example.com/"},
		{"https://octo.ghe.com", "https://uploads.octo.ghe.com/repos/a/b/releases/1/assets", "https://uploads.octo.ghe.com/"},
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080/api/uploads/x", "http://127.0.0.1:8080/"},
	}
	for _, tc := range tests {
		t.Run(tc.baseURL+" "+tc.uploadURL, func(t *testing.T) {
			got, err := checkUploadURL(tc.baseURL, tc.uploadURL)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	refused := []struct{ baseURL, uploadURL string }{
		{"https://github.com", "https://github.com/repos/a/b/releases/1/assets"},
		{"https://github.com", "https://uploads.uploads.github.com/x"},
		{"https://ghe.example.com", "https://ghe.example.com:8443/api/uploads/x"},
		{"https://ghe.example.com:8443", "https://ghe.example.com/api/uploads/x"},
		{"https://ghe.example.com", "http://ghe.example.com/api/uploads/x"},
		{"https://ghe.example.com", "https://other.example.com/api/uploads/x"},
		{"https://ghe.example.com", "https://evil-ghe.example.com/x"},
		{"https://ghe.example.com", "https://uploads.ghe.example.com.evil.example/x"},
		{"https://ghe.example.com", "/api/uploads/x"},
		{"https://ghe.example.com", ""},
		{"https://ghe.example.com", "https://user:pass@ghe.example.com/x"},
	}
	for _, tc := range refused {
		t.Run("refused "+tc.baseURL+" "+tc.uploadURL, func(t *testing.T) {
			_, err := checkUploadURL(tc.baseURL, tc.uploadURL)
			require.Error(t, err)
		})
	}
}

// ---- Failure cleanup and error text -----------------------------------------------------------

const uploadKey = "POST uploads.github.com" + releasesPath + "/" + draftID + "/assets"

func TestCreateRelease_FailedUploadDeletesTheDraft(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	f.override[uploadKey] = failWith(http.StatusUnprocessableEntity,
		`{"message":"Validation Failed","errors":[{"resource":"ReleaseAsset","code":"already_exists","field":"name"}]}`)

	_, err := githubClient(t, f).CreateRelease("v1.2.3", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
	require.Error(t, err)
	assert.Equal(t, []string{
		"POST api.github.com" + releasesPath,
		uploadKey,
		"DELETE api.github.com" + releasesPath + "/42",
	}, f.requests())

	assert.Contains(t, err.Error(), "uploading asset app.zip")
	assert.Contains(t, err.Error(), "422 Validation Failed: name already_exists")
	assertReadable(t, err)
	var apiErr *gh.ErrorResponse
	require.ErrorAs(t, err, &apiErr, "the go-github error stays reachable")
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.Response.StatusCode)
}

func TestCreateRelease_FailedPublishDeletesTheDraft(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	f.override["PATCH api.github.com"+releasesPath+"/42"] = failWith(http.StatusInternalServerError, `{"message":"Server Error"}`)

	_, err := githubClient(t, f).CreateRelease("v1.2.3", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
	require.Error(t, err)
	assert.Equal(t, []string{
		"POST api.github.com" + releasesPath,
		uploadKey,
		"PATCH api.github.com" + releasesPath + "/42",
		"DELETE api.github.com" + releasesPath + "/42",
	}, f.requests())
	assert.Contains(t, err.Error(), "publishing release v1.2.3")
	assert.Contains(t, err.Error(), "500 Server Error")
	assertReadable(t, err)
}

func TestCreateRelease_AFailedDeleteIsAppendedToTheOriginalError(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	f.override[uploadKey] = failWith(http.StatusUnprocessableEntity,
		`{"message":"Validation Failed","errors":[{"resource":"Release","code":"custom","message":"Cannot upload assets to an immutable release."}]}`)
	f.override["DELETE api.github.com"+releasesPath+"/42"] = failWith(http.StatusForbidden, `{"message":"Must have admin rights to Repository."}`)

	_, err := githubClient(t, f).CreateRelease("v1.2.3", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "Cannot upload assets to an immutable release.")
	assert.Contains(t, msg, "Must have admin rights to Repository.")
	assert.Less(t, strings.Index(msg, "immutable"), strings.Index(msg, "admin rights"), "the original error comes first")
	assertReadable(t, err)
	var apiErr *gh.ErrorResponse
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.Response.StatusCode, "the original error, not the delete's")
}

func TestCreateRelease_FailedCreateLeavesNothingToDelete(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	f.override["POST api.github.com"+releasesPath] = failWith(http.StatusUnprocessableEntity,
		`{"message":"Validation Failed","errors":[{"resource":"Release","code":"already_exists","field":"tag_name"}]}`)

	_, err := githubClient(t, f).CreateRelease("v1.2.3", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
	require.Error(t, err)
	assert.Equal(t, []string{"POST api.github.com" + releasesPath}, f.requests())
	assert.Contains(t, err.Error(), "creating release v1.2.3")
	assert.Contains(t, err.Error(), "422 Validation Failed: tag_name already_exists")
	assertReadable(t, err)
}

// assertReadable guards against go-github's own Error(), which prints its error list as a Go struct.
func assertReadable(t *testing.T, err error) {
	t.Helper()
	for _, dump := range []string{"Resource:", "Field:", "Code:", "[{", "&{"} {
		assert.NotContains(t, err.Error(), dump)
	}
}

// ---- Access check and redirects ---------------------------------------------------------------

func TestVerifyReleasesAccess(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)

	require.NoError(t, githubClient(t, f).VerifyReleasesAccess())
	calls := f.recorded()
	require.Equal(t, []string{"GET api.github.com" + releasesPath}, f.requests())
	assert.Equal(t, "1", calls[0].Query.Get("per_page"))
	assert.Equal(t, "Bearer "+testToken, calls[0].Auth)
}

func TestVerifyReleasesAccess_BadCredentials(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	f.override["GET api.github.com"+releasesPath] = failWith(http.StatusUnauthorized, `{"message":"Bad credentials"}`)

	err := githubClient(t, f).VerifyReleasesAccess()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing releases of acme/widget")
	assert.Contains(t, err.Error(), "401 Bad credentials")
	assertReadable(t, err)
}

func TestVerifyReleasesAccess_TheTokenDoesNotFollowARedirectToAnotherOrigin(t *testing.T) {
	for _, location := range []string{
		"https://evil.example/landing",
		"http://api.github.com/landing",
		"https://api.github.com:8443/landing",
	} {
		t.Run(location, func(t *testing.T) {
			f := newFakeGitHub(t, githubUploadURL)
			f.override["GET api.github.com"+releasesPath] = func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusFound)
			}
			f.override["GET "+strings.TrimPrefix(strings.TrimPrefix(location, "https://"), "http://")] = func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, http.StatusOK, []any{})
			}

			_ = githubClient(t, f).VerifyReleasesAccess()
			calls := f.recorded()
			require.Len(t, calls, 2, "%v", f.requests())
			assert.Equal(t, "Bearer "+testToken, calls[0].Auth)
			assert.Empty(t, calls[1].Auth, "token sent to %s", calls[1])
		})
	}
}

// ---- Construction -----------------------------------------------------------------------------

func TestNewClient_Refuses(t *testing.T) {
	tests := []struct {
		name, baseURL, repository, token, want string
	}{
		{"empty token", "https://github.com", testRepository, "", "token"},
		{"insecure base URL", "http://ghe.example.com", testRepository, testToken, "https"},
		{"no repository", "https://github.com", "", testToken, "repository"},
		{"no slash", "https://github.com", "acme", testToken, "owner/repo"},
		{"empty repo", "https://github.com", "acme/", testToken, "owner/repo"},
		{"empty owner", "https://github.com", "/widget", testToken, "owner/repo"},
		{"nested path", "https://github.com", "acme/widget/extra", testToken, "owner/repo"},
		{"dot-dot repo", "https://github.com", "acme/..", testToken, "owner/repo"},
		{"space", "https://github.com", "acme/wid get", testToken, "owner/repo"},
		{"query", "https://github.com", "acme/widget?x=1", testToken, "owner/repo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClient(tc.baseURL, tc.repository, tc.token, noEnv)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestNewClient_AcceptsGitHubNames(t *testing.T) {
	for _, repository := range []string{"acme/widget", "Acme-Corp/my_repo.go", "a/.github", "o-1/r-2.x"} {
		_, err := NewClient("https://github.com", repository, testToken, noEnv)
		assert.NoError(t, err, repository)
	}
}

func TestNewClient_LeavesTheGivenHTTPClientAlone(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	hc := f.httpClient()
	transport := hc.Transport

	c, err := newClient("https://github.com", testRepository, testToken, noEnv, hc)
	require.NoError(t, err)
	_, err = c.CreateRelease("v1.2.3", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
	require.NoError(t, err)
	assert.Same(t, transport, hc.Transport, "go-github must wrap a copy, not the http.Client it is given")
}
