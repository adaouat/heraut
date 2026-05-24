package github_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/github"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	p := github.New(testutil.NewMockRunner(), &config.Platform{})
	assert.Equal(t, "github", p.Name())
}

func TestReleaseURL(t *testing.T) {
	cfg := &config.Platform{Repository: "myorg/myrepo"}
	p := github.New(testutil.NewMockRunner(), cfg)
	assert.Equal(t, "https://github.com/myorg/myrepo/releases/tag/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestReleaseURL_FromEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
	p := github.New(testutil.NewMockRunner(), &config.Platform{})
	assert.Equal(t, "https://github.com/envorg/envrepo/releases/tag/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestCheck_GhMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "gh: command not found", errors.New("exit status 127"))

	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN"})
	t.Setenv("GH_TOKEN", "tok")
	t.Setenv("GITHUB_REPOSITORY", "org/repo")

	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh")
}

func TestCheck_TokenMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)

	// ensure GH_TOKEN is not set
	t.Setenv("GH_TOKEN", "")

	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN", Repository: "org/repo"})
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GH_TOKEN")
}

func TestCheck_RepositoryMissing(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)

	t.Setenv("GH_TOKEN", "tok")
	// No repository in config or env
	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN"})
	err := p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
}

func TestCheck_OK(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("gh version 2.0.0", "", nil)

	t.Setenv("GH_TOKEN", "tok")
	p := github.New(mr, &config.Platform{TokenEnv: "GH_TOKEN", Repository: "org/repo"})
	require.NoError(t, p.Check())

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "gh", mr.Calls[0].Name)
	assert.Equal(t, []string{"--version"}, mr.Calls[0].Args)
}

func TestCreateRelease_BasicArgs(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{Repository: "org/repo", TokenEnv: "GH_TOKEN"})
	require.NoError(t, p.CreateRelease("v1.2.3", "## Notes\n- thing\n"))

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "gh", call.Name)
	assert.Equal(t, []string{
		"release", "create", "v1.2.3",
		"--notes", "## Notes\n- thing\n",
		"--repo", "org/repo",
	}, call.Args)
}

func TestCreateRelease_Draft(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{Repository: "org/repo", Draft: true})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	call := mr.Calls[0]
	assert.Contains(t, call.Args, "--draft")
	assert.NotContains(t, call.Args, "--prerelease")
}

func TestCreateRelease_Prerelease(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{Repository: "org/repo", Prerelease: true})
	require.NoError(t, p.CreateRelease("v1.0.0-rc.1", "notes"))

	call := mr.Calls[0]
	assert.Contains(t, call.Args, "--prerelease")
}

func TestHasAssets(t *testing.T) {
	pEmpty := github.New(testutil.NewMockRunner(), &config.Platform{})
	assert.False(t, pEmpty.HasAssets())

	pWithAssets := github.New(testutil.NewMockRunner(), &config.Platform{Assets: []string{"dist/*"}})
	assert.True(t, pWithAssets.HasAssets())
}

func TestUploadAssets_SingleFile(t *testing.T) {
	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "myapp")
	require.NoError(t, os.WriteFile(assetPath, []byte("binary"), 0o755))

	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		Assets:     []string{assetPath},
	})
	require.NoError(t, p.UploadAssets("v1.2.3"))

	require.Len(t, mr.Calls, 1)
	call := mr.Calls[0]
	assert.Equal(t, "gh", call.Name)
	assert.Equal(t, []string{
		"release", "upload", "v1.2.3",
		assetPath,
		"--repo", "org/repo",
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

	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		Assets:     []string{filepath.Join(tmp, "app_*")},
	})
	require.NoError(t, p.UploadAssets("v1.0.0"))

	// Should have one upload call per matched file
	assert.Len(t, mr.Calls, 2)
	for _, call := range mr.Calls {
		assert.Equal(t, "gh", call.Name)
		assert.Equal(t, "release", call.Args[0])
		assert.Equal(t, "upload", call.Args[1])
		assert.Equal(t, "v1.0.0", call.Args[2])
	}
}

func TestUploadAssets_GlobNoMatch(t *testing.T) {
	mr := testutil.NewMockRunner()
	p := github.New(mr, &config.Platform{
		Repository: "org/repo",
		Assets:     []string{"/tmp/nonexistent/heraut_*"},
	})
	err := p.UploadAssets("v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no files matched")
}

func TestCreateRelease_RepoFromEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	p := github.New(mr, &config.Platform{})
	require.NoError(t, p.CreateRelease("v1.0.0", "notes"))

	call := mr.Calls[0]
	assert.Contains(t, call.Args, "envorg/envrepo")
}

func TestCreateRelease_NoRepo_Error(t *testing.T) {
	// Clear env var
	t.Setenv("GITHUB_REPOSITORY", "")
	mr := testutil.NewMockRunner()

	p := github.New(mr, &config.Platform{})
	err := p.CreateRelease("v1.0.0", "notes")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
}
