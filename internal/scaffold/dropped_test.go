package scaffold_test

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateKeeps_Empty(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
	assertKeptOnUpdate(t, cfg)
}

func TestUpdateKeeps_DefaultsConfig(t *testing.T) {
	out, err := scaffold.GenerateYAML(scaffold.Defaults(), "dev")
	require.NoError(t, err)

	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)

	assertKeptOnUpdate(t, cfg, "defaults config should round-trip without warnings")
}

func TestUpdateKeeps_Tickets_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Commits:    &config.Commits{Tickets: []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}}},
	}
	assertKeptOnUpdate(t, cfg, "tickets are preserved via Answers (T107)")
}

func TestUpdateKeeps_EnrichmentPolicy_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Commits:    &config.Commits{EnrichmentPolicy: "required"},
	}
	assertKeptOnUpdate(t, cfg, "enrichment_policy is preserved via Answers (T107)")
}

func TestUpdateKeeps_ReleaseAssets_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release:    &config.Release{Assets: []string{"dist/*.tar.gz"}},
	}
	assertKeptOnUpdate(t, cfg, "release.assets is preserved via Answers (T107)")
}

func TestUpdateKeeps_PlatformBaseURL_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Forges: []config.Forge{
			{Name: "github", Type: "github", BaseURL: "https://github.example.com"},
		},
		Release: &config.Release{
			Targets: []config.Target{{Forge: "github"}},
		},
	}
	assertKeptOnUpdate(t, cfg, "base_url is carried through via passthrough fields (T108)")
}

// TestDroppedFields_PlatformDraft_NotDropped covers only `draft` now — `prerelease` was removed
// from config.Target (ADR-0064, T337): GitHub's pre-release flag is derived from the resolved
// version, not carried through the wizard at all.
func TestUpdateKeeps_PlatformDraft_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Forges: []config.Forge{
			{Name: "github", Type: "github", BaseURL: "https://github.com"},
		},
		Release: &config.Release{
			Targets: []config.Target{{Forge: "github", Draft: true}},
		},
	}
	assertKeptOnUpdate(t, cfg, "draft is carried through via passthrough fields (T108)")
}

func TestDroppedPlatformFields_NoMismatch(t *testing.T) {
	cfg := &config.Config{
		Forges: []config.Forge{
			{Name: "github", Type: "github", BaseURL: "https://github.example.com"},
		},
		Release: &config.Release{
			Targets: []config.Target{{Forge: "github", Draft: true}},
		},
	}
	rebuilt := []scaffold.PlatformAnswer{
		{Type: "github", Name: "github"},
	}
	assert.Empty(t, scaffold.DroppedPlatformFields(cfg, rebuilt), "single-type match → no post-wizard drops")
}

func TestDroppedPlatformFields_Mismatch_OriginalLonger(t *testing.T) {
	// original had 2 github entries with overrides; rebuilt has only 1 → 2nd is lost
	cfg := &config.Config{
		Forges: []config.Forge{
			{Name: "github-com", Type: "github", BaseURL: "https://github.com"},
			{Name: "github-ent", Type: "github", BaseURL: "https://github.example.com"},
		},
		Release: &config.Release{
			Targets: []config.Target{
				{Forge: "github-com"},
				{Forge: "github-ent", Draft: true},
			},
		},
	}
	rebuilt := []scaffold.PlatformAnswer{
		{Type: "github", Name: "github-com"},
	}
	dropped := scaffold.DroppedPlatformFields(cfg, rebuilt)
	assert.Contains(t, dropped, "release.targets.github-ent.base_url")
	assert.Contains(t, dropped, "release.targets.github-ent.draft")
}

func TestDroppedPlatformFields_NilCfg(t *testing.T) {
	assert.Empty(t, scaffold.DroppedPlatformFields(nil, nil), "nil cfg (fresh init) must not panic")
}

func TestDroppedPlatformFields_NilRelease(t *testing.T) {
	cfg := &config.Config{}
	assert.Empty(t, scaffold.DroppedPlatformFields(cfg, nil))
}

func TestUpdateKeeps_EnvChangelogOverride_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Changelog: &config.ContentDriver{}},
		},
	}
	assertKeptOnUpdate(t, cfg, "env.changelog is carried through via passthrough fields (T109)")
}

func TestUpdateKeeps_EnvReleaseOverride_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Release: &config.EnvRelease{Notes: &config.ContentDriver{}}},
		},
	}
	assertKeptOnUpdate(t, cfg, "env.release is carried through via passthrough fields (T109)")
}

func TestDroppedEnvFields_NoMismatch(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Changelog: &config.ContentDriver{}},
		},
	}
	rebuilt := []scaffold.EnvAnswer{{Name: "prod", Bump: "auto"}}
	assert.Empty(t, scaffold.DroppedEnvFields(cfg, rebuilt), "name matched → no drops")
}

func TestDroppedEnvFields_Mismatch_Renamed(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Changelog: &config.ContentDriver{}},
		},
	}
	rebuilt := []scaffold.EnvAnswer{{Name: "production", Bump: "auto"}}
	dropped := scaffold.DroppedEnvFields(cfg, rebuilt)
	assert.Contains(t, dropped, "environments.prod.changelog")
}

func TestDroppedEnvFields_NilCfg(t *testing.T) {
	assert.Empty(t, scaffold.DroppedEnvFields(nil, nil), "nil cfg (fresh init) must not panic")
}

func TestDroppedEnvFields_NoEnvironments(t *testing.T) {
	cfg := &config.Config{}
	assert.Empty(t, scaffold.DroppedEnvFields(cfg, nil))
}

// assertKeptOnUpdate checks that updating cfg through the wizard, answers unchanged, writes back the
// same configuration (an unset tag_prefix may come back as its default).
func assertKeptOnUpdate(t *testing.T, cfg *config.Config, msgAndArgs ...any) {
	t.Helper()
	raw, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	want, err := config.LoadFromReader(bytes.NewReader(raw))
	require.NoError(t, err)

	content, err := scaffold.GenerateUpdatedYAML(want, scaffold.ConfigToAnswers(want), "dev")
	require.NoError(t, err)
	got, err := config.LoadFromReader(strings.NewReader(stripHeader(content)))
	require.NoError(t, err, content)

	assert.Equal(t, want, normalizePrefix(got, want), msgAndArgs...)
}
