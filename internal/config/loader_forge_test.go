package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidate_platformBaseURLTrailingSlashNormalized pins the C1 regression: a trailing
// slash on forges[].base_url must be trimmed by loader normalization (mirroring the deleted
// normalizePlatforms), since selfHosted()-style comparisons in the platform drivers are
// exact-match against the default host — an untrimmed trailing slash makes a default host
// look self-hosted and flips CI autologin off (GitLab) / skips the GITHUB_ACTIONS fast path
// (GitHub).
func TestValidate_platformBaseURLTrailingSlashNormalized(t *testing.T) {
	cfg, err := config.LoadFromReader(strings.NewReader(`
version: "1"
versioning:
  strategy: semver
forges:
  - name: github
    platform: github
    repository: acme/widget
    base_url: https://github.com/
    api_url: https://api.github.com/
release:
  targets:
    - forge: github
`))
	require.NoError(t, err)
	require.Len(t, cfg.Forges, 1)
	assert.Equal(t, "https://github.com", cfg.Forges[0].BaseURL)
	assert.Equal(t, "https://api.github.com", cfg.Forges[0].APIURL)
	assert.Empty(t, config.Validate(cfg))
}

func TestLoad_ForgesAndTargets(t *testing.T) {
	c, err := config.Load(filepath.Join("testdata", "forge-minimal.yml"))
	require.NoError(t, err)

	require.Len(t, c.Forges, 1)
	assert.Equal(t, "Primary GitLab", c.Forges[0].Name)
	assert.Equal(t, "gitlab", c.Forges[0].Type)
	assert.Equal(t, "rest", c.Forges[0].APIMode)

	require.NotNil(t, c.Commits)
	assert.Equal(t, "Primary GitLab", c.Commits.EnrichmentForge)
	assert.Equal(t, "optional", c.Commits.EnrichmentPolicy)

	require.NotNil(t, c.Release)
	require.Len(t, c.Release.Targets, 1)
	assert.Equal(t, "Primary GitLab", c.Release.Targets[0].Forge)
	assert.False(t, c.Release.Targets[0].Draft)
}
