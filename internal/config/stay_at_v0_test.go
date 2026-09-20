package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersioning_StayAtV0_NilSafe(t *testing.T) {
	assert.False(t, config.Versioning{}.StayAtV0(), "no bump block")
	assert.False(t, config.Versioning{Bump: &config.BumpConfig{}}.StayAtV0(), "bump block without the key")
	assert.True(t, config.Versioning{Bump: &config.BumpConfig{StayAtV0: true}}.StayAtV0())
}

// TestBumpConfig_StayAtV0_LoadsThroughStrictLoader proves the key is a known field: config.Load
// rejects unknown keys, so this fails until BumpConfig actually declares stay_at_v0.
func TestBumpConfig_StayAtV0_LoadsThroughStrictLoader(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
version: "1"
versioning:
  strategy: semver
  bump:
    mode: auto
    stay_at_v0: true
`), 0o644))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.True(t, cfg.Versioning.StayAtV0())
}
