package scaffold_test

import (
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaults_Strategy(t *testing.T) {
	a := scaffold.Defaults()
	assert.Equal(t, "semver", a.Strategy)
}

func TestDefaults_Prefix(t *testing.T) {
	a := scaffold.Defaults()
	assert.Equal(t, "v", a.TagPrefix)
}

func TestDefaults_EnableChangelogAndRelease(t *testing.T) {
	a := scaffold.Defaults()
	assert.True(t, a.EnableChangelog)
	assert.True(t, a.PublishReleases)
}

func TestDefaults_Platform(t *testing.T) {
	a := scaffold.Defaults()
	assert.Len(t, a.Platforms, 1)
	assert.Equal(t, "gitlab", a.Platforms[0].Type)
}

func TestConfigToAnswers_Strategy(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "calver", a.Strategy)
}

func TestConfigToAnswers_Prefix(t *testing.T) {
	prefix := "v"
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: &prefix},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "v", a.TagPrefix)
}

func TestConfigToAnswers_NilPrefix(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "", a.TagPrefix)
}

func TestConfigToAnswers_CalVerFormat(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "YYYY.MM.PATCH", a.Format)
}

func TestConfigToAnswers_EnableChangelog(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{Output: "CHANGELOG.md"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.True(t, a.EnableChangelog)
	assert.Equal(t, "CHANGELOG.md", a.ChangelogOutput)
}

func TestConfigToAnswers_NoChangelog(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.False(t, a.EnableChangelog)
}

func TestConfigToAnswers_Platforms(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Forges: []config.Forge{
			{Name: "github", Type: "github", Repository: "acme/widget", TokenEnv: "GH_TOKEN"},
			{Name: "gitlab", Type: "gitlab", Project: "acme/widget"},
		},
		Commits: &config.Commits{EnrichmentForge: "github"},
		Release: &config.Release{
			Targets: []config.Target{{Forge: "github"}, {Forge: "gitlab"}},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Len(t, a.Platforms, 2)
	assert.Equal(t, "github", a.Platforms[0].Type)
	assert.Equal(t, "acme/widget", a.Platforms[0].Repository)
	assert.Equal(t, "GH_TOKEN", a.Platforms[0].TokenEnv)
	assert.Equal(t, "gitlab", a.Platforms[1].Type)
}

func TestConfigToAnswers_Environments(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}",
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto"},
			"prod": {Bump: "promote", Source: "dev"},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, "{env}/{version}", a.TagFormat)
	assert.Len(t, a.Environments, 2)
	names := map[string]bool{}
	for _, e := range a.Environments {
		names[e.Name] = true
	}
	assert.True(t, names["dev"])
	assert.True(t, names["prod"])
}

func TestConfigToAnswers_Sprint(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.SPRINT.PATCH", Sprint: 5},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, 5, a.Sprint)
}

func TestConfigToAnswers_SprintZeroWhenAbsent(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Equal(t, 0, a.Sprint)
}

// TestConfigToAnswers_PublishReleasesFromReleasePresence covers T220 (release atomicity):
// PublishReleases must reflect release: block presence, not just whether any target resolved to
// a known forge — release.notes is no longer an independent signal (T216 default-populates it
// unconditionally), so a bare Release{} with no forges: entry must still round-trip as "release
// wanted."
func TestConfigToAnswers_PublishReleasesFromReleasePresence(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{
			Notes: &config.ContentDriver{},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.True(t, a.PublishReleases)
}

// TestConfigToAnswers_PublishReleasesZeroConfig_NoResolvedPlatforms guards the specific regression
// this task fixes: a zero-config release (release: {}, no forges:, no release.targets — the common
// CI shape per docs/specs/02-configuration.md) has zero entries in a.Platforms because
// ConfigToAnswers only populates Platforms from targets that resolve against cfg.Forges. Before
// T220, PublishReleases was derived from len(a.Platforms) > 0 and would wrongly come back false,
// silently dropping the release: block on an edit-existing-config wizard round trip.
func TestConfigToAnswers_PublishReleasesZeroConfig_NoResolvedPlatforms(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release:    &config.Release{Notes: &config.ContentDriver{}},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.Empty(t, a.Platforms, "zero-config release has no forges: entries to resolve targets against")
	assert.True(t, a.PublishReleases, "release: presence alone must round-trip as PublishReleases, independent of resolved platforms")
}

// TestConfigToAnswers_ChangelogAndReleasePresenceSurviveLoadRoundTrip guards against a regression
// where ConfigToAnswers under-reports block presence after a real config.Load round trip (as
// opposed to a hand-built struct literal, which could carry any value and would not catch a
// derivation bug). Re-running `heraut init` against an existing config and accepting the
// pre-populated defaults must not silently drop changelog/release generation.
func TestConfigToAnswers_ChangelogAndReleasePresenceSurviveLoadRoundTrip(t *testing.T) {
	yaml := `
version: "1"
versioning:
  strategy: semver
  tag_prefix: ""
changelog:
  output: CHANGELOG.md
forges:
  - name: Primary GitLab
    platform: gitlab
release:
  notes: {}
  targets:
    - forge: Primary GitLab
`
	cfg, err := config.LoadFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	a := scaffold.ConfigToAnswers(cfg)

	assert.True(t, a.EnableChangelog)
	assert.True(t, a.PublishReleases)
}

func TestValidateCalVerFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr string
	}{
		{"empty", "", "required"},
		{"missing PATCH", "YYYY.MM", "must contain PATCH"},
		{"PATCH not last", "PATCH.YYYY.MM", "PATCH must be the last"},
		{"no calendar token", "PATCH", "at least one calendar token"},
		{"unknown uppercase token", "YYYY.YYY.PATCH", `unknown token "YYY"`},
		{"unknown uppercase OOOO", "YYYY.OOOO.PATCH", `unknown token "OOOO"`},
		{"valid YYYY.MM.PATCH", "YYYY.MM.PATCH", ""},
		{"valid YYYY.WW.PATCH", "YYYY.WW.PATCH", ""},
		{"valid YYYY.SPRINT.PATCH", "YYYY.SPRINT.PATCH", ""},
		{"valid YYYY.PATCH", "YYYY.PATCH", ""},
		{"literal separator ok", "YYYY-MM.PATCH", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := scaffold.ValidateCalVerFormat(tc.format)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestDefaults_PublishReleases(t *testing.T) {
	a := scaffold.Defaults()
	assert.True(t, a.PublishReleases)
}

func TestConfigToAnswers_PublishReleasesWhenPlatformsExist(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Forges:     []config.Forge{{Name: "github", Type: "github", Repository: "acme/widget"}},
		Release: &config.Release{
			Targets: []config.Target{{Forge: "github"}},
		},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.True(t, a.PublishReleases)
}

func TestConfigToAnswers_NoPublishReleasesWhenNoPlatforms(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
	a := scaffold.ConfigToAnswers(cfg)
	assert.False(t, a.PublishReleases)
}
