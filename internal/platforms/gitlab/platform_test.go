package gitlab_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/gitlab"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	p := gitlab.New(&config.Platform{Name: "gitlab-internal"})
	assert.Equal(t, "gitlab-internal", p.Name())
}

func TestReleaseURL(t *testing.T) {
	t.Setenv("GITLAB_CI", "") // ensure CI_SERVER_URL fallback is not active
	cfg := &config.Platform{Project: "mygroup/myrepo"}
	p := gitlab.New(cfg)
	assert.Equal(t, "https://gitlab.com/mygroup/myrepo/-/releases/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestReleaseURL_FromEnv(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CI_PROJECT_PATH", "envgroup/envrepo")
	p := gitlab.New(&config.Platform{})
	assert.Equal(t, "https://gitlab.com/envgroup/envrepo/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestReleaseURL_SelfHosted(t *testing.T) {
	cfg := &config.Platform{Project: "grp/repo", BaseURL: "https://gitlab.example.com"}
	p := gitlab.New(cfg)
	assert.Equal(t, "https://gitlab.example.com/grp/repo/-/releases/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestReleaseURL_InCI_NoBaseURL_UsesServerURL(t *testing.T) {
	// When BaseURL is not configured and running in GitLab CI, the release link
	// must use CI_SERVER_URL (the actual GitLab instance) instead of gitlab.com.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", "https://git.example.com")
	cfg := &config.Platform{Project: "grp/repo"}
	p := gitlab.New(cfg)
	assert.Equal(t, "https://git.example.com/grp/repo/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestReleaseURL_InCI_NoBaseURL_NoServerURL_FallsBackToProjectURL(t *testing.T) {
	// GITLAB_CI=true, CI_SERVER_URL absent — CI_PROJECT_URL carries the host.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", "")
	t.Setenv("CI_PROJECT_URL", "https://git.example.com/acme/widget")
	cfg := &config.Platform{Project: "acme/widget"}
	p := gitlab.New(cfg)
	assert.Equal(t, "https://git.example.com/acme/widget/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestReleaseURL_NoGitlabCIVar_ProjectURLSuffices(t *testing.T) {
	// CI_PROJECT_URL set but GITLAB_CI absent/false: some CI containers and custom
	// setups expose CI_PROJECT_URL without the GITLAB_CI marker. resolveBaseURL must
	// treat CI_PROJECT_URL as the authoritative source regardless — consistent with how
	// ambientLinkContext() uses it unconditionally for notes/changelog link resolution.
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CI_SERVER_URL", "")
	t.Setenv("CI_PROJECT_URL", "https://git.example.com/acme/widget")
	cfg := &config.Platform{Project: "acme/widget"}
	p := gitlab.New(cfg)
	assert.Equal(t, "https://git.example.com/acme/widget/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

// TestReleaseURLFromContext_* verify that ReleaseURLFromContext derives the URL from the
// provided LinkContext rather than re-reading environment variables. This is the "one
// place" fix: the pipeline passes the already-resolved platformLinkContext here so the
// displayed URL is always consistent with the commit links in the release notes.

func TestReleaseURLFromContext_AmbientContext(t *testing.T) {
	// Ambient context (from CI_PROJECT_URL): Owner/Repo are empty, BaseURL is the
	// full project URL. The release URL is built directly from BaseURL.
	cfg := &config.Platform{Project: "acme/widget"}
	p := gitlab.New(cfg)
	lc := &port.LinkContext{BaseURL: "https://git.example.com/acme/widget", Platform: "gitlab"}
	assert.Equal(t, "https://git.example.com/acme/widget/-/releases/v1.0.0", p.ReleaseURLFromContext("v1.0.0", lc))
}

func TestReleaseURLFromContext_PlatformContext(t *testing.T) {
	// Platform-derived context: BaseURL is just the host, Owner/Repo are populated.
	// The release URL is assembled from host + configured project path.
	cfg := &config.Platform{Project: "acme/widget"}
	p := gitlab.New(cfg)
	lc := &port.LinkContext{BaseURL: "https://git.example.com", Owner: "acme", Repo: "widget", Platform: "gitlab"}
	assert.Equal(t, "https://git.example.com/acme/widget/-/releases/v1.0.0", p.ReleaseURLFromContext("v1.0.0", lc))
}

func TestReleaseURLFromContext_Nil_FallsBackToReleaseURL(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CI_PROJECT_URL", "")
	cfg := &config.Platform{Project: "grp/repo"}
	p := gitlab.New(cfg)
	assert.Equal(t, p.ReleaseURL("v1.0.0"), p.ReleaseURLFromContext("v1.0.0", nil))
}

// Tags may carry SemVer build metadata ("+"), which must reach the URL as %2B (ADR-0064).
func TestReleaseURL_EscapesPlusInTag(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	p := gitlab.New(&config.Platform{Project: "group/proj"})
	assert.Equal(t, "https://gitlab.com/group/proj/-/releases/v1.4.0%2B158404", p.ReleaseURL("v1.4.0+158404"))

	ambient := &port.LinkContext{BaseURL: "https://gitlab.com/group/proj", Platform: "gitlab"}
	assert.Equal(t, "https://gitlab.com/group/proj/-/releases/uat%2F7.4.1%2B158404", p.ReleaseURLFromContext("uat/7.4.1+158404", ambient))

	platform := &port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group", Repo: "proj", Platform: "gitlab"}
	assert.Equal(t, "https://gitlab.com/group/proj/-/releases/v1.4.0%2B158404", p.ReleaseURLFromContext("v1.4.0+158404", platform))
}

// GitLab routes a release tag as a single path segment, so the "/" of a per-env tag must reach
// the URL as %2F — matching the link GitLab itself renders for such a release.
func TestReleaseURL_EscapesSlashInTag(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CI_PROJECT_URL", "")
	p := gitlab.New(&config.Platform{Project: "group/proj"})

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"ReleaseURL", p.ReleaseURL("uat/7.4.1"), "https://gitlab.com/group/proj/-/releases/uat%2F7.4.1"},
		{
			"ambient context",
			p.ReleaseURLFromContext("uat/7.4.1", &port.LinkContext{BaseURL: "https://gitlab.com/group/proj", Platform: "gitlab"}),
			"https://gitlab.com/group/proj/-/releases/uat%2F7.4.1",
		},
		{
			"platform context, nested group",
			p.ReleaseURLFromContext("prod/eu/7.4.1+5", &port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group/sub", Repo: "proj", Platform: "gitlab"}),
			"https://gitlab.com/group/sub/proj/-/releases/prod%2Feu%2F7.4.1%2B5",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.got)
		})
	}
}

func TestLinkContext_NestedGroup(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	// GitLab project paths split on the LAST slash: group/subgroup is the owner, the
	// final segment is the repo.
	cfg := &config.Platform{Project: "group/sub/proj", BaseURL: "https://gitlab.com"}
	p := gitlab.New(cfg)
	assert.Equal(t, port.LinkContext{
		BaseURL:  "https://gitlab.com",
		Owner:    "group/sub",
		Repo:     "proj",
		Platform: "gitlab",
	}, p.LinkContext())
}

func TestLinkContext_SimpleProject(t *testing.T) {
	t.Setenv("GITLAB_CI", "") // ensure CI_SERVER_URL fallback is not active
	t.Setenv("GITLAB_TOKEN", "")
	// BaseURL empty (config not normalized) → falls back to the default host.
	cfg := &config.Platform{Project: "group/proj"}
	lc := gitlab.New(cfg).LinkContext()
	assert.Equal(t, "group", lc.Owner)
	assert.Equal(t, "proj", lc.Repo)
	assert.Equal(t, "https://gitlab.com", lc.BaseURL)
}

func TestLinkContext_FromEnv(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_PROJECT_PATH", "envgroup/envrepo")
	lc := gitlab.New(&config.Platform{}).LinkContext()
	assert.Equal(t, "envgroup", lc.Owner)
	assert.Equal(t, "envrepo", lc.Repo)
}

func TestLinkContext_InCI_NoBaseURL_UsesServerURL(t *testing.T) {
	// LinkContext must also use CI_SERVER_URL so release-notes links resolve to the
	// actual GitLab instance, not gitlab.com.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", "https://git.example.com")
	t.Setenv("GITLAB_TOKEN", "")
	cfg := &config.Platform{Project: "grp/repo"}
	lc := gitlab.New(cfg).LinkContext()
	assert.Equal(t, "https://git.example.com", lc.BaseURL)
}

// stubGitLab records every request and answers 201 (POST/PUT), 200 (GET) or the configured status.
type stubGitLab struct {
	srv      *httptest.Server
	requests []stubRequest
	status   map[string]int
}

type stubRequest struct {
	method, path, rawQuery string
	header                 http.Header
	body                   []byte
}

func newStub(t *testing.T) *stubGitLab {
	t.Helper()
	s := &stubGitLab{status: map[string]int{}}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.requests = append(s.requests, stubRequest{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone(), body})
		code := http.StatusCreated
		payload := `{}`
		if r.Method == http.MethodGet {
			code = http.StatusOK
			if strings.HasSuffix(r.URL.Path, "/releases") {
				payload = `[]`
			}
		}
		if c, ok := s.status[r.Method]; ok {
			code = c
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if code >= 400 {
			payload = `{"message":"denied"}`
		}
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *stubGitLab) release(t *testing.T) map[string]any {
	t.Helper()
	for _, r := range s.requests {
		if r.method == http.MethodPost && strings.HasSuffix(r.path, "/releases") {
			var m map[string]any
			require.NoError(t, json.Unmarshal(r.body, &m))
			return m
		}
	}
	t.Fatalf("no release request in %v", s.requests)
	return nil
}

func (s *stubGitLab) puts() []string {
	var out []string
	for _, r := range s.requests {
		if r.method == http.MethodPut {
			out = append(out, r.path)
		}
	}
	return out
}

// notInCI clears every GitLab CI signal so a developer's or runner's ambient env cannot leak in.
func notInCI(t *testing.T) {
	t.Helper()
	for _, k := range []string{"GITLAB_CI", "CI_SERVER_URL", "CI_PROJECT_URL", "CI_PROJECT_ID", "CI_PROJECT_PATH", "CI_JOB_TOKEN"} {
		t.Setenv(k, "")
	}
}

// ---- Check -------------------------------------------------------------------

func TestCheck_TokenMissing(t *testing.T) {
	notInCI(t)
	t.Setenv("GITLAB_TOKEN", "")
	p := gitlab.New(&config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
}

func TestCheck_ProjectMissing(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{TokenEnv: "GITLAB_TOKEN", BaseURL: st.srv.URL})

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project")
}

func TestCheck_OK_ValidatesThePersonalTokenAgainstUser(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo", BaseURL: st.srv.URL})

	require.NoError(t, p.Check())

	require.Len(t, st.requests, 1)
	assert.Equal(t, "/api/v4/user", st.requests[0].path)
	assert.Equal(t, "tok", st.requests[0].header.Get("Private-Token"))
}

// TestCheck_DefaultTokenEnv covers the tokenEnv() fallback to "GITLAB_TOKEN" when no TokenEnv is configured.
func TestCheck_DefaultTokenEnv(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_TOKEN", "tok")

	require.NoError(t, gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL}).Check())
	assert.Equal(t, "tok", st.requests[0].header.Get("Private-Token"))
}

func TestCheck_Auth_Failure(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	st.status["GET"] = http.StatusUnauthorized
	t.Setenv("GITLAB_TOKEN", "bad-token")
	p := gitlab.New(&config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo", BaseURL: st.srv.URL})

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API call failed")
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
}

// In CI the job token cannot call /user (401), so Check reads the project's releases with it, by
// numeric ID when CI_PROJECT_ID is set.
func TestCheck_InCI_UsesTheJobTokenAndNoConfiguredTokenIsRequired(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", st.srv.URL)
	t.Setenv("CI_JOB_TOKEN", "jobtok")
	t.Setenv("CI_PROJECT_ID", "42")
	t.Setenv("GITLAB_TOKEN", "") // deliberately absent

	require.NoError(t, gitlab.New(&config.Platform{Project: "grp/repo"}).Check())

	require.Len(t, st.requests, 1)
	assert.Equal(t, "/api/v4/projects/42/releases", st.requests[0].path)
	assert.Equal(t, "per_page=1", st.requests[0].rawQuery)
	assert.Equal(t, "jobtok", st.requests[0].header.Get("Job-Token"))
	assert.Empty(t, st.requests[0].header.Get("Private-Token"))
}

func TestCheck_InCI_JobTokenIgnoresAConfiguredToken(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", st.srv.URL)
	t.Setenv("CI_JOB_TOKEN", "jobtok")
	t.Setenv("CI_PROJECT_ID", "42")
	t.Setenv("GITLAB_TOKEN", "pat")

	require.NoError(t, gitlab.New(&config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"}).Check())

	assert.Equal(t, "jobtok", st.requests[0].header.Get("Job-Token"), "same-host CI keeps preferring the job token")
	assert.Empty(t, st.requests[0].header.Get("Private-Token"))
}

func TestCheck_InCI_NoProjectID_UsesTheProjectPath(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", st.srv.URL)
	t.Setenv("CI_JOB_TOKEN", "jobtok")

	require.NoError(t, gitlab.New(&config.Platform{Project: "grp/repo"}).Check())

	assert.Equal(t, "/api/v4/projects/grp/repo/releases", st.requests[0].path)
}

func TestCheck_InCI_JobTokenRejected_HintsAtJobTokenPermissions(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	st.status["GET"] = http.StatusForbidden
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", st.srv.URL)
	t.Setenv("CI_JOB_TOKEN", "jobtok")
	t.Setenv("CI_PROJECT_ID", "42")

	err := gitlab.New(&config.Platform{Project: "grp/repo"}).Check()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "API call failed")
	assert.Contains(t, err.Error(), "Job token permissions")
}

// With no CI_JOB_TOKEN to use, a CI run falls back to the configured token, and fails like a local
// run when that is missing too.
func TestCheck_InCI_NoJobToken_FallsBackToTheConfiguredToken(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", st.srv.URL)
	t.Setenv("GITLAB_TOKEN", "pat")

	require.NoError(t, gitlab.New(&config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"}).Check())
	assert.Equal(t, "/api/v4/user", st.requests[0].path)
	assert.Equal(t, "pat", st.requests[0].header.Get("Private-Token"))

	t.Setenv("GITLAB_TOKEN", "")
	err := gitlab.New(&config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"}).Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
}

// ---- Self-hosted (multi-instance, ADR-0025) ----------------------------------

// A self-hosted base_url naming a different instance than the CI runner's own never gets the job
// token (it belongs to the runner's instance), so it authenticates with the configured token.
func TestCheck_SelfHosted_DifferentCIServerURL_UsesTheConfiguredToken(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", "https://gitlab.com")
	t.Setenv("CI_JOB_TOKEN", "jobtok")
	t.Setenv("CI_PROJECT_ID", "42")
	t.Setenv("GITLAB_TOKEN", "pat")
	p := gitlab.New(&config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo", BaseURL: st.srv.URL})

	require.NoError(t, p.Check())

	assert.Equal(t, "/api/v4/user", st.requests[0].path)
	assert.Equal(t, "pat", st.requests[0].header.Get("Private-Token"))
	assert.Empty(t, st.requests[0].header.Get("Job-Token"))
}

// T215: a base_url naming the CI runner's own instance is not a separately configured target, so
// the job token is trusted there and no explicit token is required.
func TestCheck_SelfHosted_MatchesCIServerURL_TrustsTheJobToken(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", st.srv.URL)
	t.Setenv("CI_JOB_TOKEN", "jobtok")
	t.Setenv("CI_PROJECT_ID", "42")
	t.Setenv("GITLAB_TOKEN", "")

	require.NoError(t, gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL}).Check())

	assert.Equal(t, "jobtok", st.requests[0].header.Get("Job-Token"))
}

func TestCreateRelease_SelfHosted_UsesTheBaseURLAndTheConfiguredToken(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL})

	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	require.Len(t, st.requests, 1)
	assert.Equal(t, "/api/v4/projects/grp/repo/releases", st.requests[0].path)
	assert.Equal(t, "tok", st.requests[0].header.Get("Private-Token"))
}

func TestCreateRelease_SelfHosted_MatchesCIServerURL_UsesTheJobToken(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", st.srv.URL)
	t.Setenv("CI_JOB_TOKEN", "jobtok")
	p := gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL})

	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	assert.Equal(t, "jobtok", st.requests[0].header.Get("Job-Token"))
	assert.Empty(t, st.requests[0].header.Get("Private-Token"))
}

// ---- CreateRelease ------------------------------------------------------------

func TestCreateRelease_Basic(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{Project: "grp/repo", TokenEnv: "GITLAB_TOKEN", BaseURL: st.srv.URL})

	require.NoError(t, p.CreateRelease("v1.2.3", "## Notes\n- thing\n", false))

	require.Len(t, st.requests, 1, "no assets: only the release call")
	rel := st.release(t)
	assert.Equal(t, "v1.2.3", rel["tag_name"])
	assert.Equal(t, "## Notes\n- thing\n", rel["description"])
}

func TestCreateRelease_InCI_UsesTheJobTokenOnTheServerURL(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", st.srv.URL)
	t.Setenv("CI_JOB_TOKEN", "ci-job-token")
	t.Setenv("GITLAB_TOKEN", "ignored")
	p := gitlab.New(&config.Platform{Project: "grp/repo", TokenEnv: "GITLAB_TOKEN"})

	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	assert.Equal(t, "ci-job-token", st.requests[0].header.Get("Job-Token"))
	assert.Empty(t, st.requests[0].header.Get("Private-Token"))
}

func TestCreateRelease_NoToken_Error(t *testing.T) {
	notInCI(t)
	t.Setenv("GITLAB_TOKEN", "")
	err := gitlab.New(&config.Platform{Project: "grp/repo"}).CreateRelease("v1.0.0", "notes", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
}

func TestCreateRelease_Assets_UploadedThenLinked(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	tmp := t.TempDir()
	t.Chdir(tmp)
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "heraut_linux"), []byte("bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "checksums.txt"), []byte("abc"), 0o644))
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{
		Project: "grp/repo",
		BaseURL: st.srv.URL,
		Assets:  []string{filepath.Join(tmp, "heraut_linux"), filepath.Join(tmp, "checksums.txt")},
	})

	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	assert.Equal(t, []string{
		"/api/v4/projects/grp/repo/packages/generic/release-assets/v1.0.0/heraut_linux",
		"/api/v4/projects/grp/repo/packages/generic/release-assets/v1.0.0/checksums.txt",
	}, st.puts())
	links := st.release(t)["assets"].(map[string]any)["links"].([]any)
	assert.Len(t, links, 2)
}

func TestCreateRelease_Assets_GlobInOneCall(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	tmp := t.TempDir()
	t.Chdir(tmp)
	for _, name := range []string{"app_linux_amd64", "app_darwin_amd64"} {
		require.NoError(t, os.WriteFile(filepath.Join(tmp, name), []byte("bin"), 0o755))
	}
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL, Assets: []string{filepath.Join(tmp, "app_*")}})

	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	assert.ElementsMatch(t, []string{
		"/api/v4/projects/grp/repo/packages/generic/release-assets/v1.0.0/app_linux_amd64",
		"/api/v4/projects/grp/repo/packages/generic/release-assets/v1.0.0/app_darwin_amd64",
	}, st.puts())
}

func TestCreateRelease_Assets_GlobSkipsDirectories(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	tmp := t.TempDir()
	t.Chdir(tmp)
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "app"), []byte("bin"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "subdir"), 0o755))
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL, Assets: []string{filepath.Join(tmp, "*")}})

	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	assert.Equal(t, []string{"/api/v4/projects/grp/repo/packages/generic/release-assets/v1.0.0/app"}, st.puts())
}

// By the time assets resolve the tag is already pushed, so a pattern matching nothing must skip
// with a warning and still create the release rather than fail it.
func TestCreateRelease_Assets_NoMatch_StillCreates(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{
		Project: "grp/repo",
		BaseURL: st.srv.URL,
		Assets:  []string{filepath.Join(t.TempDir(), "heraut_*")},
	})

	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	require.Len(t, st.requests, 1)
	assert.Empty(t, st.puts())
	assert.NotContains(t, st.release(t), "assets")
}

func TestCreateRelease_PerEnvTag_KeepsTheTagAndSanitizesThePackageVersion(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	tmp := t.TempDir()
	t.Chdir(tmp)
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "app"), []byte("bin"), 0o755))
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL, Assets: []string{filepath.Join(tmp, "app")}})

	require.NoError(t, p.CreateRelease("uat/1.0.0+b5", "notes", false))

	assert.Equal(t, []string{"/api/v4/projects/grp/repo/packages/generic/release-assets/uat-1.0.0+b5/app"}, st.puts())
	assert.Equal(t, "uat/1.0.0+b5", st.release(t)["tag_name"])
}

// TestCreateRelease_PrereleaseParamIgnored covers ADR-0064: GitLab has no pre-release concept,
// so the shared port.Platform parameter is accepted (to satisfy the contract) but never changes
// the request, for either value.
func TestCreateRelease_PrereleaseParamIgnored(t *testing.T) {
	var bodies []map[string]any
	for _, prerelease := range []bool{false, true} {
		notInCI(t)
		st := newStub(t)
		t.Setenv("GITLAB_TOKEN", "tok")
		p := gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL})
		require.NoError(t, p.CreateRelease("v1.2.3", "notes", prerelease))
		bodies = append(bodies, st.release(t))
	}
	assert.Equal(t, bodies[0], bodies[1])
}

func TestCreateRelease_ProjectFromEnv(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	t.Setenv("CI_PROJECT_PATH", "envgrp/envrepo")
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{BaseURL: st.srv.URL})

	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	assert.Equal(t, "/api/v4/projects/envgrp/envrepo/releases", st.requests[0].path)
}

func TestCreateRelease_NoProject_Error(t *testing.T) {
	notInCI(t)
	err := gitlab.New(&config.Platform{}).CreateRelease("v1.0.0", "notes", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project")
}

func TestCreateRelease_APIError_IsReturned(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	st.status["POST"] = http.StatusForbidden
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL})

	err := p.CreateRelease("v1.0.0", "notes", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gitlab release create")
}

func TestHasAssets(t *testing.T) {
	pEmpty := gitlab.New(&config.Platform{})
	assert.False(t, pEmpty.HasAssets())

	pWithAssets := gitlab.New(&config.Platform{Assets: []string{"dist/*"}})
	assert.True(t, pWithAssets.HasAssets())
}

func TestCheckAssets(t *testing.T) {
	repo := t.TempDir()
	t.Chdir(repo)
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o644))

	assert.NoError(t, gitlab.New(&config.Platform{}).CheckAssets())
	assert.NoError(t, gitlab.New(&config.Platform{Assets: []string{"dist/*"}, AllowMissingAssets: true}).CheckAssets())

	err := gitlab.New(&config.Platform{Assets: []string{filepath.Join(outside, "*.txt")}}).CheckAssets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
}

// ADR-0076: a pattern matching nothing is an error unless release.allow_missing_assets is set.
func TestCheckAssets_UnmatchedPatternIsRefusedUnlessAllowed(t *testing.T) {
	t.Chdir(t.TempDir())

	err := gitlab.New(&config.Platform{Assets: []string{"dist/*"}}).CheckAssets()
	require.ErrorIs(t, err, port.ErrAssetUnmatched)

	assert.NoError(t, gitlab.New(&config.Platform{Assets: []string{"dist/*"}, AllowMissingAssets: true}).CheckAssets())
}

// ADR-0076 addendum: the "#" rule is GitHub's (gh label syntax); GitLab uploads over REST.
func TestCheckAssets_PathWithHashIsAccepted(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.MkdirAll("dist", 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("dist", "app#1.txt"), []byte("x"), 0o644))

	assert.NoError(t, gitlab.New(&config.Platform{Assets: []string{"dist/*"}}).CheckAssets())
}
