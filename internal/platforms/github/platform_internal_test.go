package github

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

// testPlatform builds the driver with env as its whole environment and, when f is set, every
// request routed to f. The returned buffer collects the warnings.
func testPlatform(cfg *config.Platform, env map[string]string, f *fakeGitHub) (*Platform, *bytes.Buffer) {
	var warnings bytes.Buffer
	p := &Platform{cfg: cfg, getenv: func(k string) string { return env[k] }, stderr: &warnings}
	if f != nil {
		p.httpClient = f.httpClient()
	}
	return p, &warnings
}

// inRepo moves the test into a fresh repository directory holding files (path → content).
func inRepo(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	for path, content := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
}

var githubTarget = &config.Platform{Name: "github", Repository: testRepository}

// ---- Token chain (ADR-0077) ------------------------------------------------------------------

// Check and publishing read the token the same way: token_env alone when set, otherwise gh's chain
// for the host.
func TestPlatform_TokenChain(t *testing.T) {
	const ghes = "https://github.example.com"
	tests := []struct {
		name     string
		baseURL  string
		tokenEnv string
		env      map[string]string
		want     string // the token sent; "" means refused before any request
		wantErr  []string
	}{
		{name: "token_env is read alone", tokenEnv: "CORP_TOKEN",
			env: map[string]string{"CORP_TOKEN": "corp", "GH_TOKEN": "gh"}, want: "corp"},
		{name: "an empty token_env does not fall back to the chain", tokenEnv: "CORP_TOKEN",
			env: map[string]string{"GH_TOKEN": "gh", "GITHUB_TOKEN": "actions"}, wantErr: []string{"environment variable CORP_TOKEN is not set"}},
		{name: "github.com prefers GH_TOKEN", env: map[string]string{"GH_TOKEN": "gh", "GITHUB_TOKEN": "actions"}, want: "gh"},
		{name: "github.com takes GITHUB_TOKEN alone", env: map[string]string{"GITHUB_TOKEN": "actions"}, want: "actions"},
		{name: "github.com ignores the enterprise names",
			env: map[string]string{"GH_ENTERPRISE_TOKEN": "e", "GITHUB_ENTERPRISE_TOKEN": "e"}, wantErr: []string{"GH_TOKEN", "GITHUB_TOKEN"}},
		{name: "a ghe.com tenant follows github.com", baseURL: "https://acme.ghe.com",
			env: map[string]string{"GITHUB_TOKEN": "actions", "GH_ENTERPRISE_TOKEN": "e"}, want: "actions"},
		{name: "GHES prefers GH_ENTERPRISE_TOKEN", baseURL: ghes,
			env: map[string]string{"GH_ENTERPRISE_TOKEN": "e1", "GITHUB_ENTERPRISE_TOKEN": "e2", "GH_TOKEN": "gh"}, want: "e1"},
		{name: "GHES then GITHUB_ENTERPRISE_TOKEN", baseURL: ghes,
			env: map[string]string{"GITHUB_ENTERPRISE_TOKEN": "e2", "GH_TOKEN": "gh"}, want: "e2"},
		{name: "GHES then GH_TOKEN", baseURL: ghes, env: map[string]string{"GH_TOKEN": "gh"}, want: "gh"},
		{name: "GHES does not read GITHUB_TOKEN", baseURL: ghes,
			env: map[string]string{"GITHUB_TOKEN": "actions"}, wantErr: []string{"GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_TOKEN"}},
	}
	operations := map[string]func(p *Platform) error{
		"check":   func(p *Platform) error { return p.Check() },
		"publish": func(p *Platform) error { return p.CreateRelease(t.Context(), "v1.0.0", "notes", false) },
	}
	for _, tc := range tests {
		for op, run := range operations {
			t.Run(tc.name+"/"+op, func(t *testing.T) {
				uploads := githubUploadURL
				if tc.baseURL != "" {
					uploads = tc.baseURL + "/api/uploads/repos/acme/widget/releases/42/assets{?name,label}"
				}
				f := newFakeGitHub(t, uploads)
				cfg := &config.Platform{Repository: testRepository, BaseURL: tc.baseURL, TokenEnv: tc.tokenEnv}
				p, _ := testPlatform(cfg, tc.env, f)

				err := run(p)

				if tc.want == "" {
					require.Error(t, err)
					for _, s := range tc.wantErr {
						assert.Contains(t, err.Error(), s)
					}
					assert.Empty(t, f.requests(), "no request without a token")
					return
				}
				require.NoError(t, err)
				calls := f.recorded()
				require.NotEmpty(t, calls)
				for _, c := range calls {
					assert.Equal(t, "Bearer "+tc.want, c.Auth, c.String())
				}
			})
		}
	}
}

// The Actions-only GITHUB_TOKEN rule of the gh driver is gone: Check validates the token publishing
// will use.
func TestPlatform_CheckInActionsValidatesThePublishToken(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	p, _ := testPlatform(&config.Platform{Repository: testRepository, TokenEnv: "GH_TOKEN"},
		map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_TOKEN": "actions", "GITHUB_REPOSITORY": "other/repo", "GH_TOKEN": "pat"}, f)

	require.NoError(t, p.Check())

	calls := f.recorded()
	require.Len(t, calls, 1)
	assert.Equal(t, "GET api.github.com"+releasesPath, calls[0].String())
	assert.Equal(t, "1", calls[0].Query.Get("per_page"))
	assert.Equal(t, "Bearer pat", calls[0].Auth)
}

// ---- Check ------------------------------------------------------------------------------------

func TestPlatform_CheckReportsTheTokenAndTheRepositoryTogether(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	p, _ := testPlatform(&config.Platform{TokenEnv: "GH_TOKEN"}, nil, f)

	err := p.Check()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment variable GH_TOKEN is not set")
	assert.Contains(t, err.Error(), "repository not set")
	assert.Empty(t, f.requests())
}

func TestPlatform_CheckReadsTheRepositoryFromTheEnvironment(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	p, _ := testPlatform(&config.Platform{}, map[string]string{"GH_TOKEN": "tok", "GITHUB_REPOSITORY": "acme/widget"}, f)

	require.NoError(t, p.Check())
	assert.Equal(t, []string{"GET api.github.com" + releasesPath}, f.requests())
}

func TestPlatform_CheckRefusedTokenNamesTheVariable(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	f.override["GET api.github.com"+releasesPath] = failWith(http.StatusUnauthorized, `{"message":"Bad credentials"}`)
	p, _ := testPlatform(githubTarget, map[string]string{"GITHUB_TOKEN": "bad"}, f)

	err := p.Check()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "API call failed")
	assert.Contains(t, err.Error(), "401 Bad credentials")
	assert.Contains(t, err.Error(), "GITHUB_TOKEN", "the hint names the variable the token came from")
}

func TestPlatform_UsesTheActionsAPIRootOfItsOwnInstance(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	p, _ := testPlatform(githubTarget, map[string]string{
		"GH_TOKEN": "tok", "GITHUB_ACTIONS": "true", "GITHUB_SERVER_URL": "https://github.com", "GITHUB_API_URL": "https://api.proxy.example.com",
	}, f)

	require.NoError(t, p.Check())
	assert.Equal(t, []string{"GET api.proxy.example.com" + releasesPath}, f.requests())
}

func TestPlatform_GHESIsReachedUnderItsAPIRoot(t *testing.T) {
	f := newFakeGitHub(t, "https://github.example.com/api/uploads/repos/acme/widget/releases/42/assets{?name,label}")
	p, _ := testPlatform(&config.Platform{Repository: testRepository, BaseURL: "https://github.example.com"},
		map[string]string{"GH_TOKEN": "tok"}, f)

	require.NoError(t, p.Check())
	assert.Equal(t, []string{"GET github.example.com/api/v3" + releasesPath}, f.requests())
}

// ---- CreateRelease ----------------------------------------------------------------------------

func TestPlatform_CreateReleaseSendsTheTargetOptions(t *testing.T) {
	tests := []struct {
		name              string
		draft, prerelease bool
	}{
		{"plain", false, false},
		{"draft", true, false},
		{"pre-release", false, true},
		{"draft pre-release", true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitHub(t, githubUploadURL)
			p, _ := testPlatform(&config.Platform{Repository: testRepository, Draft: tc.draft}, map[string]string{"GH_TOKEN": "tok"}, f)

			require.NoError(t, p.CreateRelease(t.Context(), "v1.2.3", "## Notes\n", tc.prerelease))

			calls := f.recorded()
			require.Len(t, calls, 1)
			assert.Equal(t, "POST api.github.com"+releasesPath, calls[0].String())
			body := jsonBody(t, calls[0])
			assert.Equal(t, "v1.2.3", body["tag_name"])
			assert.Equal(t, "## Notes\n", body["body"])
			assert.Equal(t, tc.draft, body["draft"])
			assert.Equal(t, tc.prerelease, body["prerelease"])
		})
	}
}

func TestPlatform_CreateReleaseReadsTheRepositoryFromTheEnvironment(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	p, _ := testPlatform(&config.Platform{}, map[string]string{"GH_TOKEN": "tok", "GITHUB_REPOSITORY": "envorg/envrepo"}, f)

	require.NoError(t, p.CreateRelease(t.Context(), "v1.0.0", "notes", false))
	assert.Equal(t, []string{"POST api.github.com/repos/envorg/envrepo/releases"}, f.requests())
}

func TestPlatform_CreateReleaseWithoutRepositoryCallsNothing(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	p, _ := testPlatform(&config.Platform{}, map[string]string{"GH_TOKEN": "tok"}, f)

	err := p.CreateRelease(t.Context(), "v1.0.0", "notes", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository not set")
	assert.Empty(t, f.requests())
}

func TestPlatform_CreateReleaseUploadsTheMatchedFiles(t *testing.T) {
	inRepo(t, map[string]string{"dist/app_linux": "linux", "dist/app_darwin": "darwin!", "dist/other": "x"})
	f := newFakeGitHub(t, githubUploadURL)
	p, warnings := testPlatform(&config.Platform{Repository: testRepository, Assets: []string{"dist/app_*"}},
		map[string]string{"GH_TOKEN": "tok"}, f)

	require.NoError(t, p.CreateRelease(t.Context(), "v1.0.0", "notes", false))

	assert.Equal(t, []string{
		"POST api.github.com" + releasesPath,
		"POST uploads.github.com" + releasesPath + "/" + draftID + "/assets",
		"POST uploads.github.com" + releasesPath + "/" + draftID + "/assets",
		"PATCH api.github.com" + releasesPath + "/" + draftID,
	}, f.requests())
	calls := f.recorded()
	assert.Equal(t, "app_darwin", calls[1].Query.Get("name"))
	assert.Equal(t, "darwin!", string(calls[1].Body))
	assert.Equal(t, "app_linux", calls[2].Query.Get("name"))
	assert.Equal(t, "linux", string(calls[2].Body))
	assert.Empty(t, warnings.String())
}

// The tag is already pushed when assets resolve, so a pattern matching nothing warns and the release
// is still created, without assets.
func TestPlatform_CreateReleaseSkipsAnUnmatchedPatternWithAWarning(t *testing.T) {
	inRepo(t, map[string]string{"dist/sub/x": "x"})
	f := newFakeGitHub(t, githubUploadURL)
	p, warnings := testPlatform(&config.Platform{Repository: testRepository, Assets: []string{"dist/*", "build/*"}},
		map[string]string{"GH_TOKEN": "tok"}, f)

	require.NoError(t, p.CreateRelease(t.Context(), "v1.0.0", "notes", false))

	assert.Equal(t, []string{"POST api.github.com" + releasesPath}, f.requests())
	assert.Equal(t, false, jsonBody(t, f.recorded()[0])["draft"], "with nothing to upload the release is published at once")
	assert.Contains(t, warnings.String(), `"dist/*"`)
	assert.Contains(t, warnings.String(), `"build/*"`)
}

func TestPlatform_CreateReleaseWarnsWhenGitHubRenamesAnAsset(t *testing.T) {
	inRepo(t, map[string]string{"dist/meta#1.json": "{}", "dist/plain.txt": "x"})
	f := newFakeGitHub(t, githubUploadURL)
	p, warnings := testPlatform(&config.Platform{Repository: testRepository, Assets: []string{"dist/*"}},
		map[string]string{"GH_TOKEN": "tok"}, f)

	require.NoError(t, p.CreateRelease(t.Context(), "v1.0.0", "notes", false))

	assert.Equal(t, "meta#1.json", f.recorded()[1].Query.Get("name"), "the name is sent as the file's basename")
	assert.Contains(t, warnings.String(), `"dist/meta#1.json" was published as "meta.1.json"`)
	assert.NotContains(t, warnings.String(), "plain.txt")
}

func TestPlatform_CreateReleaseRefusesAnAssetOutsideTheRepository(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o600))
	inRepo(t, nil)
	f := newFakeGitHub(t, githubUploadURL)
	p, _ := testPlatform(&config.Platform{Repository: testRepository, Assets: []string{filepath.Join(outside, "*.txt")}},
		map[string]string{"GH_TOKEN": "tok"}, f)

	err := p.CreateRelease(t.Context(), "v1.0.0", "notes", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
	assert.Empty(t, f.requests())
}

func TestPlatform_CreateReleaseFailureIsWrapped(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	f.override["POST api.github.com"+releasesPath] = failWith(http.StatusUnprocessableEntity,
		`{"message":"Validation Failed","errors":[{"resource":"Release","code":"already_exists","field":"tag_name"}]}`)
	p, _ := testPlatform(githubTarget, map[string]string{"GH_TOKEN": "tok"}, f)

	err := p.CreateRelease(t.Context(), "v1.0.0", "notes", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "github release create")
	assert.Contains(t, err.Error(), "422 Validation Failed: tag_name already_exists")
}
