package scaffold_test

import (
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDroppedFields_Empty(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
	assert.Empty(t, scaffold.DroppedFields(cfg))
}

func TestDroppedFields_DefaultsConfig(t *testing.T) {
	out, err := scaffold.GenerateYAML(scaffold.Defaults(), "dev")
	require.NoError(t, err)

	cfg, err := config.LoadFromReader(strings.NewReader(stripHeader(out)))
	require.NoError(t, err)

	assert.Empty(t, scaffold.DroppedFields(cfg), "defaults config should round-trip without warnings")
}

func TestDroppedFields_Tickets_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Commits:    &config.Commits{Tickets: []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}}},
	}
	assert.Empty(t, scaffold.DroppedFields(cfg), "tickets are preserved via Answers (T107)")
}

func TestDroppedFields_RemoteMetadata_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Commits:    &config.Commits{RemoteMetadata: "required"},
	}
	assert.Empty(t, scaffold.DroppedFields(cfg), "remote_metadata is preserved via Answers (T107)")
}

func TestDroppedFields_ReleaseAssets_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release:    &config.Release{Assets: []string{"dist/*.tar.gz"}},
	}
	assert.Empty(t, scaffold.DroppedFields(cfg), "release.assets is preserved via Answers (T107)")
}

func TestDroppedFields_PlatformBaseURL_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{
			Platforms: []config.Platform{
				{Name: "github", Type: "github", BaseURL: "https://github.example.com"},
			},
		},
	}
	assert.Empty(t, scaffold.DroppedFields(cfg), "base_url is carried through via passthrough fields (T108)")
}

func TestDroppedFields_PlatformDraftAndPrerelease_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{
			Platforms: []config.Platform{
				{Name: "github", Type: "github", BaseURL: "https://github.com", Draft: true, Prerelease: true},
			},
		},
	}
	assert.Empty(t, scaffold.DroppedFields(cfg), "draft/prerelease are carried through via passthrough fields (T108)")
}

func TestDroppedPlatformFields_NoMismatch(t *testing.T) {
	cfg := &config.Config{
		Release: &config.Release{
			Platforms: []config.Platform{
				{Name: "github", Type: "github", BaseURL: "https://github.example.com", Draft: true},
			},
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
		Release: &config.Release{
			Platforms: []config.Platform{
				{Name: "github-com", Type: "github", BaseURL: "https://github.com"},
				{Name: "github-ent", Type: "github", BaseURL: "https://github.example.com", Draft: true},
			},
		},
	}
	rebuilt := []scaffold.PlatformAnswer{
		{Type: "github", Name: "github-com"},
	}
	dropped := scaffold.DroppedPlatformFields(cfg, rebuilt)
	assert.Contains(t, dropped, "release.platforms.github-ent.base_url")
	assert.Contains(t, dropped, "release.platforms.github-ent.draft")
}

func TestDroppedPlatformFields_NilRelease(t *testing.T) {
	cfg := &config.Config{}
	assert.Empty(t, scaffold.DroppedPlatformFields(cfg, nil))
}

func TestDroppedFields_EnvChangelogOverride_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Changelog: &config.ContentDriver{Generator: "git-cliff"}},
		},
	}
	assert.Empty(t, scaffold.DroppedFields(cfg), "env.changelog is carried through via passthrough fields (T109)")
}

func TestDroppedFields_EnvReleaseOverride_NotDropped(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Release: &config.EnvRelease{Notes: &config.ContentDriver{Generator: "git-cliff"}}},
		},
	}
	assert.Empty(t, scaffold.DroppedFields(cfg), "env.release is carried through via passthrough fields (T109)")
}

func TestDroppedEnvFields_NoMismatch(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Changelog: &config.ContentDriver{Generator: "git-cliff"}},
		},
	}
	rebuilt := []scaffold.EnvAnswer{{Name: "prod", Bump: "auto"}}
	assert.Empty(t, scaffold.DroppedEnvFields(cfg, rebuilt), "name matched → no drops")
}

func TestDroppedEnvFields_Mismatch_Renamed(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Changelog: &config.ContentDriver{Generator: "git-cliff"}},
		},
	}
	rebuilt := []scaffold.EnvAnswer{{Name: "production", Bump: "auto"}}
	dropped := scaffold.DroppedEnvFields(cfg, rebuilt)
	assert.Contains(t, dropped, "environments.prod.changelog")
}

func TestDroppedEnvFields_NoEnvironments(t *testing.T) {
	cfg := &config.Config{}
	assert.Empty(t, scaffold.DroppedEnvFields(cfg, nil))
}
