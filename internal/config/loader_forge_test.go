package config_test

import (
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
