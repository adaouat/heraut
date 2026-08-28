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

// ── bump (root-level, T225) ──────────────────────────────────────────────────

func TestValidate_invalidBump(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
  bump: sometimes
`)
	errs := config.Validate(cfg)
	e := findErr(errs, "versioning.bump")
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
  bump: `+bump+`
`)
			assert.Empty(t, config.Validate(cfg))
		})
	}
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
    commit: "- {{ upperFirst .Description }} ({{ .ShortHash }})"
`)
	assert.Empty(t, config.Validate(cfg))
}

// TestValidate_RenderingTemplatesTicketValid covers T240: "ticket" is a valid overridable
// block, added alongside "commit" so ticket-link rendering can be customized in isolation.
func TestValidate_RenderingTemplatesTicketValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
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
    commit: "{{ .Description "
`)
	e := findErr(config.Validate(cfg), "rendering.templates.commit")
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

func TestValidate_RenderingTemplatesRenamedBlocksValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    release_header: "## [{{ .Version }}]"
    release_notes: "{{range .Groups}}{{ template \"group\" . }}{{end}}"
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
	assert.Contains(t, e.Hint, "release_header")
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
