package app

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestResolveEnrichForgeIfNeeded(t *testing.T) {
	t.Run("resolves and constructs a gitlab forge for a native driver", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("https://gitlab.com/group/subgroup/project.git\n", "", nil)
		cfg := &config.Config{}
		f, id, degradedReason, err := resolveEnrichForgeIfNeeded(mr, fakeEnv(nil), cfg, false, &config.ContentDriver{})
		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "gitlab", id.Type)
		require.NotNil(t, f, "a concrete gitlab.Forge must be constructed and assigned through the interface")
		assert.Equal(t, "gitlab", f.Type())
		assert.Empty(t, degradedReason)
	})

	t.Run("resolves and constructs a github forge for a native driver", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", assertNoOriginErr)
		cfg := &config.Config{Forges: []config.Forge{{Name: "gh", Type: "github", Repository: "acme/widget"}}}
		f, id, degradedReason, err := resolveEnrichForgeIfNeeded(mr, fakeEnv(nil), cfg, false, &config.ContentDriver{})
		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "github", id.Type)
		require.NotNil(t, f, "a concrete github.Forge must be constructed and assigned through the interface")
		assert.Equal(t, "github", f.Type())
		assert.Empty(t, degradedReason)
	})

	t.Run("resolves and constructs an azure forge for a native driver", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", assertNoOriginErr)
		cfg := &config.Config{Forges: []config.Forge{{
			Name: "az", Type: "azure_devops", Project: "myorg/myproject", Repository: "myrepo",
		}}}
		f, id, degradedReason, err := resolveEnrichForgeIfNeeded(mr, fakeEnv(nil), cfg, false, &config.ContentDriver{})
		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "azure_devops", id.Type)
		require.NotNil(t, f, "a concrete azure.Forge must be constructed and assigned through the interface")
		assert.Equal(t, "azure_devops", f.Type())
		assert.Empty(t, degradedReason)
	})

	// T175: optional (the default) promises "on failure, degrade" — a resolution failure is just
	// another enrichment failure, so it must not be fatal. This is what makes heraut check's
	// warn-only severity (T172) for this exact config correct by construction, instead of check
	// predicting success while heraut changelog hard-fails.
	t.Run("ambiguous forge under the default policy degrades instead of erroring (T175)", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", assertNoOriginErr)
		env := fakeEnv(map[string]string{"GITLAB_TOKEN": "glpat", "GITHUB_TOKEN": "ghp"})
		cfg := &config.Config{}
		f, id, degradedReason, err := resolveEnrichForgeIfNeeded(mr, env, cfg, false, &config.ContentDriver{})
		require.NoError(t, err)
		assert.Nil(t, f)
		assert.Nil(t, id)
		assert.NotEmpty(t, degradedReason, "the caller must be able to mark the generator degraded")
	})

	t.Run("ambiguous forge under enrichment_policy: required still propagates as an error", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", assertNoOriginErr)
		env := fakeEnv(map[string]string{"GITLAB_TOKEN": "glpat", "GITHUB_TOKEN": "ghp"})
		cfg := &config.Config{Commits: &config.Commits{EnrichmentPolicy: "required"}}
		f, id, degradedReason, err := resolveEnrichForgeIfNeeded(mr, env, cfg, false, &config.ContentDriver{})
		require.Error(t, err, "required is a guarantee — a resolution failure must stay fatal")
		assert.Nil(t, f)
		assert.Nil(t, id)
		assert.Empty(t, degradedReason)
	})

	t.Run("--force downgrades a required-policy resolution failure to degrade, same as a fetch failure", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", assertNoOriginErr)
		env := fakeEnv(map[string]string{"GITLAB_TOKEN": "glpat", "GITHUB_TOKEN": "ghp"})
		cfg := &config.Config{Commits: &config.Commits{EnrichmentPolicy: "required"}}
		f, id, degradedReason, err := resolveEnrichForgeIfNeeded(mr, env, cfg, true, &config.ContentDriver{})
		require.NoError(t, err)
		assert.Nil(t, f)
		assert.Nil(t, id)
		assert.NotEmpty(t, degradedReason)
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
		f, id, degradedReason, err := resolveEnrichForgeIfNeeded(mr, env, cfg, false, &config.ContentDriver{})
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
		assert.Empty(t, degradedReason)
	})

	t.Run("policy disabled skips resolution even when the environment is ambiguous", func(t *testing.T) {
		mr := exectest.NewMockRunner() // no response queued: a git call here would error
		env := fakeEnv(map[string]string{"GITLAB_TOKEN": "glpat", "GITHUB_TOKEN": "ghp"})
		cfg := &config.Config{Commits: &config.Commits{EnrichmentPolicy: "disabled"}}
		f, id, degradedReason, err := resolveEnrichForgeIfNeeded(mr, env, cfg, false, &config.ContentDriver{})
		require.NoError(t, err)
		assert.Nil(t, f)
		assert.Nil(t, id)
		assert.Empty(t, degradedReason)
		assert.Empty(t, mr.Calls, "disabled policy must skip resolution entirely, not just swallow the error")
	})
}

// TestBuildChangelogPipelineConfig_AmbiguousForgeDegradesUnderOptionalPolicy is the end-to-end
// proof for T175: a changelog-only pipeline (generator: native, default/optional policy, no
// forges:, no release.targets) on an ambiguous machine no longer hard-fails the way heraut
// changelog used to — it builds successfully, and the resulting generator reports Degraded(),
// the same outcome heraut check already predicts for this config (T172).
func TestBuildChangelogPipelineConfig_AmbiguousForgeDegradesUnderOptionalPolicy(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("GITLAB_TOKEN", "glpat")
	t.Setenv("GITHUB_TOKEN", "ghp")

	runner := exectest.NewMockRunner()
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("", "", assertNoOriginErr)

	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{},
	}

	cCfg, err := buildChangelogPipelineConfig(runner, readRunner, cfg, PipelineOpts{})
	require.NoError(t, err, "an ambiguous forge under the default policy must degrade, not hard-fail")
	require.NotNil(t, cCfg.Changelog)

	degraded, ok := cCfg.Changelog.(interface{ Degraded() bool })
	require.True(t, ok, "the native generator exposes Degraded()")
	assert.True(t, degraded.Degraded())
}

// TestHasResolvablePublishTarget_NilConfig guards a nil-pointer panic (T173): its sibling
// effectiveTargetPlatforms (internal/app/check.go) nil-guards cfg before calling resolveForge,
// but HasResolvablePublishTarget did not — forge.Resolve dereferences cfg.Forges directly, so a
// nil cfg panicked instead of returning false.
func TestHasResolvablePublishTarget_NilConfig(t *testing.T) {
	mr := exectest.NewMockRunner()
	var got bool
	require.NotPanics(t, func() {
		got = HasResolvablePublishTarget(mr, nil, "")
	})
	assert.False(t, got)
}

// TestBuildReleasePipelineConfig_UsesReadRunnerForForgeResolution proves that
// buildReleasePipelineConfig resolves the forge's git origin via a dedicated read-only runner —
// not the (possibly dry-run) pipeline runner passed for everything else. Before the fix, a
// dry-run runner (which returns ("", "", nil) with no error, per forge/exec) was used directly for
// `git remote get-url origin`, silently producing an empty origin and falling into the
// token-only/ambiguous branches of forge.Resolve.
func TestBuildReleasePipelineConfig_UsesReadRunnerForForgeResolution(t *testing.T) {
	testutil.ClearCIEnv(t)                     // buildReleasePipelineConfig passes os.Getenv, so the ambient CI env must not leak in
	pipelineRunner := exectest.NewMockRunner() // stands in for a dry-run runner: no response queued
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("https://gitlab.com/group/subgroup/project.git\n", "", nil)

	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{Output: "CHANGELOG.md"},
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
