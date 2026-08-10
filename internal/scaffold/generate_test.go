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
		TagPrefix:          "v",
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
	require.NotNil(t, cfg.Versioning.TagPrefix)
	assert.Equal(t, "v", *cfg.Versioning.TagPrefix)
}

func TestGenerateYAML_CalVer(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "calver",
		TagPrefix:          "",
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
	require.NotNil(t, cfg.Versioning.TagPrefix)
	assert.Equal(t, "", *cfg.Versioning.TagPrefix)
}

// TestGenerateYAML_EmptyPrefix_ExplicitlyWritten verifies that leaving the
// wizard prefix blank produces prefix: "" in YAML — not an absent field,
// which would silently fall back to the "v" default in the resolver.
func TestGenerateYAML_EmptyPrefix_ExplicitlyWritten(t *testing.T) {
	a := scaffold.Answers{
		Strategy:  "semver",
		TagPrefix: "",
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	assert.Contains(t, out, "tag_prefix:", "tag_prefix key must be written even when empty")

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)
	require.NotNil(t, cfg.Versioning.TagPrefix)
	assert.Equal(t, "", *cfg.Versioning.TagPrefix)
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

func TestGenerateYAML_PlatformNamesDefaultedAndDeduped(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "semver",
		ChangelogGenerator: "git-cliff",
		ChangelogOutput:    "CHANGELOG.md",
		Platforms: []scaffold.PlatformAnswer{
			{Type: "github", Repository: "acme/widget"},
			{Type: "gitlab", Project: "acme/widget"},
			{Type: "gitlab", Project: "tools/widget-mirror"},
		},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)

	body := stripHeader(out)
	cfg, err := config.LoadFromReader(strings.NewReader(body))
	require.NoError(t, err)
	assert.Empty(t, config.Validate(cfg))

	require.Len(t, cfg.Forges, 3)
	assert.Equal(t, "github", cfg.Forges[0].Name)
	assert.Equal(t, "gitlab", cfg.Forges[1].Name)
	assert.Equal(t, "gitlab-2", cfg.Forges[2].Name)

	require.Len(t, cfg.Release.Targets, 3)
	assert.Equal(t, "github", cfg.Release.Targets[0].Forge)
	assert.Equal(t, "gitlab", cfg.Release.Targets[1].Forge)
	assert.Equal(t, "gitlab-2", cfg.Release.Targets[2].Forge)
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

func TestConfigToAnswers_PreservesAssetsTicketsRemoteMetadata(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Commits: &config.Commits{
			EnrichmentPolicy: "required",
			Tickets:          []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}},
		},
		Release: &config.Release{Assets: []string{"dist/*.tar.gz"}},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "required", a.RemoteMetadata)
	assert.Equal(t, []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}}, a.Tickets)
	assert.Equal(t, []string{"dist/*.tar.gz"}, a.Assets)
}

func TestGenerateYAML_AssetsTicketsRemoteMetadata(t *testing.T) {
	a := scaffold.Answers{
		Strategy:       "semver",
		RemoteMetadata: "required",
		Tickets:        []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}},
		Assets:         []string{"dist/*.tar.gz"},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)

	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)
	assert.Equal(t, "required", cfg.EnrichmentPolicy())
	assert.Equal(t, []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}}, cfg.Tickets())
	require.NotNil(t, cfg.Release)
	assert.Equal(t, []string{"dist/*.tar.gz"}, cfg.Release.Assets)
}

func TestConfigToAnswers_PreservesPlatformPassthroughFields(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Forges: []config.Forge{
			{
				Name: "gh-internal", Type: "github", Repository: "org/repo",
				BaseURL: "https://github.example.com", TokenEnv: "GH_TOKEN",
			},
		},
		Release: &config.Release{
			Targets: []config.Target{{Forge: "gh-internal", Draft: true, Prerelease: true}},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	require.Len(t, a.Platforms, 1)
	assert.Equal(t, "gh-internal", a.Platforms[0].Name)
	assert.Equal(t, "https://github.example.com", a.Platforms[0].BaseURL)
	assert.True(t, a.Platforms[0].Draft)
	assert.True(t, a.Platforms[0].Prerelease)
}

func TestGenerateYAML_PlatformUsesPassthroughName(t *testing.T) {
	a := scaffold.Answers{
		Strategy:       "semver",
		NotesGenerator: "git-cliff",
		Platforms: []scaffold.PlatformAnswer{
			{Name: "gh-internal", Type: "github", Repository: "org/repo", TokenEnv: "GH_TOKEN"},
		},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)
	require.Len(t, cfg.Forges, 1)
	assert.Equal(t, "gh-internal", cfg.Forges[0].Name)
	require.Len(t, cfg.Release.Targets, 1)
	assert.Equal(t, "gh-internal", cfg.Release.Targets[0].Forge)
}

func TestGenerateYAML_PlatformPassthroughFieldsRoundTrip(t *testing.T) {
	a := scaffold.Answers{
		Strategy:       "semver",
		NotesGenerator: "git-cliff",
		Platforms: []scaffold.PlatformAnswer{
			{
				Name: "gh-internal", Type: "github", Repository: "org/repo", TokenEnv: "GH_TOKEN",
				BaseURL: "https://github.example.com", Draft: true, Prerelease: true,
			},
		},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)
	require.Len(t, cfg.Forges, 1)
	assert.Equal(t, "https://github.example.com", cfg.Forges[0].BaseURL)
	require.Len(t, cfg.Release.Targets, 1)
	target := cfg.Release.Targets[0]
	assert.True(t, target.Draft)
	assert.True(t, target.Prerelease)
}

func TestConfigToAnswers_PreservesEnvPassthroughFields(t *testing.T) {
	driver := &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"}
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"prod": {
				Bump:      "auto",
				Changelog: driver,
				Release:   &config.EnvRelease{Notes: &config.ContentDriver{Generator: "communique"}},
			},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	require.Len(t, a.Environments, 1)
	assert.Equal(t, driver, a.Environments[0].Changelog)
	assert.NotNil(t, a.Environments[0].Release)
}

func TestGenerateYAML_EnvPassthroughFieldsRoundTrip(t *testing.T) {
	a := scaffold.Answers{
		Strategy: "semver-per-env",
		Environments: []scaffold.EnvAnswer{
			{
				Name:      "prod",
				Bump:      "auto",
				Changelog: &config.ContentDriver{TagPattern: "prod/changelog/*", Output: "CHANGELOG.md"},
				Release:   &config.EnvRelease{Notes: &config.ContentDriver{TagPattern: "prod/notes/*"}},
			},
		},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)
	prod := cfg.Environments["prod"]
	require.NotNil(t, prod.Changelog)
	assert.Equal(t, "prod/changelog/*", prod.Changelog.TagPattern)
	require.NotNil(t, prod.Release)
	assert.Equal(t, "prod/notes/*", prod.Release.Notes.TagPattern)
}

func TestConfigToAnswers_DefaultsEmptyChangelogOutput(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
		Changelog: &config.ContentDriver{
			Generator: "git-cliff",
			Output:    "",
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "CHANGELOG.md", a.ChangelogOutput)
}

func TestGenerateYAML_DefaultsEmptyChangelogOutput(t *testing.T) {
	a := scaffold.Answers{
		Strategy:           "semver",
		ChangelogGenerator: "git-cliff",
		ChangelogOutput:    "",
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	assert.Contains(t, out, "output: CHANGELOG.md")
}

// TestConfigToAnswers_PreservesEnrichmentForge pins I7: ConfigToAnswers never read back
// commits.enrichment_forge, so re-running `heraut init` on a two-forge config whose enrichment
// source is the *second* forge silently repointed it at the first (answersToConfig's
// stopgap default) — with no warning. Round-tripping through Answers (without running the
// wizard, which never edits forge selection — that redesign is T164/P4) must preserve the
// existing choice.
func TestConfigToAnswers_PreservesEnrichmentForge(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Commits:    &config.Commits{EnrichmentForge: "gitlab-internal", EnrichmentPolicy: "optional"},
		Forges: []config.Forge{
			{Name: "github", Type: "github", Repository: "acme/widget"},
			{Name: "gitlab-internal", Type: "gitlab", Project: "acme/widget"},
		},
		Release: &config.Release{
			Targets: []config.Target{{Forge: "github"}, {Forge: "gitlab-internal"}},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "gitlab-internal", a.EnrichmentForge)
}

func TestGenerateYAML_PreservesEnrichmentForgeOnSecondForge(t *testing.T) {
	a := scaffold.Answers{
		Strategy:        "semver",
		EnrichmentForge: "gitlab-internal",
		Platforms: []scaffold.PlatformAnswer{
			{Name: "github", Type: "github", Repository: "acme/widget"},
			{Name: "gitlab-internal", Type: "gitlab", Project: "acme/widget"},
		},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)
	require.NotNil(t, cfg.Commits)
	assert.Equal(t, "gitlab-internal", cfg.Commits.EnrichmentForge,
		"re-running heraut init must not silently repoint enrichment_forge at the first forge")
	assert.Empty(t, config.Validate(cfg))
}

func TestGenerateYAML_EnrichmentForgeDefaultsToFirstWhenUnset(t *testing.T) {
	a := scaffold.Answers{
		Strategy: "semver",
		Platforms: []scaffold.PlatformAnswer{
			{Type: "github", Repository: "acme/widget"},
			{Type: "gitlab", Project: "acme/widget"},
		},
	}
	out, err := scaffold.GenerateYAML(a, "dev")
	require.NoError(t, err)
	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)
	require.NotNil(t, cfg.Commits)
	assert.Equal(t, cfg.Forges[0].Name, cfg.Commits.EnrichmentForge,
		"first-forge default remains the stopgap when no prior choice exists")
}
