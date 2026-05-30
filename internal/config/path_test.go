package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestResolvePath_explicit(t *testing.T) {
	assert.Equal(t, "/some/explicit/path.yml", config.ResolvePath("/some/explicit/path.yml"))
}

func TestResolvePath_dotConfigPresent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".config"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".config", "heraut.yml"), []byte("version: '1'"), 0o644))

	assert.Equal(t, ".config/heraut.yml", config.ResolvePath(""))
}

func TestResolvePath_dotHerautPresent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".heraut.yml"), []byte("version: '1'"), 0o644))

	assert.Equal(t, ".heraut.yml", config.ResolvePath(""))
}

func TestResolvePath_fallbackWhenNoFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Neither .config/heraut.yml nor .heraut.yml exists: return default.
	assert.Equal(t, ".heraut.yml", config.ResolvePath(""))
}

func TestResolvePath_dotConfigTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".config"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".config", "heraut.yml"), []byte("version: '1'"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".heraut.yml"), []byte("version: '1'"), 0o644))

	assert.Equal(t, ".config/heraut.yml", config.ResolvePath(""))
}

func TestResolvePath_envVar(t *testing.T) {
	t.Setenv("HERAUT_FILE", "/env/path.yml")
	assert.Equal(t, "/env/path.yml", config.ResolvePath(""))
}

func TestResolvePath_explicitFlagWinsOverEnvVar(t *testing.T) {
	t.Setenv("HERAUT_FILE", "/env/path.yml")
	assert.Equal(t, "/flag/path.yml", config.ResolvePath("/flag/path.yml"))
}

func TestResolvePath_envVarEmptyStringFallsThrough(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", "")
	// Empty string is treated as unset; falls through to auto-discovery.
	assert.Equal(t, ".heraut.yml", config.ResolvePath(""))
}

func TestResolvePath_envVarWinsOverAutoDiscovery(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".config"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".config", "heraut.yml"), []byte("version: '1'"), 0o644))

	t.Setenv("HERAUT_FILE", "/env/path.yml")
	assert.Equal(t, "/env/path.yml", config.ResolvePath(""))
}

func TestInitDest_withDotConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".config"), 0o755))

	assert.Equal(t, ".config/heraut.yml", config.InitDest())
}

func TestInitDest_withoutDotConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	assert.Equal(t, ".heraut.yml", config.InitDest())
}
