package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

// findErr returns the first ValidationError whose Path contains wantPath, or nil.
func findErr(errs config.ValidationErrors, wantPath string) *config.ValidationError {
	for i := range errs {
		if errs[i].Path == wantPath {
			return &errs[i]
		}
	}
	return nil
}

// mustLoad is a test helper that parses inline YAML and panics on loader error.
func mustLoad(t *testing.T, src string) *config.Config {
	t.Helper()
	cfg, err := config.LoadFromReader(strings.NewReader(src))
	require.NoError(t, err, "mustLoad: unexpected parse error")
	return cfg
}

// ── valid configs ────────────────────────────────────────────────────────────

func TestValidate_validSemver(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  bump: auto
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_validCalver(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_validSemverPerEnv(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  dev:
    bump: auto
  prod:
    bump: promote
    source: dev
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_validCalverPerEnv(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
`)
	assert.Empty(t, config.Validate(cfg))
}

// ── version ──────────────────────────────────────────────────────────────────

func TestValidate_missingVersion(t *testing.T) {
	cfg := mustLoad(t, `
versioning:
  strategy: semver
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "version")
	require.NotNil(t, e, "expected error on 'version'")
	assert.Contains(t, e.Message, "required")
	assert.NotEmpty(t, e.Hint)
}

func TestValidate_invalidVersion(t *testing.T) {
	cfg := mustLoad(t, `
version: "2"
versioning:
  strategy: semver
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "version")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, `"2"`)
}

// ── strategy ─────────────────────────────────────────────────────────────────

func TestValidate_missingStrategy(t *testing.T) {
	cfg := mustLoad(t, `version: "1"`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.strategy")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

func TestValidate_invalidStrategy(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: foobar
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.strategy")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "foobar")
}

// ── tag_type ──────────────────────────────────────────────────────────────────

func TestValidate_invalidTagType(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  tag_type: signed
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.tag_type")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "signed")
	assert.Contains(t, e.Hint, "annotated")
	assert.Contains(t, e.Hint, "lightweight")
}

func TestValidate_validTagTypes(t *testing.T) {
	for _, tagType := range []string{"annotated", "lightweight"} {
		t.Run(tagType, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  tag_type: `+tagType+`
`)
			assert.Empty(t, config.Validate(cfg))
		})
	}
}

// ── remote_metadata ─────────────────────────────────────────────────────────────

func TestValidate_invalidRemoteMetadata(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  remote_metadata: sometimes
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "commits.remote_metadata")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "sometimes")
	assert.Contains(t, e.Hint, "required")
	assert.Contains(t, e.Hint, "optional")
	assert.Contains(t, e.Hint, "disabled")
}

func TestValidate_validRemoteMetadata(t *testing.T) {
	for _, v := range []string{"required", "optional", "disabled"} {
		t.Run(v, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  remote_metadata: `+v+`
`)
			assert.Empty(t, config.Validate(cfg))
		})
	}
}

func TestValidate_emptyRemoteMetadataIsValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
`)
	assert.Nil(t, findErr(config.Validate(cfg), "remote_metadata"))
}

// ── calver format ─────────────────────────────────────────────────────────────

func TestValidate_calverMissingFormat(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.format")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

func TestValidate_calverPerEnvMissingFormat(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.format")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

// ── generators ───────────────────────────────────────────────────────────────

func TestValidate_changelogMissingGenerator(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  output: CHANGELOG.md
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "changelog.generator")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

func TestValidate_changelogInvalidGenerator(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: bad-gen
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "changelog.generator")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "bad-gen")
}

func TestValidate_releaseNotesInvalidGenerator(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  notes:
    generator: not-valid
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "release.notes.generator")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "not-valid")
}

// ── platforms ────────────────────────────────────────────────────────────────

func TestValidate_platformMissing(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - token_env: GH_TOKEN
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "release.platforms[0].platform")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

func TestValidate_platformInvalid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: aws
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "release.platforms[0].platform")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "aws")
}

func TestValidate_multiplePlatforms(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      name: github
    - platform: gitlab
      name: gitlab
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_platformNameRequired(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      repository: acme/widget
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "release.platforms[0].name")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

func TestValidate_platformNameDuplicate(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: gitlab
      name: primary
      project: acme/widget
    - platform: gitlab
      name: primary
      project: tools/widget-mirror
      base_url: https://gitlab.example.com
      token_env: GITLAB_INTERNAL_TOKEN
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "release.platforms[1].name")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "duplicate")
}

// ── platform base_url (ADR-0020) ─────────────────────────────────────────────

func TestValidate_platformBaseURLDefaultApplied(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      name: github
      repository: acme/widget
`)
	require.NotNil(t, cfg.Release)
	require.Len(t, cfg.Release.Platforms, 1)
	assert.Equal(t, "https://github.com", cfg.Release.Platforms[0].BaseURL)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_platformBaseURLDefaultAppliedGitLab(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: gitlab
      name: gitlab
      project: acme/widget
`)
	require.Len(t, cfg.Release.Platforms, 1)
	assert.Equal(t, "https://gitlab.com", cfg.Release.Platforms[0].BaseURL)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_platformBaseURLExplicitDefault(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: gitlab
      name: gitlab
      project: acme/widget
      base_url: https://gitlab.com
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_platformBaseURLTrailingSlashNormalized(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      name: github
      repository: acme/widget
      base_url: https://github.com/
`)
	assert.Equal(t, "https://github.com", cfg.Release.Platforms[0].BaseURL)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_platformBaseURLSelfHostedAccepted(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: gitlab
      name: gitlab-internal
      project: acme/widget
      base_url: https://gitlab.example.com
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_platformBaseURLMalformed(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  platforms:
    - platform: github
      name: github
      repository: acme/widget
      base_url: "not a url"
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "release.platforms[0].base_url")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "not a valid URL")
	assert.NotContains(t, e.Message, "not yet supported")
}

func TestValidate_envOverridePlatformBaseURLSelfHostedAccepted(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
    release:
      platforms:
        - platform: gitlab
          name: gitlab-internal
          project: acme/widget
          base_url: https://gitlab.example.com
`)
	assert.Empty(t, config.Validate(cfg))
}

// ── env overrides (top-level environments) ───────────────────────────────────

func TestValidate_envOverrideInvalidPlatform(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
    release:
      platforms:
        - platform: aws
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.release.platforms[0].platform")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "aws")
}

// ── per-env strategy ─────────────────────────────────────────────────────────

func TestValidate_perEnvNoEnvironments(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

func TestValidate_perEnvMissingBump(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.bump")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

func TestValidate_perEnvInvalidBump(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: manual
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.bump")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "manual")
}

// ── tag_format ────────────────────────────────────────────────────────────────

func TestValidate_commonTagFormatMissingVersionToken(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}-release"
environments:
  dev:
    bump: auto
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.tag_format")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "{version}")
}

func TestValidate_envTagFormatMissingVersionToken(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev-only"
    bump: auto
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.tag_format")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "{version}")
}

func TestValidate_noTagFormatAnywhere(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    bump: auto
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.tag_format")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

// ── source validation (ADR-0008) ─────────────────────────────────────────────

func TestValidate_sourceOnAutoEnv(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
    source: prod
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: dev
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.source")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "auto")
}

func TestValidate_sourceNonExistentEnv(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: nonexistent
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.prod.source")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "nonexistent")
}

func TestValidate_sourceSelfReference(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: prod
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.prod.source")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "itself")
}

func TestValidate_promoteNoAutoEnvs(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  staging:
    tag_format: "staging/{version}"
    bump: promote
  prod:
    tag_format: "prod/{version}"
    bump: promote
`)
	errs := config.Validate(cfg)
	// Both staging and prod should have source errors
	e := findErr(errs, "environments.prod.source")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "no auto environment")
}

func TestValidate_promoteAmbiguousSource(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  hotfix:
    tag_format: "hotfix/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.prod.source")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "ambiguous")
}

// ── cycle detection ───────────────────────────────────────────────────────────

func TestValidate_sourceCycle(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  staging:
    tag_format: "staging/{version}"
    bump: promote
    source: prod
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: staging
`)
	errs := config.Validate(cfg)
	// Should detect a cycle involving prod and staging.
	var cycleErr *config.ValidationError
	for i := range errs {
		if strings.Contains(errs[i].Message, "cycle") {
			cycleErr = &errs[i]
			break
		}
	}
	require.NotNil(t, cycleErr, "expected a cycle detection error")
	assert.Contains(t, cycleErr.Message, "→")
	assert.NotEmpty(t, cycleErr.Hint)
}

// ── collect all errors ────────────────────────────────────────────────────────

func TestValidate_collectsAllErrors(t *testing.T) {
	cfg := mustLoad(t, `
version: "2"
versioning:
  strategy: not-valid
changelog:
  generator: bad-gen
release:
  platforms:
    - platform: aws
`)
	errs := config.Validate(cfg)
	assert.GreaterOrEqual(t, len(errs), 3, "expected at least 3 errors")
	assert.NotNil(t, findErr(errs, "version"))
	assert.NotNil(t, findErr(errs, "versioning.strategy"))
	assert.NotNil(t, findErr(errs, "changelog.generator"))
	assert.NotNil(t, findErr(errs, "release.platforms[0].platform"))
}

// ── fixture-based tests ───────────────────────────────────────────────────────

func TestValidate_validFixtures(t *testing.T) {
	fixtures := []string{
		"../../testdata/config/valid/semver.yml",
		"../../testdata/config/valid/calver.yml",
		"../../testdata/config/valid/semver-per-env.yml",
		"../../testdata/config/valid/calver-per-env.yml",
		"../../testdata/config/valid/platform-base-url.yml",
	}
	for _, path := range fixtures {
		t.Run(path, func(t *testing.T) {
			cfg, err := config.Load(path)
			require.NoError(t, err)
			assert.Empty(t, config.Validate(cfg))
		})
	}
}

func TestValidate_invalidFixtures(t *testing.T) {
	tests := []struct {
		fixture     string
		wantPath    string
		wantMessage string
	}{
		{
			fixture:     "../../testdata/config/invalid/missing_version.yml",
			wantPath:    "version",
			wantMessage: "required",
		},
		{
			fixture:     "../../testdata/config/invalid/invalid_strategy.yml",
			wantPath:    "versioning.strategy",
			wantMessage: "not-a-strategy",
		},
		{
			fixture:     "../../testdata/config/invalid/invalid_generator.yml",
			wantPath:    "changelog.generator",
			wantMessage: "unknown-generator",
		},
		{
			fixture:     "../../testdata/config/invalid/perenv_no_environments.yml",
			wantPath:    "environments",
			wantMessage: "required",
		},
		{
			fixture:     "../../testdata/config/invalid/flat_strategy_with_environments.yml",
			wantPath:    "environments",
			wantMessage: "semver",
		},
		{
			fixture:     "../../testdata/config/invalid/source_ambiguous.yml",
			wantPath:    "environments.prod.source",
			wantMessage: "ambiguous",
		},
	}
	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			cfg, err := config.Load(tc.fixture)
			require.NoError(t, err, "fixture should load without parse error")
			errs := config.Validate(cfg)
			require.NotEmpty(t, errs, "expected at least one validation error")
			e := findErr(errs, tc.wantPath)
			require.NotNil(t, e, "expected error on path %q", tc.wantPath)
			assert.Contains(t, e.Message, tc.wantMessage)
		})
	}
}

func TestValidate_sourceCycleFixture(t *testing.T) {
	cfg, err := config.Load("../../testdata/config/invalid/source_cycle.yml")
	require.NoError(t, err)
	errs := config.Validate(cfg)
	var found bool
	for _, e := range errs {
		if strings.Contains(e.Message, "cycle") {
			found = true
			assert.Contains(t, e.Message, "→")
			break
		}
	}
	assert.True(t, found, "expected a cycle detection error")
}

// ── flat strategy with environments ──────────────────────────────────────────

func TestValidate_flatStrategyWithEnvironments(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "semver")
}

// ── contradiction warnings ────────────────────────────────────────────────────

func TestValidate_disableChangelogAndChangelogOverride(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
    disable_changelog: true
    changelog:
      generator: git-cliff
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.changelog")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unreachable")
}

func TestValidate_disableNotesAndNotesOverride(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
    disable_notes: true
    release:
      notes:
        generator: git-cliff
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.release.notes")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unreachable")
}

// ADR-0019: per-env content drivers merge over the top-level. A partial per-env
// changelog block (no generator) inherits the top-level generator and is valid.
func TestValidate_perEnvChangelogInheritsGenerator(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
changelog:
  generator: git-cliff
  output: CHANGELOG.md
environments:
  prod:
    bump: auto
    changelog:
      config: cliff.prod.toml
`)
	errs := config.Validate(cfg)
	assert.Nil(t, findErr(errs, "environments.prod.changelog.generator"),
		"per-env block should inherit the top-level generator")
}

// A per-env changelog with no generator and no top-level changelog to inherit from
// is still invalid — the merged driver has no generator.
func TestValidate_perEnvChangelogNoGeneratorAnywhere(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  prod:
    bump: auto
    changelog:
      config: cliff.prod.toml
`)
	errs := config.Validate(cfg)
	assert.NotNil(t, findErr(errs, "environments.prod.changelog.generator"),
		"no generator at either level must still fail")
}

// ── tickets ──────────────────────────────────────────────────────────────────

func TestValidate_TicketsValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
commits:
  tickets:
    - pattern: '[A-Z]+-[0-9]+'
      url: 'https://acme.atlassian.net/browse/{ticket}'
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_TicketsInvalidRegex(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  tickets:
    - pattern: '[A-Z'
      url: 'https://x.test/{ticket}'
`)
	e := findErr(config.Validate(cfg), "commits.tickets[0].pattern")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "regex")
}

func TestValidate_TicketsURLMissingPlaceholder(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  tickets:
    - pattern: '[A-Z]+-[0-9]+'
      url: 'https://x.test/browse/'
`)
	e := findErr(config.Validate(cfg), "commits.tickets[0].url")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "{ticket}")
}

func TestValidate_TicketsNonGitCliffGenerator(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: communique
commits:
  tickets:
    - pattern: '[A-Z]+-[0-9]+'
      url: 'https://x.test/{ticket}'
`)
	e := findErr(config.Validate(cfg), "commits.tickets")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "git-cliff")
}

// ── native generator ──────────────────────────────────────────────────────────

func TestValidate_NativeGenerator(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: native
  output: CHANGELOG.md
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_TicketsNativeGeneratorOK(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: native
commits:
  tickets:
    - pattern: '[A-Z]+-[0-9]+'
      url: 'https://x.test/{ticket}'
`)
	assert.Empty(t, config.Validate(cfg))
}

// ── rendering.templates / template (native only) ──────────────────────────────

func TestValidate_RenderingTemplatesRequiresNative(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: communique
  config: communique.toml
rendering:
  templates:
    commit: "- {{ .Description }}"
`)
	e := findErr(config.Validate(cfg), "rendering.templates")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "native")
}

func TestValidate_RenderingTemplatesNativeValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: native
rendering:
  templates:
    commit: "- {{ upperFirst .Description }} ({{ .ShortHash }})"
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_RenderingTemplatesBadSnippet(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: native
rendering:
  templates:
    commit: "{{ .Description "
`)
	e := findErr(config.Validate(cfg), "rendering.templates.commit")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "template")
}

func TestValidate_DriverTemplateRequiresNative(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: communique
  config: communique.toml
  template: .config/heraut/changelog.tmpl
`)
	e := findErr(config.Validate(cfg), "changelog.template")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "native")
}

func TestValidate_DriverTemplateFileMissing(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: native
  template: .config/heraut/does-not-exist.tmpl
`)
	e := findErr(config.Validate(cfg), "changelog.template")
	require.NotNil(t, e)
}

// TestValidate_NativePerEnvAccepted verifies native is now supported under a per-env strategy
// (T138): the app layer scopes native's tag walk to the env via the derived TagGlob, so the
// former blanket rejection is gone.
func TestValidate_NativePerEnvAccepted(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  dev:
    branch: develop
    bump: auto
  prod:
    branch: main
    bump: promote
    source: dev
changelog:
  generator: native
  output: CHANGELOG.md
`)
	assert.Nil(t, findErr(config.Validate(cfg), "changelog.generator"),
		"native is now supported under a per-env strategy")
}

// ── tag_pattern ──────────────────────────────────────────────────────────────

func TestValidate_changelogTagPatternRequiresGitCliff(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: communique
  tag_pattern: "v[0-9]*"
`)
	e := findErr(config.Validate(cfg), "changelog.tag_pattern")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "git-cliff")
}

// TestValidate_NativeTagPatternAccepted verifies an explicit tag_pattern is now valid with native
// (T139), treated as a Go regex.
func TestValidate_NativeTagPatternAccepted(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: native
  tag_pattern: "^v.*-prod$"
`)
	assert.Nil(t, findErr(config.Validate(cfg), "changelog.tag_pattern"),
		"native accepts an explicit tag_pattern (Go regex)")
}

// TestValidate_NativeTagPatternInvalidRegex rejects a tag_pattern that does not compile as a Go regex.
func TestValidate_NativeTagPatternInvalidRegex(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: native
  tag_pattern: "["
`)
	e := findErr(config.Validate(cfg), "changelog.tag_pattern")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "invalid regex")
}

func TestValidate_GeneratorCocogittoRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: cocogitto
`)
	e := findErr(config.Validate(cfg), "changelog.generator")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "cocogitto")
	assert.Contains(t, e.Hint, "git-cliff")
	assert.Contains(t, e.Hint, "communique")
	assert.NotContains(t, e.Hint, "cocogitto")
}

func TestValidate_changelogTagPatternGitCliffValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  tag_pattern: "v[0-9]*"
`)
	assert.Nil(t, findErr(config.Validate(cfg), "changelog.tag_pattern"))
}

func TestValidate_releaseNotesTagPatternRequiresGitCliff(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  notes:
    generator: communique
    tag_pattern: "v[0-9]*"
`)
	e := findErr(config.Validate(cfg), "release.notes.tag_pattern")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "git-cliff")
}

// A per-env changelog that only sets tag_pattern inherits the top-level git-cliff
// generator via MergeContentDriver, so the effective driver is valid.
func TestValidate_perEnvTagPatternInheritsGitCliff(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
changelog:
  generator: git-cliff
environments:
  prod:
    bump: auto
    changelog:
      tag_pattern: "prod/*"
`)
	assert.Nil(t, findErr(config.Validate(cfg), "environments.prod.changelog.tag_pattern"))
}

// A per-env changelog that switches to a non-git-cliff generator and sets tag_pattern
// fully replaces the inherited driver (ADR-0019), so the effective generator is the
// override's — and tag_pattern is rejected.
func TestValidate_perEnvTagPatternGeneratorSwitchRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
changelog:
  generator: git-cliff
environments:
  prod:
    bump: auto
    changelog:
      generator: communique
      tag_pattern: "prod/*"
`)
	e := findErr(config.Validate(cfg), "environments.prod.changelog.tag_pattern")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "git-cliff")
}

// ── changelog.remote (ADR-0026) ─────────────────────────────────────────────

func TestValidate_changelogRemoteAzureDevOpsValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  remote:
    type: azure_devops
    project: my-org/my-project
    repository: my-repo
`)
	errs := config.Validate(cfg)
	for _, e := range errs {
		assert.NotContains(t, e.Path, "remote", "unexpected error: %+v", e)
	}
}

func TestValidate_changelogRemoteMissingType(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  remote:
    repository: acme/widgets
`)
	e := findErr(config.Validate(cfg), "changelog.remote.type")
	require.NotNil(t, e)
	assert.Equal(t, "required", e.Message)
}

func TestValidate_changelogRemoteInvalidType(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  remote:
    type: bitbucket
    repository: acme/widgets
`)
	e := findErr(config.Validate(cfg), "changelog.remote.type")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "bitbucket")
}

func TestValidate_changelogRemoteAzureDevOpsMissingFields(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  remote:
    type: azure_devops
`)
	errs := config.Validate(cfg)
	require.NotNil(t, findErr(errs, "changelog.remote.project"))
	require.NotNil(t, findErr(errs, "changelog.remote.repository"))
}

func TestValidate_changelogRemoteGithubMissingRepository(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  remote:
    type: github
`)
	e := findErr(config.Validate(cfg), "changelog.remote.repository")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "github")
}

func TestValidate_changelogRemoteGitlabMissingProject(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  remote:
    type: gitlab
`)
	e := findErr(config.Validate(cfg), "changelog.remote.project")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "gitlab")
}

func TestValidate_changelogRemoteRequiresGitCliff(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: communique
  remote:
    type: azure_devops
    project: my-org/my-project
    repository: my-repo
`)
	e := findErr(config.Validate(cfg), "changelog.remote")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "git-cliff")
}

func TestValidate_changelogRemoteInvalidAPIURL(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
  remote:
    type: azure_devops
    project: my-org/my-project
    repository: my-repo
    api_url: "not-a-url"
`)
	e := findErr(config.Validate(cfg), "changelog.remote.api_url")
	require.NotNil(t, e)
}

func TestValidate_releaseNotesRemoteRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
release:
  notes:
    generator: git-cliff
    remote:
      type: azure_devops
      project: my-org/my-project
      repository: my-repo
`)
	e := findErr(config.Validate(cfg), "release.notes.remote")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "changelog")
}

// A per-env changelog that only sets remote inherits the top-level git-cliff generator
// via MergeContentDriver, so the effective driver is valid.
func TestValidate_perEnvChangelogRemoteInheritsGitCliff(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
changelog:
  generator: git-cliff
environments:
  prod:
    bump: auto
    changelog:
      remote:
        type: azure_devops
        project: my-org/my-project
        repository: my-repo
`)
	errs := config.Validate(cfg)
	for _, e := range errs {
		assert.NotContains(t, e.Path, "remote", "unexpected error: %+v", e)
	}
}

func TestValidate_perEnvReleaseNotesRemoteRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
release:
  notes:
    generator: git-cliff
environments:
  prod:
    bump: auto
    release:
      notes:
        remote:
          type: azure_devops
          project: my-org/my-project
          repository: my-repo
`)
	e := findErr(config.Validate(cfg), "environments.prod.release.notes.remote")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "changelog")
}

// ── commits ──────────────────────────────────────────────────────────────────

func TestValidate_CommitsValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  types:
    - name: feat
    - name: fix
    - name: docs
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_CommitsEmptyTypes_OK(t *testing.T) {
	// An empty types list is valid under ADR-0033 — it means "use the built-in defaults".
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  types: []
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_CommitsEmptyTypeName(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  types:
    - name: feat
    - name: ""
`)
	e := findErr(config.Validate(cfg), "commits.types[1].name")
	require.NotNil(t, e)
}

func TestValidate_CommitsInvalidTypeName(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  types:
    - name: feat
    - name: "not a type"
`)
	e := findErr(config.Validate(cfg), "commits.types[1].name")
	require.NotNil(t, e)
}

func TestValidate_CommitsDuplicateType(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  types:
    - name: feat
    - name: feat
`)
	e := findErr(config.Validate(cfg), "commits.types[1].name")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "duplicate")
}

func TestValidate_CommitsScopeNameRequired(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  scopes:
    - name: cmd
    - name: ""
`)
	e := findErr(config.Validate(cfg), "commits.scopes[1].name")
	require.NotNil(t, e)
}

func TestValidate_CommitsScopesRestrictedWithDefaultsOK(t *testing.T) {
	// The built-in default scopes (deps/deps-dev/release) satisfy scopes_restricted, so it is
	// valid with no user-listed scopes.
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  scopes_restricted: true
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_CommitsScopesRestrictedAllRemoved(t *testing.T) {
	// With every default scope removed and none added, scopes_restricted has nothing to
	// allow → error.
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  scopes_restricted: true
  scopes:
    - name: deps
      remove: true
    - name: deps-dev
      remove: true
    - name: release
      remove: true
`)
	e := findErr(config.Validate(cfg), "commits.scopes_restricted")
	require.NotNil(t, e)
}

func TestValidate_CommitsAbsent_NoError(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
`)
	assert.Empty(t, config.Validate(cfg))
}
