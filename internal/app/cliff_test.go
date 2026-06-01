package app_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveCliffConfig_NilDriver(t *testing.T) {
	toml, err := app.EffectiveCliffConfig(&config.Config{}, nil, "changelog", "")
	require.NoError(t, err)
	assert.Contains(t, toml, "[changelog]")
	assert.Contains(t, toml, "[git]")
}

func TestEffectiveCliffConfig_EmptyDriver(t *testing.T) {
	driver := &config.ContentDriver{}
	toml, err := app.EffectiveCliffConfig(&config.Config{}, driver, "changelog", "")
	require.NoError(t, err)
	assert.Contains(t, toml, "[changelog]")
}

func TestEffectiveCliffConfig_ReleaseNotesMode(t *testing.T) {
	toml, err := app.EffectiveCliffConfig(&config.Config{}, nil, "release-notes", "")
	require.NoError(t, err)
	// release-notes uses a different template but still valid TOML
	assert.Contains(t, toml, "[changelog]")
}

func TestEffectiveCliffConfig_NonGitcliff(t *testing.T) {
	driver := &config.ContentDriver{Generator: "communique"}
	_, err := app.EffectiveCliffConfig(&config.Config{}, driver, "changelog", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "communique")
}

func TestEffectiveCliffConfig_BuildFormatInjectsPostprocessor(t *testing.T) {
	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}-{build}",
		},
	}
	driver := &config.ContentDriver{Generator: "git-cliff"}
	toml, err := app.EffectiveCliffConfig(cfg, driver, "changelog", "main")
	require.NoError(t, err)
	assert.Contains(t, toml, "postprocessors")
	assert.NotContains(t, toml, "postprocessors = []")
}
