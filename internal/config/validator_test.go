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
  prefix: "v"
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
    - platform: gitlab
`)
	assert.Empty(t, config.Validate(cfg))
}

// ── env overrides (top-level environments) ───────────────────────────────────

func TestValidate_envOverrideInvalidPlatform(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
environments:
  dev:
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
	e := findErr(errs, "versioning.environments")
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
	e := findErr(errs, "versioning.environments.dev.bump")
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
	e := findErr(errs, "versioning.environments.dev.bump")
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
	e := findErr(errs, "versioning.environments.dev.tag_format")
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
	e := findErr(errs, "versioning.environments.dev.tag_format")
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
	e := findErr(errs, "versioning.environments.dev.source")
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
	e := findErr(errs, "versioning.environments.prod.source")
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
	e := findErr(errs, "versioning.environments.prod.source")
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
	e := findErr(errs, "versioning.environments.prod.source")
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
	e := findErr(errs, "versioning.environments.prod.source")
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
			wantPath:    "versioning.environments",
			wantMessage: "required",
		},
		{
			fixture:     "../../testdata/config/invalid/source_ambiguous.yml",
			wantPath:    "versioning.environments.prod.source",
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
