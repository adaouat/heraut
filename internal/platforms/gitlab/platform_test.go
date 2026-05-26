package gitlab_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/gitlab"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	p := gitlab.New(testutil.NewMockRunner(), &config.Platform{})
	assert.Equal(t, "gitlab", p.Name())
}

func TestReleaseURL(t *testing.T) {
	cfg := &config.Platform{Project: "mygroup/myrepo"}
	p := gitlab.New(testutil.NewMockRunner(), cfg)
	assert.Equal(t, "https://gitlab.com/mygroup/myrepo/-/releases/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestReleaseURL_FromEnv(t *testing.T) {
	t.Setenv("CI_PROJECT_PATH", "envgroup/envrepo")
	p := gitlab.New(testutil.NewMockRunner(), &config.Platform{})
	assert.Equal(t, "https://gitlab.com/envgroup/envrepo/-/releases/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestCheck_GlabMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "glab: command not found", errors.New("exit status 127"))

	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})
	t.Setenv("GITLAB_TOKEN", "tok")

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "glab")
}

func TestCheck_TokenMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)

	t.Setenv("GITLAB_TOKEN", "")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
}

func TestCheck_ProjectMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)

	t.Setenv("GITLAB_TOKEN", "tok")
	t.Setenv("CI_PROJECT_PATH", "")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN"})

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project")
}

func TestCheck_OK(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)

	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(mr, &config.Platform{TokenEnv: "GITLAB_TOKEN", Project: "grp/repo"})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "glab", mr.Calls[0].Name)
	assert.Equal(t, []string{"--version"}, mr.Calls[0].Args)
}

func TestCreateRelease_BasicArgs(t *testing.T) {
	mr := testutil.NewMockRunner()
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


func TestHasAssets(t *testing.T) {
	pEmpty := gitlab.New(testutil.NewMockRunner(), &config.Platform{})
	assert.False(t, pEmpty.HasAssets())

	pWithAssets := gitlab.New(testutil.NewMockRunner(), &config.Platform{Assets: []string{"dist/*"}})
	assert.True(t, pWithAssets.HasAssets())
}

func TestUploadAssets_SingleFile(t *testing.T) {
	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "myapp")
	require.NoError(t, os.WriteFile(assetPath, []byte("binary"), 0o755))

	mr := testutil.NewMockRunner()
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
		"release", "upload-asset", "v1.2.3",
		assetPath,
		"-R", "grp/repo",
	}, call.Args)
}

func TestUploadAssets_Glob(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"app_linux_amd64", "app_darwin_amd64"} {
		require.NoError(t, os.WriteFile(filepath.Join(tmp, name), []byte("bin"), 0o755))
	}

	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		Assets:  []string{filepath.Join(tmp, "app_*")},
	})
	require.NoError(t, p.UploadAssets("v1.0.0"))

	assert.Len(t, mr.Calls, 2)
	for _, call := range mr.Calls {
		assert.Equal(t, "glab", call.Name)
		assert.Equal(t, "release", call.Args[0])
		assert.Equal(t, "upload-asset", call.Args[1])
		assert.Equal(t, "v1.0.0", call.Args[2])
	}
}

func TestUploadAssets_GlobNoMatch(t *testing.T) {
	mr := testutil.NewMockRunner()
	p := gitlab.New(mr, &config.Platform{
		Project: "grp/repo",
		Assets:  []string{"/tmp/nonexistent/heraut_*"},
	})
	err := p.UploadAssets("v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no files matched")
}

func TestCreateRelease_ProjectFromEnv(t *testing.T) {
	t.Setenv("CI_PROJECT_PATH", "envgrp/envrepo")
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := gitlab.New(mr, &config.Platform{})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	call := mr.Calls[0]
	assert.Contains(t, call.Args, "envgrp/envrepo")
}

func TestCreateRelease_NoProject_Error(t *testing.T) {
	t.Setenv("CI_PROJECT_PATH", "")
	mr := testutil.NewMockRunner()

	p := gitlab.New(mr, &config.Platform{})
	err := p.CreateRelease("v1.0.0", "notes")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project")
}

// TestCheck_DefaultTokenEnv covers the tokenEnv() fallback to "GITLAB_TOKEN" when no TokenEnv is configured.
func TestCheck_DefaultTokenEnv(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("glab version 1.0.0", "", nil)

	t.Setenv("GITLAB_TOKEN", "tok")
	// No TokenEnv configured — should fall back to "GITLAB_TOKEN" default
	p := gitlab.New(mr, &config.Platform{Project: "grp/repo"})
	require.NoError(t, p.Check())
}
