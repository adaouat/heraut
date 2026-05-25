package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd_ExistsOnRoot(t *testing.T) {
	root := cmd.NewRootCmd()
	var found bool
	for _, c := range root.Commands() {
		if c.Use == "init" {
			found = true
		}
	}
	assert.True(t, found, "init command missing from root")
}

func TestInitCmd_DefaultsCreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")

	_, err := executeRoot("init", "--defaults", "--config", cfgPath)
	require.NoError(t, err)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "yaml-language-server")
	assert.Contains(t, string(data), "semver")
}

func TestInitCmd_DefaultsProducesValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")

	_, err := executeRoot("init", "--defaults", "--config", cfgPath)
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	errs := config.Validate(cfg)
	assert.Empty(t, errs)
}

func TestInitCmd_DefaultsForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("existing content"), 0o644))

	_, err := executeRoot("init", "--defaults", "--force", "--config", cfgPath)
	require.NoError(t, err)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "semver")
	assert.NotContains(t, string(data), "existing content")
}

func TestInitCmd_DefaultsWithExistingNoForceOverwrites(t *testing.T) {
	// --defaults is always non-interactive: existing file is overwritten without prompt.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("existing content"), 0o644))

	_, err := executeRoot("init", "--defaults", "--config", cfgPath)
	require.NoError(t, err)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "semver")
}
