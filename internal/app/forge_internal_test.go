package app

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsesNative(t *testing.T) {
	tests := []struct {
		name    string
		drivers []*config.ContentDriver
		want    bool
	}{
		{name: "no drivers", drivers: nil, want: false},
		{name: "nil driver only", drivers: []*config.ContentDriver{nil}, want: false},
		{name: "git-cliff only", drivers: []*config.ContentDriver{{Generator: "git-cliff"}}, want: false},
		{name: "native changelog", drivers: []*config.ContentDriver{{Generator: "native"}, nil}, want: true},
		{name: "native notes among others", drivers: []*config.ContentDriver{{Generator: "git-cliff"}, {Generator: "native"}}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, usesNative(tc.drivers...))
		})
	}
}

func TestGitOriginURL(t *testing.T) {
	t.Run("returns trimmed origin", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("https://gitlab.example.com/group/project.git\n", "", nil)
		assert.Equal(t, "https://gitlab.example.com/group/project.git", gitOriginURL(mr))
	})
	t.Run("no origin configured returns empty", func(t *testing.T) {
		mr := exectest.NewMockRunner() // no response queued → Run errors
		assert.Empty(t, gitOriginURL(mr))
	})
}

// fakeEnv returns a getenv backed by m, so a test's resolution outcome depends only on the
// variables it declares. Reading the real environment here made these tests pass locally and fail
// on GitHub Actions, whose own GITHUB_ACTIONS / GITHUB_REPOSITORY variables pin a github forge.
func fakeEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// clearCIEnv neutralizes every CI marker forge detection keys off. Needed only by tests that
// exercise a path still reading os.Getenv internally.
func clearCIEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"GITHUB_ACTIONS", "GITHUB_SERVER_URL", "GITHUB_API_URL", "GITHUB_REPOSITORY", "GITHUB_TOKEN",
		"GITLAB_CI", "CI_SERVER_URL", "CI_API_V4_URL", "CI_PROJECT_PATH", "CI_JOB_TOKEN", "GITLAB_TOKEN",
		"TF_BUILD", "SYSTEM_COLLECTIONURI", "SYSTEM_TEAMPROJECT", "SYSTEM_ACCESSTOKEN", "AZURE_DEVOPS_TOKEN",
	} {
		t.Setenv(k, "")
	}
}

func TestResolveEnrichForgeIfNeeded(t *testing.T) {
	t.Run("skips resolution when no driver is native", func(t *testing.T) {
		mr := exectest.NewMockRunner() // no response queued: a git call here would error
		cfg := &config.Config{}
		f, id, err := resolveEnrichForgeIfNeeded(mr, fakeEnv(nil), cfg, &config.ContentDriver{Generator: "git-cliff"})
		require.NoError(t, err)
		assert.Nil(t, f)
		assert.Nil(t, id)
		assert.Empty(t, mr.Calls, "no git call when no driver is native")
	})

	t.Run("resolves and constructs a gitlab forge for a native driver", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("https://gitlab.com/group/subgroup/project.git\n", "", nil)
		cfg := &config.Config{}
		f, id, err := resolveEnrichForgeIfNeeded(mr, fakeEnv(nil), cfg, &config.ContentDriver{Generator: "native"})
		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "gitlab", id.Type)
		require.NotNil(t, f, "a concrete gitlab.Forge must be constructed and assigned through the interface")
		assert.Equal(t, "gitlab", f.Type())
	})

	t.Run("non-gitlab resolved forge yields no enrich forge but keeps the identity", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", assertNoOriginErr)
		cfg := &config.Config{Forges: []config.Forge{{Name: "gh", Type: "github", Repository: "acme/widget"}}}
		f, id, err := resolveEnrichForgeIfNeeded(mr, fakeEnv(nil), cfg, &config.ContentDriver{Generator: "native"})
		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "github", id.Type)
		assert.Nil(t, f, "no port.Forge implementation exists yet for github")
	})

	t.Run("ambiguous forge propagates as an error", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", assertNoOriginErr)
		env := fakeEnv(map[string]string{"GITLAB_TOKEN": "glpat", "GITHUB_TOKEN": "ghp"})
		cfg := &config.Config{}
		_, _, err := resolveEnrichForgeIfNeeded(mr, env, cfg, &config.ContentDriver{Generator: "native"})
		require.Error(t, err)
	})

	t.Run("GitLab CI end-to-end: resolution, job-token header selection, and forge construction compose (T160)", func(t *testing.T) {
		// GITLAB_CI env detection wins over the git origin regardless of what it resolves to
		// (forge.Resolve always reads the origin, but resolveAuto's CI branch ignores it); the
		// constructed forge must carry the CI-provided host/project + job token. Kept hermetic:
		// the queued response is a fixed local string, no network.
		mr := exectest.NewMockRunner()
		mr.QueueResponse("https://gitlab.com/other/project.git\n", "", nil)
		env := fakeEnv(map[string]string{
			"GITLAB_CI":       "true",
			"CI_SERVER_URL":   "https://gitlab.example.com",
			"CI_API_V4_URL":   "https://gitlab.example.com/api/v4",
			"CI_PROJECT_PATH": "group/subgroup/project",
			"CI_JOB_TOKEN":    "job-token-xyz",
		})

		cfg := &config.Config{}
		f, id, err := resolveEnrichForgeIfNeeded(mr, env, cfg, &config.ContentDriver{Generator: "native"})
		require.NoError(t, err)

		require.NotNil(t, id)
		assert.Equal(t, "gitlab", id.Type)
		assert.Equal(t, "https://gitlab.example.com", id.Host)
		assert.Equal(t, "https://gitlab.example.com/api/v4", id.APIURL)
		assert.Equal(t, "group/subgroup/project", id.Project)
		assert.Equal(t, "job-token-xyz", id.Token)
		assert.Equal(t, port.TokenJob, id.TokenKind)

		require.NotNil(t, f, "a concrete gitlab.Forge must be constructed for the resolved identity")
		assert.Equal(t, "gitlab", f.Type())
		assert.Equal(t, port.TokenJob, f.Identity().TokenKind, "the constructed forge's identity must carry the job-token kind through to header selection")
	})

	t.Run("policy disabled skips resolution even when the environment is ambiguous", func(t *testing.T) {
		mr := exectest.NewMockRunner() // no response queued: a git call here would error
		env := fakeEnv(map[string]string{"GITLAB_TOKEN": "glpat", "GITHUB_TOKEN": "ghp"})
		cfg := &config.Config{Commits: &config.Commits{EnrichmentPolicy: "disabled"}}
		f, id, err := resolveEnrichForgeIfNeeded(mr, env, cfg, &config.ContentDriver{Generator: "native"})
		require.NoError(t, err)
		assert.Nil(t, f)
		assert.Nil(t, id)
		assert.Empty(t, mr.Calls, "disabled policy must skip resolution entirely, not just swallow the error")
	})
}

// TestBuildReleasePipelineConfig_UsesReadRunnerForForgeResolution proves that
// buildReleasePipelineConfig resolves the forge's git origin via a dedicated read-only runner —
// not the (possibly dry-run) pipeline runner passed for everything else. Before the fix, a
// dry-run runner (which returns ("", "", nil) with no error, per forge/exec) was used directly for
// `git remote get-url origin`, silently producing an empty origin and falling into the
// token-only/ambiguous branches of forge.Resolve.
func TestBuildReleasePipelineConfig_UsesReadRunnerForForgeResolution(t *testing.T) {
	clearCIEnv(t)                              // buildReleasePipelineConfig passes os.Getenv, so the ambient CI env must not leak in
	pipelineRunner := exectest.NewMockRunner() // stands in for a dry-run runner: no response queued
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("https://gitlab.com/group/subgroup/project.git\n", "", nil)

	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{Generator: "native", Output: "CHANGELOG.md"},
	}

	pCfg, err := buildReleasePipelineConfig(pipelineRunner, readRunner, cfg, "", "", false, false)
	require.NoError(t, err)
	require.NotNil(t, pCfg.ForgeIdentity)
	assert.Equal(t, "gitlab", pCfg.ForgeIdentity.Type)
	assert.Equal(t, "https://gitlab.com", pCfg.ForgeIdentity.Host)
	assert.Equal(t, "group/subgroup/project", pCfg.ForgeIdentity.Project)

	assert.Empty(t, pipelineRunner.Calls, "the pipeline (dry-run-capable) runner must not be used for origin detection")
	require.Len(t, readRunner.Calls, 1)
	assert.Equal(t, []string{"remote", "get-url", "origin"}, readRunner.Calls[0].Args)
}

// assertNoOriginErr simulates `git remote get-url origin` failing (no origin configured).
var assertNoOriginErr = assertError("exit status 1")

type assertError string

func (e assertError) Error() string { return string(e) }
