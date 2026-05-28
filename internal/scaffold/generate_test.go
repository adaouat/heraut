package scaffold_test

import (
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateYAML_SchemaHeader(t *testing.T) {
	out, err := scaffold.GenerateYAML(scaffold.Defaults(), "dev")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, "# yaml-language-server: $schema="), "missing schema header")
	assert.Contains(t, out, "https://raw.githubusercontent.com/adaouat/heraut/main/schema.json")
}

func TestGenerateYAML_VersionedSchemaHeader(t *testing.T) {
	out, err := scaffold.GenerateYAML(scaffold.Defaults(), "v1.2.3")
	require.NoError(t, err)
	assert.Contains(t, out, "https://raw.githubusercontent.com/adaouat/heraut/v1.2.3/schema.json")
}

func TestGenerateYAML_DefaultsRoundTrip(t *testing.T) {
	out, err := scaffold.GenerateYAML(scaffold.Defaults(), "dev")
	require.NoError(t, err)

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)

	errs := config.Validate(cfg)
	assert.Empty(t, errs, "defaults YAML did not pass validation: %v", errs)
}

func TestGenerateYAML_SemVer(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "semver",
		Prefix:             "v",
		ChangelogGenerator: "git-cliff",
		ChangelogOutput:    "CHANGELOG.md",
		NotesGenerator:     "git-cliff",
		Platforms:          []scaffold.PlatformAnswer{{Type: "github", Repository: "org/repo"}},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)

	errs := config.Validate(cfg)
	assert.Empty(t, errs)
	assert.Equal(t, "semver", cfg.Versioning.Strategy)
	require.NotNil(t, cfg.Versioning.Prefix)
	assert.Equal(t, "v", *cfg.Versioning.Prefix)
}

func TestGenerateYAML_CalVer(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "calver",
		Prefix:             "",
		Format:             "YYYY.MM.PATCH",
		ChangelogGenerator: "git-cliff",
		ChangelogOutput:    "CHANGELOG.md",
		Platforms:          []scaffold.PlatformAnswer{{Type: "gitlab"}},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)

	errs := config.Validate(cfg)
	assert.Empty(t, errs)
	assert.Equal(t, "calver", cfg.Versioning.Strategy)
	assert.Equal(t, "YYYY.MM.PATCH", cfg.Versioning.Format)
	// Empty prefix must survive the round-trip as &"" so the resolver does not
	// fall back to the "v" default (nil = unset = use default).
	require.NotNil(t, cfg.Versioning.Prefix)
	assert.Equal(t, "", *cfg.Versioning.Prefix)
}

// TestGenerateYAML_EmptyPrefix_ExplicitlyWritten verifies that leaving the
// wizard prefix blank produces prefix: "" in YAML — not an absent field,
// which would silently fall back to the "v" default in the resolver.
func TestGenerateYAML_EmptyPrefix_ExplicitlyWritten(t *testing.T) {
	a := scaffold.Answers{
		Strategy: "semver",
		Prefix:   "",
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	assert.Contains(t, out, "prefix:", "prefix key must be written even when empty")

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)
	require.NotNil(t, cfg.Versioning.Prefix)
	assert.Equal(t, "", *cfg.Versioning.Prefix)
}

func TestGenerateYAML_PerEnv(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "semver-per-env",
		TagFormat:          "{env}/{version}",
		ChangelogGenerator: "git-cliff",
		ChangelogOutput:    "CHANGELOG.md",
		Platforms:          []scaffold.PlatformAnswer{{Type: "gitlab"}},
		Environments: []scaffold.EnvAnswer{
			{Name: "dev", Bump: "auto"},
			{Name: "prod", Bump: "promote", Source: "dev"},
		},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)

	errs := config.Validate(cfg)
	assert.Empty(t, errs)
	assert.Equal(t, "semver-per-env", cfg.Versioning.Strategy)
	assert.Contains(t, cfg.Environments, "dev")
	assert.Contains(t, cfg.Environments, "prod")
}

func TestGenerateYAML_CalVerSprint(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "calver",
		Format:             "YYYY.SPRINT.PATCH",
		Sprint:             3,
		ChangelogGenerator: "git-cliff",
		ChangelogOutput:    "CHANGELOG.md",
		Platforms:          []scaffold.PlatformAnswer{{Type: "gitlab"}},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)

	errs := config.Validate(cfg)
	assert.Empty(t, errs)
	assert.Equal(t, 3, cfg.Versioning.Sprint)
}

func TestGenerateYAML_SprintOmittedWhenZero(t *testing.T) {
	a := scaffold.Answers{
		Strategy:  "calver",
		Format:    "YYYY.MM.PATCH",
		Sprint:    0,
		Platforms: []scaffold.PlatformAnswer{{Type: "gitlab"}},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	assert.NotContains(t, out, "sprint:")
}

func TestGenerateYAML_NoPlatforms(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "semver",
		ChangelogGenerator: "git-cliff",
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)
	assert.Nil(t, cfg.Release)
}

// stripHeader removes the leading comment line(s) so the YAML body can be parsed.
func stripHeader(yaml string) string {
	lines := strings.Split(yaml, "\n")
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
