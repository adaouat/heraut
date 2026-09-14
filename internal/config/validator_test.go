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
  bump:
    mode: auto
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

// ── bump (root-level, T225) ──────────────────────────────────────────────────

func TestValidate_invalidBump(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  bump:
    mode: sometimes
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.bump.mode")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "sometimes")
	assert.Contains(t, e.Hint, "auto")
	assert.Contains(t, e.Hint, "manual")
}

func TestValidate_validBumpModes(t *testing.T) {
	for _, bump := range []string{"auto", "manual"} {
		t.Run(bump, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  bump:
    mode: `+bump+`
`)
			assert.Empty(t, config.Validate(cfg))
		})
	}
}

// ── bump.overrides (T261) ───────────────────────────────────────────────────

func TestValidate_bumpOverride_missingMatcher(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  bump:
    mode: auto
    overrides:
      - bump: none
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.bump.overrides[0]")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "at least one")
}

func TestValidate_bumpOverride_typeAndRegexBothSet(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  bump:
    mode: auto
    overrides:
      - type: chore
        regex: '^chore\(deps'
        bump: none
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.bump.overrides[0]")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "only one")
}

func TestValidate_bumpOverride_invalidRegex(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  bump:
    mode: auto
    overrides:
      - regex: '['
        bump: none
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.bump.overrides[0].regex")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "invalid regex")
}

func TestValidate_bumpOverride_invalidBumpLevel(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  bump:
    mode: auto
    overrides:
      - type: chore
        bump: gigantic
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.bump.overrides[0].bump")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "gigantic")
	assert.Contains(t, e.Hint, "major")
	assert.Contains(t, e.Hint, "none")
}

func TestValidate_bumpOverride_breakingOnlyMatcherIsValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  bump:
    mode: auto
    overrides:
      - breaking: true
        bump: minor
`)
	assert.Empty(t, config.Validate(cfg))
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

// ── enrichment_policy (renamed from remote_metadata, T160) ──────────────────────

func TestValidate_invalidEnrichmentPolicy(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  enrichment_policy: sometimes
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "commits.enrichment_policy")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "sometimes")
	assert.Contains(t, e.Hint, "required")
	assert.Contains(t, e.Hint, "optional")
	assert.Contains(t, e.Hint, "disabled")
}

func TestValidate_validEnrichmentPolicy(t *testing.T) {
	for _, v := range []string{"required", "optional", "disabled"} {
		t.Run(v, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  enrichment_policy: `+v+`
`)
			assert.Empty(t, config.Validate(cfg))
		})
	}
}

func TestValidate_emptyEnrichmentPolicyIsValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
`)
	assert.Nil(t, findErr(config.Validate(cfg), "enrichment_policy"))
}

// TestLoad_RemovedKey_RemoteMetadata confirms the removed commits.remote_metadata key now
// fails at load time with the migration error, instead of the old enum-validation path
// (superseded by TestLoad_RemovedKeys in migration_test.go — this asserts mustLoad's own
// callers see the same failure via config.Load's error, not just config.Validate).
func TestLoad_RemovedKey_RemoteMetadata(t *testing.T) {
	_, err := config.LoadFromReader(strings.NewReader(`
version: "1"
versioning:
  strategy: semver
commits:
  remote_metadata: required
`))
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrRemovedConfigKey)
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

// ── sprint (required when format contains the SPRINT token, T225) ──────────────

func TestValidate_sprintRequiredWithSprintToken(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.SPRINT.PATCH"
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.sprint")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "required")
}

func TestValidate_sprintSetSatisfiesSprintToken(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.SPRINT.PATCH"
  sprint: 3
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_sprintNotRequiredWithoutSprintToken(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
`)
	assert.Nil(t, findErr(config.Validate(cfg), "versioning.sprint"))
}

// ── generators ───────────────────────────────────────────────────────────────

// TestValidate_changelogAbsentGeneratorIsValid pins the T177 follow-up (Step 7): once
// generator: is a removed key, an absent generator must not also be a validator error — native
// is implicit. This specific test exists ahead of T180's broader cleanup because a real config
// going through Load-then-Validate (e.g. this repo's own .config/heraut.yml, loaded by
// `heraut commit verify`, this project's commit-msg hook) has no valid state otherwise.
func TestValidate_changelogAbsentGeneratorIsValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  output: CHANGELOG.md
`)
	errs := config.Validate(cfg)
	assert.Nil(t, findErr(errs, "changelog.generator"))
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
forges:
  - name: gh
    platform: aws
`)
	errs := config.Validate(cfg)
	assert.GreaterOrEqual(t, len(errs), 3, "expected at least 3 errors")
	assert.NotNil(t, findErr(errs, "version"))
	assert.NotNil(t, findErr(errs, "versioning.strategy"))
	assert.NotNil(t, findErr(errs, "forges[0].platform"))
}

// ── fixture-based tests ───────────────────────────────────────────────────────

func TestValidate_validFixtures(t *testing.T) {
	fixtures := []string{
		"../../testdata/config/valid/semver.yml",
		"../../testdata/config/valid/calver.yml",
		"../../testdata/config/valid/semver-per-env.yml",
		"../../testdata/config/valid/calver-per-env.yml",
		"../../testdata/config/valid/platform-base-url.yml",
		"../../testdata/config/valid/changelog-rotation-calver.yml",
		"../../testdata/config/valid/changelog-rotation-semver.yml",
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
		{
			fixture:     "../../testdata/config/invalid/changelog_rotation_wrong_family.yml",
			wantPath:    "changelog.output",
			wantMessage: "YYYY",
		},
		{
			fixture:     "../../testdata/config/invalid/changelog_rotation_not_prefix.yml",
			wantPath:    "changelog.output",
			wantMessage: "prefix",
		},
		{
			fixture:     "../../testdata/config/invalid/changelog_rotation_per_env.yml",
			wantPath:    "changelog.output",
			wantMessage: "per-env",
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
      output: CHANGELOG.md
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.changelog")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unreachable")
}

// TestValidate_disableReleaseAndReleaseOverride covers T217: disable_release: true now turns off
// the entire release: behavior for the environment (notes and publish together), so ANY
// environments.<env>.release override — not just release.notes — becomes unreachable, unlike the
// pre-T217 disable_notes toggle which only shadowed the notes sub-block.
func TestValidate_disableReleaseAndReleaseOverride(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
    disable_release: true
    release:
      notes:
        tag_pattern: "v[0-9]*"
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "environments.dev.release")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unreachable")
}

// ── tickets ──────────────────────────────────────────────────────────────────

func TestValidate_TicketsValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
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

// ── rendering.templates / template (native only) ──────────────────────────────

func TestValidate_RenderingTemplatesNativeValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    commit:
      message: "- {{ upperFirst .Description }} ({{ .ShortHash }})"
`)
	assert.Empty(t, config.Validate(cfg))
}

// TestValidate_RenderingTemplatesTicketValid covers T240: "commit.ticket" is a valid overridable
// block, added alongside "commit.message" so ticket-link rendering can be customized in isolation.
func TestValidate_RenderingTemplatesTicketValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    commit:
      ticket: "🎫[{{ .Text }}]({{ .Href }})"
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_RenderingTemplatesTitleSubtitleValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    title: "# MyApp Changelog"
    subtitle: "All notable changes."
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_RenderingTemplatesBadSnippet(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    commit:
      message: "{{ .Description "
`)
	e := findErr(config.Validate(cfg), "rendering.templates.commit.message")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "template")
}

func TestValidate_RenderingTemplatesUnknownBlock(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    commits: "- {{ .Description }}"
`)
	e := findErr(config.Validate(cfg), "rendering.templates.commits")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unknown template block")
}

// TestValidate_RenderingTemplatesNamespacedBlocksValid covers ADR-0059: the release- and
// commit-cadence blocks nest under release:/commit: YAML objects and flatten to dotted keys.
func TestValidate_RenderingTemplatesNamespacedBlocksValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    release:
      section: "## [{{ .Version }}]"
      group: "{{ .HeadingPrefix }} {{ .Name }}"
      contributors: "contribs"
      stats: "stats"
      footer: "rel-footer"
    commit:
      message: "- {{ .Description }}"
      ticket: "[{{ .Text }}]"
      contributor: "* {{ .Author.Username }}"
    release_notes: "{{range .Groups}}{{ template \"release.group\" . }}{{end}}"
`)
	assert.Empty(t, config.Validate(cfg))
}

// The pre-rename block names are config errors after ADR-0048 — no deprecated-alias shim.
func TestValidate_RenderingTemplatesOldHeaderKeyRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    header: "## [{{ .Version }}]"
`)
	e := findErr(config.Validate(cfg), "rendering.templates.header")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unknown template block")
	assert.Contains(t, e.Hint, "release.section")
}

// TestValidate_RenderingTemplatesOldFlatBlockKeysRejected covers ADR-0059: the ADR-0037/
// ADR-0048/ADR-0049-vintage flat block names are config errors now that release-/commit-cadence
// blocks are namespaced — no deprecated-alias shim, consistent with every prior block rename.
func TestValidate_RenderingTemplatesOldFlatBlockKeysRejected(t *testing.T) {
	tests := []struct {
		name, snippetYAML, wantPath string
	}{
		{"release_header", `release_header: "## [{{ .Version }}]"`, "release_header"},
		{"group", `group: "{{ .HeadingPrefix }} {{ .Name }}"`, "group"},
		{"commit", `commit: "- {{ .Description }}"`, "commit"},
		{"ticket", `ticket: "[{{ .Text }}]"`, "ticket"},
		{"contributor", `contributor: "* {{ .Author.Username }}"`, "contributor"},
		{"contributors", `contributors: "contribs"`, "contributors"},
		{"stats", `stats: "stats"`, "stats"},
		{"release_footer", `release_footer: "rel-footer"`, "release_footer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    `+tc.snippetYAML+`
`)
			e := findErr(config.Validate(cfg), "rendering.templates."+tc.wantPath)
			require.NotNil(t, e)
			assert.Contains(t, e.Message, "unknown template block")
		})
	}
}

// TestValidate_RenderingTemplatesUnknownNestedSubBlockRejected covers ADR-0059: an unrecognized
// sub-key under release:/commit: is caught by the same "unknown template block" mechanism as any
// other unrecognized key, keyed by its flattened dotted path.
func TestValidate_RenderingTemplatesUnknownNestedSubBlockRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    release:
      bogus: "..."
`)
	e := findErr(config.Validate(cfg), "rendering.templates.release.bogus")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unknown template block")
}

func TestValidate_RenderingTemplatesOldHyphenatedReleaseNotesKeyRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    release-notes: "{{range .Groups}}{{ template \"group\" . }}{{end}}"
`)
	e := findErr(config.Validate(cfg), "rendering.templates.release-notes")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unknown template block")
	assert.Contains(t, e.Hint, "release_notes")
}

func TestValidate_RenderingTrailersValidConfigs(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"renderer", `
        - token: Co-authored-by
          renderer: "**Co-authored by:** {{ .Value }}"`},
		{"hide", `
        - token: Refs
          hide: true`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    commit:
      trailers:`+tc.yaml+`
`)
			assert.Empty(t, config.Validate(cfg))
		})
	}
}

func TestValidate_RenderingTrailersTokenRequired(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    commit:
      trailers:
        - hide: true
`)
	e := findErr(config.Validate(cfg), "rendering.templates.commit.trailers[0].token")
	require.NotNil(t, e)
}

func TestValidate_RenderingTrailersExactlyOneOfRendererHide(t *testing.T) {
	tests := []struct {
		name      string
		rule      string
		wantMatch string
	}{
		{"neither set", `
        - token: Refs`, "exactly one"},
		{"both set", `
        - token: Refs
          renderer: "{{ .Value }}"
          hide: true`, "only one"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    commit:
      trailers:`+tc.rule+`
`)
			e := findErr(config.Validate(cfg), "rendering.templates.commit.trailers[0]")
			require.NotNil(t, e)
			assert.Contains(t, e.Message, tc.wantMatch)
		})
	}
}

func TestValidate_RenderingTrailersBadRendererSnippet(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    commit:
      trailers:
        - token: Refs
          renderer: "{{ .Value "
`)
	e := findErr(config.Validate(cfg), "rendering.templates.commit.trailers[0].renderer")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "template")
}

func TestValidate_RenderingTrailersDuplicateToken(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    commit:
      trailers:
        - token: Refs
          hide: true
        - token: refs
          renderer: "{{ .Value }}"
`)
	e := findErr(config.Validate(cfg), "rendering.templates.commit.trailers[1].token")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "duplicate")
}

func TestValidate_DriverTemplateFileMissing(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  template: .config/heraut/does-not-exist.tmpl
`)
	e := findErr(config.Validate(cfg), "changelog.template")
	require.NotNil(t, e)
}

// ── tag_pattern ──────────────────────────────────────────────────────────────

// TestValidate_NativeTagPatternAccepted verifies an explicit tag_pattern is now valid with native
// (T139), treated as a Go regex.
func TestValidate_NativeTagPatternAccepted(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
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
  tag_pattern: "["
`)
	e := findErr(config.Validate(cfg), "changelog.tag_pattern")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "invalid regex")
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

// ── commits.rules (ADR-0056) ──────────────────────────────────────────────────

func TestValidate_CommitsRule_ValidConfigs(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"deny, default target", `
    - name: no-wip
      deny: '(?i)\bwip\b'
      message: "commit message must not contain WIP"`},
		{"require", `
    - name: require-scope-word
      require: 'x'
      message: "must contain x"`},
		{"require_ticket", `
    - name: needs-ticket
      require_ticket: true`},
		{"target header", `
    - name: no-wip-header
      deny: 'wip'
      message: "no wip"
      target: header`},
		{"target body", `
    - name: no-wip-body
      deny: 'wip'
      message: "no wip"
      target: body`},
		{"target footer", `
    - name: no-wip-footer
      deny: 'wip'
      message: "no wip"
      target: footer`},
		{"target message", `
    - name: no-wip-message
      deny: 'wip'
      message: "no wip"
      target: message`},
		{"scoped by types and scopes", `
    - name: scoped
      deny: 'wip'
      message: "no wip"
      types: [fix]
      scopes: [cmd]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  tickets:
    - pattern: '[A-Z]{2,}-\d+'
      url: "https://jira.example.com/browse/{ticket}"
  rules:`+tc.yaml+`
`)
			assert.Empty(t, config.Validate(cfg))
		})
	}
}

func TestValidate_CommitsRuleExactlyOneOfDenyRequireRequireTicket(t *testing.T) {
	tests := []struct {
		name      string
		rule      string
		wantMatch string
	}{
		{"none set", `
    - name: broken
      message: "nothing to check"`, "exactly one"},
		{"deny and require both set", `
    - name: ambiguous
      deny: 'wip'
      require: 'x'
      message: "conflicting"`, "only one"},
		{"deny and require_ticket both set", `
    - name: ambiguous
      deny: 'wip'
      require_ticket: true
      message: "conflicting"`, "only one"},
		{"all three set", `
    - name: ambiguous
      deny: 'wip'
      require: 'x'
      require_ticket: true
      message: "conflicting"`, "only one"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  tickets:
    - pattern: '[A-Z]{2,}-\d+'
      url: "https://jira.example.com/browse/{ticket}"
  rules:`+tc.rule+`
`)
			e := findErr(config.Validate(cfg), "commits.rules[0]")
			require.NotNil(t, e)
			assert.Contains(t, e.Message, tc.wantMatch)
		})
	}
}

func TestValidate_CommitsRuleNameRequired(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  rules:
    - deny: 'wip'
      message: "no wip"
`)
	e := findErr(config.Validate(cfg), "commits.rules[0].name")
	require.NotNil(t, e)
}

func TestValidate_CommitsRuleDuplicateName(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  rules:
    - name: dup
      deny: 'wip'
      message: "a"
    - name: dup
      deny: 'todo'
      message: "b"
`)
	e := findErr(config.Validate(cfg), "commits.rules[1].name")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "duplicate")
}

func TestValidate_CommitsRuleInvalidRegex(t *testing.T) {
	tests := []struct {
		name string
		rule string
		path string
	}{
		{"deny", `
    - name: bad-deny
      deny: '('
      message: "bad"`, "commits.rules[0].deny"},
		{"require", `
    - name: bad-require
      require: '('
      message: "bad"`, "commits.rules[0].require"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  rules:`+tc.rule+`
`)
			e := findErr(config.Validate(cfg), tc.path)
			require.NotNil(t, e)
			assert.Contains(t, e.Message, "invalid regex")
		})
	}
}

func TestValidate_CommitsRuleMessageRequiredForDenyAndRequire(t *testing.T) {
	tests := []struct {
		name string
		rule string
	}{
		{"deny", `
    - name: no-message
      deny: 'wip'`},
		{"require", `
    - name: no-message
      require: 'x'`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  rules:`+tc.rule+`
`)
			e := findErr(config.Validate(cfg), "commits.rules[0].message")
			require.NotNil(t, e)
		})
	}
}

func TestValidate_CommitsRuleRequireTicketNeedsNonEmptyTickets(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  rules:
    - name: needs-ticket
      require_ticket: true
`)
	e := findErr(config.Validate(cfg), "commits.rules[0].require_ticket")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "commits.tickets")
}

func TestValidate_CommitsRuleInvalidTarget(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commits:
  rules:
    - name: bad-target
      deny: 'wip'
      message: "no wip"
      target: nonsense
`)
	e := findErr(config.Validate(cfg), "commits.rules[0].target")
	require.NotNil(t, e)
}

// ── changelog rotation tokens (T246) ─────────────────────────────────────────

func TestValidate_rotationOutput_NoTokens_Valid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  output: CHANGELOG.md
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_rotationOutput_CalverSingleToken_Valid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
changelog:
  output: "CHANGELOG_{YYYY}.md"
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_rotationOutput_CalverMultiToken_Valid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
changelog:
  output: "CHANGELOG_{YYYY}_{MM}.md"
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_rotationOutput_CalverNotAPrefix_Error(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
changelog:
  output: "CHANGELOG_{MM}.md"
`)
	e := findErr(config.Validate(cfg), "changelog.output")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "prefix")
}

func TestValidate_rotationOutput_CalverTokenNotInFormat_Error(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
changelog:
  output: "CHANGELOG_{QQ}.md"
`)
	e := findErr(config.Validate(cfg), "changelog.output")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "QQ")
}

func TestValidate_rotationOutput_CalverDuplicateToken_Error(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
changelog:
  output: "CHANGELOG_{YYYY}_{YYYY}.md"
`)
	e := findErr(config.Validate(cfg), "changelog.output")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "once")
}

func TestValidate_rotationOutput_SemverMajorOnly_Valid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  output: "CHANGELOG_{MAJOR}.md"
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_rotationOutput_SemverMajorMinor_Valid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  output: "CHANGELOG_{MAJOR}_{MINOR}.md"
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_rotationOutput_SemverMinorAlone_Error(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  output: "CHANGELOG_{MINOR}.md"
`)
	e := findErr(config.Validate(cfg), "changelog.output")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "MAJOR")
}

func TestValidate_rotationOutput_WrongFamily_Error(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  output: "CHANGELOG_{YYYY}.md"
`)
	e := findErr(config.Validate(cfg), "changelog.output")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "YYYY")
}

func TestValidate_rotationOutput_PerEnvStrategy_RootOutput_Error(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"
changelog:
  output: "CHANGELOG_{YYYY}.md"
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
`)
	e := findErr(config.Validate(cfg), "changelog.output")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "per-env")
}

func TestValidate_rotationOutput_PerEnvStrategy_EnvOutput_Error(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
    changelog:
      output: "CHANGELOG_{YYYY}.md"
`)
	e := findErr(config.Validate(cfg), "environments.dev.changelog.output")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "per-env")
}

// TestValidateChangelogRotationForWizard covers T258: `heraut init` let a user pick
// calver-per-env/semver-per-env and then type a rotating changelog output with no live feedback,
// producing a .heraut.yml that failed config.Validate on the very next command. This wraps the same
// validateChangelogRotation logic config.Validate uses, exposed for the wizard's live per-keystroke
// field validation — the wizard has no full *Config yet, only the strategy/format picked so far.
func TestValidateChangelogRotationForWizard(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		format   string
		output   string
		wantErr  string
	}{
		{"no tokens, valid", "calver-per-env", "YYYY.MM.PATCH", "CHANGELOG.md", ""},
		{"calver-per-env with tokens, rejected", "calver-per-env", "YYYY.MM.PATCH", "CHANGELOG_{YYYY}.md", "per-env"},
		{"semver-per-env with tokens, rejected", "semver-per-env", "", "CHANGELOG_{MAJOR}.md", "per-env"},
		{"flat calver with valid tokens", "calver", "YYYY.MM.PATCH", "CHANGELOG_{YYYY}.md", ""},
		{"flat calver, token not in format", "calver", "YYYY.MM.PATCH", "CHANGELOG_{QQ}.md", "QQ"},
		{"flat semver with valid tokens", "semver", "", "CHANGELOG_{MAJOR}.md", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := config.ValidateChangelogRotationForWizard(tc.strategy, tc.format, tc.output)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

// TestValidateTagFormatForWizard covers T259, part of the same audit as T258: heraut init's
// "Common tag format" and "Tag format override" fields had no live validation against
// validatePerEnv's actual {version} requirement (internal/config/validator.go), so a mistyped
// tag_format sailed through the wizard and only failed on the next command. Same rule, shared
// with validatePerEnv itself via tagFormatMissingVersion so the two can't drift apart.
func TestValidateTagFormatForWizard(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr string
	}{
		{"empty is valid", "", ""},
		{"contains version", "{env}/{version}", ""},
		{"missing version", "{env}", "{version}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := config.ValidateTagFormatForWizard(tc.format)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
