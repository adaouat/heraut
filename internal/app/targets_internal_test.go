package app

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlatformConfigFromTarget(t *testing.T) {
	t.Run("github maps project into Repository", func(t *testing.T) {
		got := platformConfigFromTarget(
			config.Target{Forge: "GH", Draft: true, Assets: []string{"dist/*"}},
			config.Forge{Name: "GH", Type: "github", TokenEnv: "MY_GH_TOKEN"},
			port.ForgeIdentity{Type: "github", Host: "https://github.example.com", Project: "acme/widget"},
		)
		assert.Equal(t, "GH", got.Name)
		assert.Equal(t, "github", got.Type)
		assert.Equal(t, "https://github.example.com", got.BaseURL)
		assert.Equal(t, "acme/widget", got.Repository)
		assert.Empty(t, got.Project, "github carries the path in Repository, not Project")
		assert.Equal(t, "MY_GH_TOKEN", got.TokenEnv)
		assert.True(t, got.Draft)
		assert.Equal(t, []string{"dist/*"}, got.Assets)
	})

	t.Run("gitlab maps project into Project", func(t *testing.T) {
		got := platformConfigFromTarget(
			config.Target{Forge: "GL"},
			config.Forge{Name: "GL", Type: "gitlab"},
			port.ForgeIdentity{Type: "gitlab", Host: "https://gitlab.example.com", Project: "group/subgroup/project"},
		)
		assert.Equal(t, "gitlab", got.Type)
		assert.Equal(t, "https://gitlab.example.com", got.BaseURL)
		assert.Equal(t, "group/subgroup/project", got.Project)
		assert.Empty(t, got.Repository)
		assert.Empty(t, got.TokenEnv, "no token_env leaves the driver's per-type default in force")
	})
}

// TestBuildReleasePipelineConfig_TargetsWiring proves buildReleasePipelineConfig builds
// pCfg.Platforms from config.EffectiveTargets + the resolved forge identities, sharing a single
// forge.Resolve call with enrichment (no duplicate git call — see
// TestBuildReleasePipelineConfig_UsesReadRunnerForForgeResolution for the read-runner half of
// that contract).
func TestBuildReleasePipelineConfig_TargetsWiring(t *testing.T) {
	t.Run("target with no forge: resolves against the sole configured forge", func(t *testing.T) {
		clearCIEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Forges: []config.Forge{
				{Name: "gh", Type: "github", Repository: "acme/widget", TokenEnv: "MY_GH_TOKEN"},
			},
			Release: &config.Release{
				Targets: []config.Target{{Draft: true}},
			},
		}

		pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.NoError(t, err)
		require.Len(t, pCfg.Platforms, 1)
		require.Len(t, readRunner.Calls, 1, "exactly one git call — shared with enrichment resolution")
	})

	t.Run("zero-config: no release.targets at all, one auto-detected forge yields one driver", func(t *testing.T) {
		clearCIEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("https://gitlab.com/group/subgroup/project.git\n", "", nil)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
		}

		pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.NoError(t, err)
		require.Len(t, pCfg.Platforms, 1, "zero-config: one resolved forge must yield one driver")
	})

	t.Run("release.platforms alone does not gain a silent zero-config duplicate", func(t *testing.T) {
		// A config still on release.platforms (no forges:, no release.targets) must keep building
		// exactly the platforms it declares — even when the ambient CI environment would otherwise
		// auto-resolve a forge for zero-config publish. Without hasEffectivePlatforms gating the
		// synthesized default target, this config would silently publish twice.
		clearCIEnv(t)
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("GITHUB_SERVER_URL", "https://github.com")
		t.Setenv("GITHUB_REPOSITORY", "acme/widget")

		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Release: &config.Release{
				Platforms: []config.Platform{{Type: "github", Name: "gh"}},
			},
		}

		pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.NoError(t, err)
		require.Len(t, pCfg.Platforms, 1, "release.platforms's own entry, and nothing synthesized from zero-config")
	})

	t.Run("no forge resolves at all: zero platforms, no error", func(t *testing.T) {
		clearCIEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
		}

		pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.NoError(t, err)
		assert.Empty(t, pCfg.Platforms)
	})

	t.Run("release.assets propagates to targets that declare none", func(t *testing.T) {
		clearCIEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Forges: []config.Forge{
				{Name: "gh", Type: "github", Repository: "acme/widget"},
			},
			Release: &config.Release{
				Targets: []config.Target{{Forge: "gh"}},
				Assets:  []string{"dist/*"},
			},
		}

		pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.NoError(t, err)
		require.Len(t, pCfg.Platforms, 1)
	})

	t.Run("multi-instance publishing: multiple targets yield multiple drivers (ADR-0025)", func(t *testing.T) {
		clearCIEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Forges: []config.Forge{
				{Name: "gh", Type: "github", Repository: "acme/widget"},
				{Name: "gl", Type: "gitlab", Project: "group/subgroup/project"},
			},
			Release: &config.Release{
				Targets: []config.Target{{Forge: "gh"}, {Forge: "gl"}},
			},
		}

		pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.NoError(t, err)
		require.Len(t, pCfg.Platforms, 2)
	})

	t.Run("unknown forge name in a target errors clearly", func(t *testing.T) {
		clearCIEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Forges: []config.Forge{
				{Name: "gh", Type: "github", Repository: "acme/widget"},
			},
			Release: &config.Release{
				Targets: []config.Target{{Forge: "does-not-exist"}},
			},
		}

		_, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does-not-exist")
	})
}

// TestBuildReleasePipelineConfig_ForgeResolutionErrorScope pins when a forge-resolution failure is
// allowed to abort a release. Resolution is local (config / CI env / git origin) and can fail on a
// plain developer machine — e.g. both GITHUB_TOKEN and GITLAB_TOKEN exported, the standard default
// token envs for gh/glab, outside CI with no recognised origin, which forge.Resolve reports as
// ErrAmbiguousForge. That must only be fatal when something actually consumes the resolution:
// enrichment (native + policy not disabled), a non-empty release.targets, or the zero-config
// publish synthesis path. A release.platforms-only config consumes none of it and must publish
// exactly as it did before publishing started resolving forges at all.
func TestBuildReleasePipelineConfig_ForgeResolutionErrorScope(t *testing.T) {
	// ambiguousEnv makes forge.Resolve's zero-config auto-detection fail with ErrAmbiguousForge:
	// two candidate token envs and nothing (CI marker, git origin) to disambiguate them.
	ambiguousEnv := func(t *testing.T) {
		t.Helper()
		clearCIEnv(t)
		t.Setenv("GITHUB_TOKEN", "ghp-placeholder")
		t.Setenv("GITLAB_TOKEN", "glpat-placeholder")
	}

	t.Run("not fatal when nothing consumes the resolution (git-cliff + release.platforms only)", func(t *testing.T) {
		ambiguousEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Changelog:  &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"},
			Release: &config.Release{
				Platforms: []config.Platform{{Type: "github", Name: "gh", Repository: "acme/widget"}},
			},
		}

		pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.NoError(t, err, "an unusable forge must not abort a release that never consults one")
		require.Len(t, pCfg.Platforms, 1, "release.platforms must still build its declared platform")
		assert.Nil(t, pCfg.ForgeIdentity, "no forge-derived identity when resolution was not consumed")
	})

	t.Run("fatal when release.targets needs the resolution", func(t *testing.T) {
		ambiguousEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Release: &config.Release{
				Targets: []config.Target{{}},
			},
		}

		_, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.Error(t, err, "a target that cannot be resolved to a forge must abort the release")
		assert.ErrorIs(t, err, forge.ErrAmbiguousForge)
	})

	t.Run("fatal when the zero-config publish synthesis path needs the resolution", func(t *testing.T) {
		ambiguousEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
		}

		_, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.Error(t, err)
		assert.ErrorIs(t, err, forge.ErrAmbiguousForge)
	})

	t.Run("fatal when enrichment needs the resolution (native generator)", func(t *testing.T) {
		ambiguousEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Changelog:  &config.ContentDriver{Generator: "native", Output: "CHANGELOG.md"},
			Release: &config.Release{
				Platforms: []config.Platform{{Type: "github", Name: "gh", Repository: "acme/widget"}},
			},
		}

		_, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.Error(t, err)
		assert.ErrorIs(t, err, forge.ErrAmbiguousForge)
	})

	t.Run("not fatal when the native generator has enrichment disabled", func(t *testing.T) {
		// --offline (and enrichment_policy: disabled) must never let an unusable forge cause a
		// failure — the pre-existing guarantee in resolveEnrichForgeIfNeeded's contract.
		ambiguousEnv(t)
		runner := exectest.NewMockRunner()
		readRunner := exectest.NewMockRunner()
		readRunner.QueueResponse("", "", assertNoOriginErr)

		cfg := &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver"},
			Changelog:  &config.ContentDriver{Generator: "native", Output: "CHANGELOG.md"},
			Commits:    &config.Commits{EnrichmentPolicy: "disabled"},
			Release: &config.Release{
				Platforms: []config.Platform{{Type: "github", Name: "gh", Repository: "acme/widget"}},
			},
		}

		pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
		require.NoError(t, err)
		require.Len(t, pCfg.Platforms, 1)
		assert.Nil(t, pCfg.ForgeIdentity)
	})
}
