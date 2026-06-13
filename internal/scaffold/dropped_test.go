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

func TestDroppedFields_Tickets(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Tickets:    []config.Ticket{{Pattern: `JIRA-(\d+)`, URL: "https://example.atlassian.net/browse/{ticket}"}},
	}
	assert.Equal(t, []string{"tickets"}, scaffold.DroppedFields(cfg))
}

func TestDroppedFields_RemoteMetadata(t *testing.T) {
	cfg := &config.Config{
		Version:        "1",
		Versioning:     config.Versioning{Strategy: "semver"},
		RemoteMetadata: "required",
	}
	assert.Equal(t, []string{"remote_metadata"}, scaffold.DroppedFields(cfg))
}

func TestDroppedFields_ReleaseAssets(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release:    &config.Release{Assets: []string{"dist/*.tar.gz"}},
	}
	assert.Equal(t, []string{"release.assets"}, scaffold.DroppedFields(cfg))
}

func TestDroppedFields_PlatformBaseURL_CustomFlagged(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{
			Platforms: []config.Platform{
				{Name: "github", Type: "github", BaseURL: "https://github.example.com"},
			},
		},
	}
	assert.Equal(t, []string{"release.platforms.github.base_url"}, scaffold.DroppedFields(cfg))
}

func TestDroppedFields_PlatformBaseURL_DefaultNotFlagged(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{
			Platforms: []config.Platform{
				{Name: "github", Type: "github", BaseURL: "https://github.com"},
			},
		},
	}
	assert.Empty(t, scaffold.DroppedFields(cfg))
}

func TestDroppedFields_PlatformDraftAndPrerelease(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{
			Platforms: []config.Platform{
				{Name: "github", Type: "github", BaseURL: "https://github.com", Draft: true, Prerelease: true},
			},
		},
	}
	assert.Equal(t, []string{"release.platforms.github.draft", "release.platforms.github.prerelease"}, scaffold.DroppedFields(cfg))
}

func TestDroppedFields_EnvChangelogOverride(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Changelog: &config.ContentDriver{Generator: "git-cliff"}},
		},
	}
	assert.Equal(t, []string{"environments.prod.changelog"}, scaffold.DroppedFields(cfg))
}

func TestDroppedFields_EnvReleaseOverride(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", Release: &config.EnvRelease{Notes: &config.ContentDriver{Generator: "git-cliff"}}},
		},
	}
	assert.Equal(t, []string{"environments.prod.release"}, scaffold.DroppedFields(cfg))
}

func TestDroppedFields_MultipleEnvsSortedByName(t *testing.T) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"staging": {Bump: "auto", Changelog: &config.ContentDriver{Generator: "git-cliff"}},
			"prod":    {Bump: "auto", Changelog: &config.ContentDriver{Generator: "git-cliff"}},
		},
	}
	assert.Equal(t, []string{"environments.prod.changelog", "environments.staging.changelog"}, scaffold.DroppedFields(cfg))
}
