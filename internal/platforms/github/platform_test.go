package github_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/github"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	p := github.New(exectest.NewMockRunner(), &config.Platform{Name: "github-internal"})
	assert.Equal(t, "github-internal", p.Name())
}

func TestReleaseURL(t *testing.T) {
	cfg := &config.Platform{Repository: "myorg/myrepo"}
	p := github.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://github.com/myorg/myrepo/releases/tag/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestReleaseURL_FromEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
	p := github.New(exectest.NewMockRunner(), &config.Platform{})
	assert.Equal(t, "https://github.com/envorg/envrepo/releases/tag/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestReleaseURL_SelfHosted(t *testing.T) {
	cfg := &config.Platform{Repository: "org/repo", BaseURL: "https://github.example.com"}
	p := github.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://github.example.com/org/repo/releases/tag/v1.2.3", p.ReleaseURL("v1.2.3"))
}

// TestReleaseURLFromContext_* verify that ReleaseURLFromContext derives the release URL from
// a pre-resolved link context rather than re-resolving from config/env, so the pipeline can
// keep the displayed URL consistent with the one used to generate release notes (ADR-0022).
func TestReleaseURLFromContext_AmbientContext(t *testing.T) {
	// Ambient context: BaseURL is the full project URL (Owner/Repo empty) — built from
	// GITHUB_SERVER_URL + GITHUB_REPOSITORY in ambientLinkContext().
	cfg := &config.Platform{Repository: "test/gh-repo"}
	p := github.New(exectest.NewMockRunner(), cfg)
	lc := &port.LinkContext{BaseURL: "https://github.com/test/gh-repo", Platform: "github"}
	assert.Equal(t, "https://github.com/test/gh-repo/releases/tag/v1.0.0", p.ReleaseURLFromContext("v1.0.0", lc))
}

func TestReleaseURLFromContext_PlatformContext(t *testing.T) {
	// Platform context: BaseURL is the host (Owner/Repo set) — built from LinkContext().
	cfg := &config.Platform{Repository: "test/gh-repo"}
	p := github.New(exectest.NewMockRunner(), cfg)
	lc := &port.LinkContext{BaseURL: "https://github.com", Owner: "test", Repo: "gh-repo", Platform: "github"}
	assert.Equal(t, "https://github.com/test/gh-repo/releases/tag/v1.0.0", p.ReleaseURLFromContext("v1.0.0", lc))
}

func TestReleaseURLFromContext_Nil_FallsBackToReleaseURL(t *testing.T) {
	cfg := &config.Platform{Repository: "org/repo"}
	p := github.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, p.ReleaseURL("v1.0.0"), p.ReleaseURLFromContext("v1.0.0", nil))
}

// Tags may carry SemVer build metadata ("+"), which must reach the URL as %2B (ADR-0064).
func TestReleaseURL_EscapesPlusInTag(t *testing.T) {
	p := github.New(exectest.NewMockRunner(), &config.Platform{Repository: "acme/widget"})
	assert.Equal(t, "https://github.com/acme/widget/releases/tag/v1.4.0%2B158404", p.ReleaseURL("v1.4.0+158404"))

	ambient := &port.LinkContext{BaseURL: "https://github.com/acme/widget", Platform: "github"}
	assert.Equal(t, "https://github.com/acme/widget/releases/tag/uat/7.4.1%2B158404", p.ReleaseURLFromContext("uat/7.4.1+158404", ambient))

	platform := &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"}
	assert.Equal(t, "https://github.com/acme/widget/releases/tag/v1.4.0%2B158404", p.ReleaseURLFromContext("v1.4.0+158404", platform))
}

func TestLinkContext(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	cfg := &config.Platform{Repository: "acme/widget", BaseURL: "https://github.com"}
	p := github.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, port.LinkContext{
		BaseURL:  "https://github.com",
		Owner:    "acme",
		Repo:     "widget",
		Platform: "github",
	}, p.LinkContext())
}

func TestLinkContext_DefaultBaseURL(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	// BaseURL empty (config not normalized) → falls back to the default host.
	cfg := &config.Platform{Repository: "acme/widget"}
	lc := github.New(exectest.NewMockRunner(), cfg).LinkContext()
	assert.Equal(t, "https://github.com", lc.BaseURL)
}

func TestLinkContext_FromEnv(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
	lc := github.New(exectest.NewMockRunner(), &config.Platform{}).LinkContext()
	assert.Equal(t, "envorg", lc.Owner)
	assert.Equal(t, "envrepo", lc.Repo)
}

func TestCheck_GhMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "gh: command not found", errors.New("exit status 127"))

	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN"})
	t.Setenv("GH_TOKEN", "tok")
	t.Setenv("GITHUB_REPOSITORY", "org/repo")

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh")
}

func TestCheck_TokenMissing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)

	// ensure GH_TOKEN is not set
	t.Setenv("GH_TOKEN", "")

	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN", Repository: "org/repo"})
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GH_TOKEN")
}

func TestCheck_RepositoryMissing(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)

	t.Setenv("GH_TOKEN", "tok")
	t.Setenv("GITHUB_REPOSITORY", "") // CI sets this automatically; clear it for this test
	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN"})
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
}

func TestCheck_OK(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)
	mr.QueueResponse(`[]`, "", nil) // API auth check

	t.Setenv("GH_TOKEN", "tok")
	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN", Repository: "org/repo"})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"--version"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"api", "repos/org/repo/releases?per_page=1"}, mr.Calls[1].Args)
	assert.Contains(t, mr.Calls[1].Env, "GH_TOKEN=tok")
}

func TestCheck_APIAuthFails(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)
	mr.QueueResponse("", "bad credentials", errors.New("exit status 1"))

	t.Setenv("GH_TOKEN", "badtoken")
	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN", Repository: "org/repo"})
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API call failed")
}

// TestCheck_NonGitHubActions_UsesExplicitRepo ensures that outside GITHUB_ACTIONS (e.g.
// GitLab CI triggering a GitHub release), the auth probe uses the explicit owner/repo from
// config rather than {owner}/{repo} placeholders. gh resolves placeholders from git remotes,
// which fails when the remote points to GitLab, not GitHub.
func TestCheck_NonGitHubActions_UsesExplicitRepo(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_REPOSITORY", "") // not set — simulating non-GitHub CI
	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)
	mr.QueueResponse(`[]`, "", nil) // API auth check

	t.Setenv("GH_TOKEN", "tok")
	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN", Repository: "org/repo"})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"api", "repos/org/repo/releases?per_page=1"}, mr.Calls[1].Args)
}

func TestCheck_GitHubActions_OK(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_TOKEN", "ghtoken")
	t.Setenv("GITHUB_REPOSITORY", "org/repo")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)
	mr.QueueResponse(`[]`, "", nil)

	t.Setenv("GH_TOKEN", "tok")
	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN", Repository: "org/repo"})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	apiCall := mr.Calls[1]
	assert.Equal(t, []string{"api", "repos/org/repo/releases?per_page=1"}, apiCall.Args)
	assert.Contains(t, apiCall.Env, "GH_TOKEN=ghtoken") // uses GITHUB_TOKEN, not token_env
}

func TestCheck_GitHubActions_AuthFails(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_TOKEN", "badtoken")
	t.Setenv("GITHUB_REPOSITORY", "org/repo")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)
	mr.QueueResponse("", "bad credentials", errors.New("exit status 1"))

	t.Setenv("GH_TOKEN", "tok")
	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN", Repository: "org/repo"})
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITHUB_TOKEN")
}

func TestCheck_GitHubActions_NoGITHUB_TOKEN(t *testing.T) {
	// GITHUB_TOKEN absent (e.g. user uses a PAT via token_env): fall through to
	// validate the configured token instead of silently skipping the auth check.
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_TOKEN", "") // not available
	t.Setenv("GITHUB_REPOSITORY", "org/repo")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)
	mr.QueueResponse(`[]`, "", nil) // configured-token fallback API call

	t.Setenv("GH_TOKEN", "tok")
	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN", Repository: "org/repo"})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"api", "repos/org/repo/releases?per_page=1"}, mr.Calls[1].Args)
	assert.Contains(t, mr.Calls[1].Env, "GH_TOKEN=tok")
}

func TestCreateRelease_BasicArgs(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{Repository: "org/repo", TokenEnv: "GH_TOKEN"})
	require.NoError(t, p.CreateRelease("v1.2.3", "## Notes\n- thing\n", false))

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "gh", call.Name)
	// Notes are written to a temp file to avoid ARG_MAX limits on large changelogs.
	require.Equal(t, []string{"release", "create", "v1.2.3"}, call.Args[:3])
	assert.Equal(t, "--notes-file", call.Args[3])
	assert.NotEmpty(t, call.Args[4], "notes file path must be non-empty")
	assert.Equal(t, []string{"--repo", "org/repo"}, call.Args[5:])
	assert.NotContains(t, call.Args, "--notes")
}

func TestCreateRelease_Draft(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{Repository: "org/repo", Draft: true})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	call := mr.Calls[0]
	assert.Contains(t, call.Args, "--draft")
	assert.NotContains(t, call.Args, "--prerelease")
}

// TestCreateRelease_Prerelease covers ADR-0064: the --prerelease flag is derived by the caller
// from the resolved version (not read from static config) and passed as a parameter.
func TestCreateRelease_Prerelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{Repository: "org/repo"})
	require.NoError(t, p.CreateRelease("v1.0.0-rc.1", "notes", true))

	call := mr.Calls[0]
	assert.Contains(t, call.Args, "--prerelease")
}

func TestCreateRelease_Assets_IncludesFilesInCreate(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "heraut_linux"), []byte("bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "checksums.txt"), []byte("abc"), 0o644))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		Assets:     []string{filepath.Join(tmp, "heraut_linux"), filepath.Join(tmp, "checksums.txt")},
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "gh", call.Name)
	assert.Equal(t, "release", call.Args[0])
	assert.Equal(t, "create", call.Args[1])
	// Asset files must be included in the create call (avoids GitHub HTTP 422 on upload)
	assert.Contains(t, call.Args, filepath.Join(tmp, "heraut_linux"))
	assert.Contains(t, call.Args, filepath.Join(tmp, "checksums.txt"))
}

func TestCreateRelease_DraftAndPrerelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{Repository: "org/repo", Draft: true})
	require.NoError(t, p.CreateRelease("v1.0.0-rc.1", "notes", true))

	call := mr.Calls[0]
	assert.Contains(t, call.Args, "--draft")
	assert.Contains(t, call.Args, "--prerelease")
}

func TestHasAssets(t *testing.T) {
	pEmpty := github.New(exectest.NewMockRunner(), &config.Platform{})
	assert.False(t, pEmpty.HasAssets())

	pWithAssets := github.New(exectest.NewMockRunner(), &config.Platform{Assets: []string{"dist/*"}})
	assert.True(t, pWithAssets.HasAssets())
}

func TestCreateRelease_RepoFromEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	call := mr.Calls[0]
	assert.Contains(t, call.Args, "envorg/envrepo")
}

func TestCreateRelease_NoRepo_Error(t *testing.T) {
	// Clear env var
	t.Setenv("GITHUB_REPOSITORY", "")
	mr := exectest.NewMockRunner()

	p := github.New(mr, &config.Platform{})
	err := p.CreateRelease("v1.0.0", "notes", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
}

// TestCheck_DefaultTokenEnv covers the tokenEnv() fallback to "GH_TOKEN" when no TokenEnv is configured.
func TestCheck_DefaultTokenEnv(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)
	mr.QueueResponse(`[]`, "", nil) // API auth check

	t.Setenv("GH_TOKEN", "tok")
	// No TokenEnv configured — should fall back to "GH_TOKEN" default
	p := github.New(mr, &config.Platform{Repository: "org/repo"})
	require.NoError(t, p.Check())
}

func TestCreateRelease_TokenForwarded(t *testing.T) {
	t.Setenv("CORP_TOKEN", "secret123")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{Repository: "org/repo", TokenEnv: "CORP_TOKEN"})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	require.Len(t, mr.Calls, 1)
	assert.Contains(t, mr.Calls[0].Env, "GH_TOKEN=secret123")
}

func TestCreateRelease_Assets_GlobInOneCall(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"app_linux_amd64", "app_darwin_amd64"} {
		require.NoError(t, os.WriteFile(filepath.Join(tmp, name), []byte("bin"), 0o755))
	}

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		Assets:     []string{filepath.Join(tmp, "app_*")},
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, []string{"release", "create", "v1.0.0"}, call.Args[:3])
	assert.Contains(t, call.Args, filepath.Join(tmp, "app_linux_amd64"))
	assert.Contains(t, call.Args, filepath.Join(tmp, "app_darwin_amd64"))
}

func TestCreateRelease_Assets_GlobSkipsDirectories(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "app"), []byte("bin"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "subdir"), 0o755))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		Assets:     []string{filepath.Join(tmp, "*")},
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	require.Len(t, mr.Calls, 1)
	assert.Contains(t, mr.Calls[0].Args, filepath.Join(tmp, "app"))
	assert.NotContains(t, mr.Calls[0].Args, filepath.Join(tmp, "subdir"))
}

// By the time assets resolve the tag is already pushed, so a pattern matching nothing must skip
// with a warning and still create the release rather than fail it.
func TestCreateRelease_Assets_NoMatch_StillCreates(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		Assets:     []string{filepath.Join(t.TempDir(), "heraut_*")},
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"--repo", "org/repo"}, mr.Calls[0].Args[5:])
}

// ---- Self-hosted (multi-instance, ADR-0025) ----------------------------------

func TestCreateRelease_SelfHosted_SetsGhHostEnv(t *testing.T) {
	t.Setenv("GH_TOKEN", "ent-token")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		BaseURL:    "https://github.example.com",
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes", false))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"GH_TOKEN=ent-token", "GH_HOST=github.example.com", "GH_ENTERPRISE_TOKEN=ent-token"}, mr.Calls[0].Env)
}

func TestCheck_SelfHosted_SkipsActionsAutologin(t *testing.T) {
	// Even when GITHUB_ACTIONS=true, a self-hosted GHES instance must not rely on the
	// GITHUB_TOKEN-based autologin (which targets api.github.com) — it always
	// authenticates via the configured token, with GH_HOST/GH_ENTERPRISE_TOKEN pointing
	// gh at the right host.
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_TOKEN", "actions-token")
	t.Setenv("GITHUB_REPOSITORY", "org/repo")

	mr := exectest.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)
	mr.QueueResponse(`[]`, "", nil)

	t.Setenv("GH_TOKEN", "ent-token")
	p := github.New(mr, &config.Platform{
		TokenEnv:   "GH_TOKEN",
		Repository: "org/repo",
		BaseURL:    "https://github.example.com",
	})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"api", "repos/org/repo/releases?per_page=1"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"GH_TOKEN=ent-token", "GH_HOST=github.example.com", "GH_ENTERPRISE_TOKEN=ent-token"}, mr.Calls[1].Env)
}
