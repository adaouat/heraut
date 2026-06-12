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
	cfg := &config.Platform{Project: "mygroup/myrepo"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://gitlab.com/mygroup/myrepo/-/releases/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestReleaseURL_FromEnv(t *testing.T) {
	t.Setenv("CI_PROJECT_PATH", "envgroup/envrepo")
	p := gitlab.New(exectest.NewMockRunner(), &config.Platform{})
	assert.Equal(t, "https://gitlab.com/envgroup/envrepo/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestReleaseURL_SelfHosted(t *testing.T) {
	cfg := &config.Platform{Project: "grp/repo", BaseURL: "https://gitlab.example.com"}
	p := gitlab.New(exectest.NewMockRunner(), cfg)
	assert.Equal(t, "https://gitlab.example.com/grp/repo/-/releases/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestLinkContext_NestedGroup(t *testing.T) {
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

func TestLinkContext_SimpleProject(t *testing.T) {
	// BaseURL empty (config not normalized) → falls back to the default host.
	cfg := &config.Platform{Project: "group/proj"}
	lc := gitlab.New(exectest.NewMockRunner(), cfg).LinkContext()
	assert.Equal(t, "group", lc.Owner)
	assert.Equal(t, "proj", lc.Repo)
	assert.Equal(t, "https://gitlab.com", lc.BaseURL)
}

func TestLinkContext_FromEnv(t *testing.T) {
	t.Setenv("CI_PROJECT_PATH", "envgroup/envrepo")
	lc := gitlab.New(exectest.NewMockRunner(), &config.Platform{}).LinkContext()
	assert.Equal(t, "envgroup", lc.Owner)
	assert.Equal(t, "envrepo", lc.Repo)
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
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)

	t.Setenv("GITLAB_TOKEN", "")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
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
	assert.Equal(t, []string{
		"release", "create", "v1.2.3",
		"--notes", "## Notes\n- thing\n",
		"-R", "grp/repo",
	}, call.Args)
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
		"-R", "grp/repo",
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
	// When CI_PROJECT_ID is unset, skip the auth check gracefully.
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_ID", "")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)
	// No second response needed — auth check is skipped

	t.Setenv("GITLAB_TOKEN", "ci-token")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})
	require.NoError(t, p.Check())
	require.Len(t, mr.Calls, 1) // only binary check
}

// ---- Self-hosted (multi-instance, ADR-0025) ----------------------------------

func TestCreateRelease_SelfHosted_SetsGitlabHostEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		BaseURL: "https://gitlab.example.com",
	})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"GITLAB_HOST=gitlab.example.com"}, mr.Calls[0].Env)
}

func TestUploadAssets_SelfHosted_SetsGitlabHostEnv(t *testing.T) {
	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "myapp")
	require.NoError(t, os.WriteFile(assetPath, []byte("binary"), 0o755))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		Assets:  []string{assetPath},
		BaseURL: "https://gitlab.example.com",
	})
	require.NoError(t, p.UploadAssets("v1.2.3"))

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"GITLAB_HOST=gitlab.example.com"}, mr.Calls[0].Env)
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
