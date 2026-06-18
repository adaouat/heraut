package gitlab_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/gitlab"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	p := gitlab.New(exectest.NewMockRunner(), &config.Platform{Name: "gitlab-internal"})
	assert.Equal(t, "gitlab-internal", p.Name())
}

func TestReleaseURL(t *testing.T) {
	t.Setenv("GITLAB_CI", "") // ensure CI_SERVER_URL fallback is not active
	cfg := &config.Platform{Project: "mygroup/myrepo"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://gitlab.com/mygroup/myrepo/-/releases/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestReleaseURL_FromEnv(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CI_PROJECT_PATH", "envgroup/envrepo")
	p := gitlab.New(exectest.NewMockRunner(), &config.Platform{})
	assert.Equal(t, "https://gitlab.com/envgroup/envrepo/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestReleaseURL_SelfHosted(t *testing.T) {
	cfg := &config.Platform{Project: "grp/repo", BaseURL: "https://gitlab.example.com"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://gitlab.example.com/grp/repo/-/releases/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestReleaseURL_InCI_NoBaseURL_UsesServerURL(t *testing.T) {
	// When BaseURL is not configured and running in GitLab CI, the release link
	// must use CI_SERVER_URL (the actual GitLab instance) instead of gitlab.com.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", "https://git.example.com")
	cfg := &config.Platform{Project: "grp/repo"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://git.example.com/grp/repo/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestReleaseURL_InCI_NoBaseURL_NoServerURL_FallsBackToProjectURL(t *testing.T) {
	// GITLAB_CI=true, CI_SERVER_URL absent — CI_PROJECT_URL carries the host.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_SERVER_URL", "")
	t.Setenv("CI_PROJECT_URL", "https://git.adaouat.dev/bchatard/ecom-poc-release")
	cfg := &config.Platform{Project: "bchatard/ecom-poc-release"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://git.adaouat.dev/bchatard/ecom-poc-release/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestReleaseURL_NoGitlabCIVar_ProjectURLSuffices(t *testing.T) {
	// CI_PROJECT_URL set but GITLAB_CI absent/false: some CI containers and custom
	// setups expose CI_PROJECT_URL without the GITLAB_CI marker. resolveBaseURL must
	// treat CI_PROJECT_URL as the authoritative source regardless — consistent with how
	// ambientLinkContext() uses it unconditionally for notes/changelog link resolution.
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CI_SERVER_URL", "")
	t.Setenv("CI_PROJECT_URL", "https://git.adaouat.dev/bchatard/ecom-poc-release")
	cfg := &config.Platform{Project: "bchatard/ecom-poc-release"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://git.adaouat.dev/bchatard/ecom-poc-release/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

// TestReleaseURLFromContext_* verify that ReleaseURLFromContext derives the URL from the
// provided LinkContext rather than re-reading environment variables. This is the "one
// place" fix: the pipeline passes the already-resolved platformLinkContext here so the
// displayed URL is always consistent with the commit links in the release notes.

func TestReleaseURLFromContext_AmbientContext(t *testing.T) {
	// Ambient context (from CI_PROJECT_URL): Owner/Repo are empty, BaseURL is the
	// full project URL. The release URL is built directly from BaseURL.
	cfg := &config.Platform{Project: "bchatard/ecom-poc-release"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	lc := &port.LinkContext{BaseURL: "https://git.adaouat.dev/bchatard/ecom-poc-release", Platform: "gitlab"}
	assert.Equal(t, "https://git.adaouat.dev/bchatard/ecom-poc-release/-/releases/v1.0.0", p.ReleaseURLFromContext("v1.0.0", lc))
}

func TestReleaseURLFromContext_PlatformContext(t *testing.T) {
	// Platform-derived context: BaseURL is just the host, Owner/Repo are populated.
	// The release URL is assembled from host + configured project path.
	cfg := &config.Platform{Project: "bchatard/ecom-poc-release"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	lc := &port.LinkContext{BaseURL: "https://git.adaouat.dev", Owner: "bchatard", Repo: "ecom-poc-release", Platform: "gitlab"}
	assert.Equal(t, "https://git.adaouat.dev/bchatard/ecom-poc-release/-/releases/v1.0.0", p.ReleaseURLFromContext("v1.0.0", lc))
}

func TestReleaseURLFromContext_Nil_FallsBackToReleaseURL(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CI_PROJECT_URL", "")
	cfg := &config.Platform{Project: "grp/repo"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, p.ReleaseURL("v1.0.0"), p.ReleaseURLFromContext("v1.0.0", nil))
}

func TestLinkContext_NestedGroup(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	// GitLab project paths split on the LAST slash: group/subgroup is the owner, the
	// final segment is the repo.
	cfg := &config.Platform{Project: "group/sub/proj", BaseURL: "https://gitlab.com"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, port.LinkContext{
		BaseURL:  "https://gitlab.com",
		Owner:    "group/sub",
		Repo:     "proj",
		Platform: "gitlab",
	}, p.LinkContext())
}

func TestLinkContext_Token(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "gltoken")
	cfg := &config.Platform{Project: "grp/repo", TokenEnv: "GITLAB_TOKEN"}
	lc := gitlab.New(exectest.NewMockRunner(), cfg).LinkContext()
	assert.Equal(t, "gltoken", lc.Token)
}

func TestLinkContext_Token_CustomEnv(t *testing.T) {
	t.Setenv("CORP_GL_TOKEN", "corptoken")
	cfg := &config.Platform{Project: "grp/repo", TokenEnv: "CORP_GL_TOKEN"}
	lc := gitlab.New(exectest.NewMockRunner(), cfg).LinkContext()
	assert.Equal(t, "corptoken", lc.Token)
}

func TestLinkContext_SimpleProject(t *testing.T) {
	t.Setenv("GITLAB_CI", "") // ensure CI_SERVER_URL fallback is not active
	t.Setenv("GITLAB_TOKEN", "")
	// BaseURL empty (config not normalized) → falls back to the default host.
	cfg := &config.Platform{Project: "group/proj"}
	lc := gitlab.New(exectest.NewMockRunner(), cfg).LinkContext()
	assert.Equal(t, "group", lc.Owner)
	assert.Equal(t, "proj", lc.Repo)
	assert.Equal(t, "https://gitlab.com", lc.BaseURL)
}

func TestLinkContext_FromEnv(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_PROJECT_PATH", "envgroup/envrepo")
	lc := gitlab.New(exectest.NewMockRunner(), &config.Platform{}).LinkContext()
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
	lc := gitlab.New(exectest.NewMockRunner(), cfg).LinkContext()
	assert.Equal(t, "https://git.example.com", lc.BaseURL)
}

func TestCheck_GlabMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "glab: command not found", errors.New("exit status 127"))

	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})
	t.Setenv("GITLAB_TOKEN", "tok")

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "glab")
}

func TestCheck_TokenMissing(t *testing.T) {
	t.Setenv("GITLAB_CI", "") // ensure CI autologin is not active
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)

	t.Setenv("GITLAB_TOKEN", "")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
}

func TestCheck_InCI_TokenNotRequired(t *testing.T) {
	// In CI autologin mode glab authenticates via JOB-TOKEN; heraut must not require
	// an explicit token env var when GITLAB_CI=true and the platform is not self-hosted.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_ID", "42")
	t.Setenv("GITLAB_TOKEN", "") // deliberately absent

	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)
	mr.QueueResponse(`[]`, "", nil) // CI auth check via numeric project ID

	p := gitlab.New(mr, &config.Platform{Project: "grp/repo"})
	require.NoError(t, p.Check())
}

func TestCheck_ProjectMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)

	t.Setenv("GITLAB_TOKEN", "tok")
	t.Setenv("CI_PROJECT_PATH", "")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN"})

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project")
}

func TestCheck_OK(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)
	mr.QueueResponse(`{"username":"alice"}`, "", nil) // auth check

	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"--version"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"api", "user"}, mr.Calls[1].Args)
}

func TestCreateRelease_BasicArgs(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{Project: "grp/repo", TokenEnv: "GITLAB_TOKEN"})
	require.NoError(t, p.CreateRelease("v1.2.3", "## Notes\n- thing\n"))

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "glab", call.Name)
	// Notes are written to a temp file to avoid ARG_MAX limits on large changelogs.
	require.Equal(t, []string{"release", "create", "v1.2.3"}, call.Args[:3])
	assert.Equal(t, "--notes-file", call.Args[3])
	assert.NotEmpty(t, call.Args[4], "notes file path must be non-empty")
	assert.Equal(t, []string{"--repo", "grp/repo"}, call.Args[5:])
	assert.NotContains(t, call.Args, "--notes")
}

func TestCreateRelease_LenientAssets_IncludesFilesInCreate(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "heraut_linux"), []byte("bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "checksums.txt"), []byte("abc"), 0o644))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project:       "grp/repo",
		Assets:        []string{filepath.Join(tmp, "heraut_linux"), filepath.Join(tmp, "checksums.txt")},
		LenientAssets: true,
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "glab", call.Name)
	assert.Equal(t, "release", call.Args[0])
	assert.Equal(t, "create", call.Args[1])
	assert.Contains(t, call.Args, filepath.Join(tmp, "heraut_linux"))
	assert.Contains(t, call.Args, filepath.Join(tmp, "checksums.txt"))
}

func TestUploadAssets_LenientGlobs_IsNoop(t *testing.T) {
	mr := exectest.NewMockRunner()
	p := gitlab.New(mr, &config.Platform{
		Project:       "grp/repo",
		Assets:        []string{"dist/heraut_*"},
		LenientAssets: true,
	})
	require.NoError(t, p.UploadAssets("v1.0.0"))
	assert.Empty(t, mr.Calls)
}

func TestHasAssets(t *testing.T) {
	pEmpty := gitlab.New(exectest.NewMockRunner(), &config.Platform{})
	assert.False(t, pEmpty.HasAssets())

	pWithAssets := gitlab.New(exectest.NewMockRunner(), &config.Platform{Assets: []string{"dist/*"}})
	assert.True(t, pWithAssets.HasAssets())
}

func TestUploadAssets_SingleFile(t *testing.T) {
	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "myapp")
	require.NoError(t, os.WriteFile(assetPath, []byte("binary"), 0o755))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		Assets:  []string{assetPath},
	})
	require.NoError(t, p.UploadAssets("v1.2.3"))

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "glab", call.Name)
	assert.Equal(t, []string{
		"release", "upload", "v1.2.3",
		"--use-package-registry",
		"--repo", "grp/repo",
		assetPath,
	}, call.Args)
}

func TestUploadAssets_Glob(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"app_linux_amd64", "app_darwin_amd64"} {
		require.NoError(t, os.WriteFile(filepath.Join(tmp, name), []byte("bin"), 0o755))
	}

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // one batch call

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		Assets:  []string{filepath.Join(tmp, "app_*")},
	})
	require.NoError(t, p.UploadAssets("v1.0.0"))

	require.Len(t, mr.Calls, 1) // all files in one call
	call := mr.Calls[0]
	assert.Equal(t, "glab", call.Name)
	assert.Equal(t, "release", call.Args[0])
	assert.Equal(t, "upload", call.Args[1])
	assert.Equal(t, "v1.0.0", call.Args[2])
	assert.Contains(t, call.Args, "--use-package-registry")
}

func TestUploadAssets_GlobSkipsDirectories(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "app"), []byte("bin"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "subdir"), 0o755))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		Assets:  []string{filepath.Join(tmp, "*")},
	})
	require.NoError(t, p.UploadAssets("v1.0.0"))

	require.Len(t, mr.Calls, 1)
	assert.Contains(t, mr.Calls[0].Args, filepath.Join(tmp, "app"))
	assert.NotContains(t, mr.Calls[0].Args, filepath.Join(tmp, "subdir"))
}

func TestUploadAssets_GlobNoMatch(t *testing.T) {
	mr := exectest.NewMockRunner()
	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		Assets:  []string{"/tmp/nonexistent/heraut_*"},
	})
	err := p.UploadAssets("v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no files matched")
}

func TestUploadAssets_LenientGlobs_NoMatch_Warns(t *testing.T) {
	mr := exectest.NewMockRunner()
	p := gitlab.New(mr, &config.Platform{
		Project:       "grp/repo",
		Assets:        []string{"/tmp/nonexistent/heraut_*"},
		LenientAssets: true,
	})
	require.NoError(t, p.UploadAssets("v1.0.0"))
	assert.Empty(t, mr.Calls)
}

func TestUploadAssets_LenientGlobs_WithMatch_IsNoop(t *testing.T) {
	mr := exectest.NewMockRunner()
	p := gitlab.New(mr, &config.Platform{
		Project:       "grp/repo",
		Assets:        []string{"dist/heraut_*"},
		LenientAssets: true,
	})
	require.NoError(t, p.UploadAssets("v1.0.0"))
	assert.Empty(t, mr.Calls)
}

func TestCreateRelease_ProjectFromEnv(t *testing.T) {
	t.Setenv("CI_PROJECT_PATH", "envgrp/envrepo")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	call := mr.Calls[0]
	assert.Contains(t, call.Args, "envgrp/envrepo")
}

func TestCreateRelease_NoProject_Error(t *testing.T) {
	t.Setenv("CI_PROJECT_PATH", "")
	mr := exectest.NewMockRunner()

	p := gitlab.New(mr, &config.Platform{})
	err := p.CreateRelease("v1.0.0", "notes")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project")
}

// TestCheck_DefaultTokenEnv covers the tokenEnv() fallback to "GITLAB_TOKEN" when no TokenEnv is configured.
func TestCheck_DefaultTokenEnv(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)
	mr.QueueResponse(`{"username":"alice"}`, "", nil) // auth check

	t.Setenv("GITLAB_TOKEN", "tok")
	// No TokenEnv configured — should fall back to "GITLAB_TOKEN" default
	p := gitlab.New(mr, &config.Platform{Project: "grp/repo"})
	require.NoError(t, p.Check())
}

// ---- Auth check -------------------------------------------------------------

func TestCheck_OK_WithAuth(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)   // --version
	mr.QueueResponse(`{"username":"alice"}`, "", nil) // glab api user

	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"--version"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"api", "user"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"GITLAB_TOKEN=tok"}, mr.Calls[1].Env)
}

func TestCheck_Auth_Failure(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)
	mr.QueueResponse("", "401 Unauthorized", errors.New("exit status 1"))

	t.Setenv("GITLAB_TOKEN", "bad-token")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API call failed")
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
}

func TestCheck_Auth_InCI_OK(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_ID", "42")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)
	mr.QueueResponse(`[]`, "", nil) // glab api projects/42/releases?per_page=1

	t.Setenv("GITLAB_TOKEN", "ci-token")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"api", "projects/42/releases?per_page=1"}, mr.Calls[1].Args)
	// In CI, no token injection — glab uses its own CI autologin
	assert.Nil(t, mr.Calls[1].Env)
}

func TestCheck_Auth_InCI_NoProjectID(t *testing.T) {
	// CI_PROJECT_ID absent: fall through to validate the configured token instead
	// of silently skipping the auth check.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_ID", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)
	mr.QueueResponse(`{"username":"alice"}`, "", nil) // configured-token fallback API call

	t.Setenv("GITLAB_TOKEN", "ci-token")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})
	require.NoError(t, p.Check())
	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"api", "user"}, mr.Calls[1].Args)
	assert.Contains(t, mr.Calls[1].Env, "GITLAB_TOKEN=ci-token")
}

// ---- Self-hosted (multi-instance, ADR-0025) ----------------------------------

func TestCreateRelease_SelfHosted_SetsGitlabHostEnv(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "tok")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		BaseURL: "https://gitlab.example.com",
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"GITLAB_TOKEN=tok", "GITLAB_HOST=gitlab.example.com"}, mr.Calls[0].Env)
}

func TestUploadAssets_SelfHosted_SetsGitlabHostEnv(t *testing.T) {
	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "myapp")
	require.NoError(t, os.WriteFile(assetPath, []byte("binary"), 0o755))

	t.Setenv("GITLAB_TOKEN", "tok")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		Assets:  []string{assetPath},
		BaseURL: "https://gitlab.example.com",
	})
	require.NoError(t, p.UploadAssets("v1.2.3"))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"GITLAB_TOKEN=tok", "GITLAB_HOST=gitlab.example.com"}, mr.Calls[0].Env)
}

func TestCreateRelease_InCI_NoEnvInjection(t *testing.T) {
	// When GITLAB_CI=true and not self-hosted, heraut must not inject GITLAB_TOKEN.
	// Injecting it switches glab from JOB-TOKEN to PRIVATE-TOKEN auth, breaking
	// path-based project lookups on non-default gitlab.com runners.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_JOB_TOKEN", "ci-job-token")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{Project: "grp/repo", TokenEnv: "CI_JOB_TOKEN"})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	require.Len(t, mr.Calls, 1)
	assert.Nil(t, mr.Calls[0].Env, "must not inject env in CI autologin mode")
}

func TestUploadAssets_InCI_NoEnvInjection(t *testing.T) {
	// Same as CreateRelease: CI autologin mode must not inject GITLAB_TOKEN.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_JOB_TOKEN", "ci-job-token")

	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "myapp")
	require.NoError(t, os.WriteFile(assetPath, []byte("binary"), 0o755))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{Project: "grp/repo", Assets: []string{assetPath}, TokenEnv: "CI_JOB_TOKEN"})
	require.NoError(t, p.UploadAssets("v1.0.0"))

	require.Len(t, mr.Calls, 1)
	assert.Nil(t, mr.Calls[0].Env, "must not inject env in CI autologin mode")
}

func TestCheck_SelfHosted_SkipsCIAutologin(t *testing.T) {
	// Even when GITLAB_CI=true, a self-hosted instance must not rely on glab's CI
	// autologin (which targets the gitlab.com job token) — it always authenticates
	// via the configured token, with GITLAB_HOST pointing glab at the right host.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_ID", "42")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)   // --version
	mr.QueueResponse(`{"username":"alice"}`, "", nil) // glab api user

	t.Setenv("GITLAB_TOKEN", "ci-token")
	p := gitlab.New(mr, &config.Platform{
		TokenEnv: "GITLAB_TOKEN",
		Project:  "grp/repo",
		BaseURL:  "https://gitlab.example.com",
	})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"api", "user"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"GITLAB_TOKEN=ci-token", "GITLAB_HOST=gitlab.example.com"}, mr.Calls[1].Env)
}
