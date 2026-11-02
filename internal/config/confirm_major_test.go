package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestVersioning_ConfirmMajor_NilSafe(t *testing.T) {
	assert.False(t, config.Versioning{}.ConfirmMajor(), "no bump block")
	assert.False(t, config.Versioning{Bump: &config.BumpConfig{}}.ConfirmMajor(), "bump block without the key")
	assert.False(t, config.Versioning{Bump: &config.BumpConfig{StayAtV0: true}}.ConfirmMajor(), "stay_at_v0 is a different key")
	assert.True(t, config.Versioning{Bump: &config.BumpConfig{ConfirmMajor: true}}.ConfirmMajor())
}

// config.Load rejects unknown keys, so this fails until BumpConfig declares confirm_major.
func TestBumpConfig_ConfirmMajor_LoadsThroughStrictLoader(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
version: "1"
versioning:
  strategy: semver
  bump:
    confirm_major: true
`), 0o644))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.True(t, cfg.Versioning.ConfirmMajor())
}
