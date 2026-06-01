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
	t.Setenv("HERAUT_FILE", "")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".config"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".config", "heraut.yml"), []byte("version: '1'"), 0o644))

	assert.Equal(t, ".config/heraut.yml", config.ResolvePath(""))
}

func TestResolvePath_dotHerautPresent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", "")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".heraut.yml"), []byte("version: '1'"), 0o644))

	assert.Equal(t, ".heraut.yml", config.ResolvePath(""))
}

func TestResolvePath_fallbackWhenNoFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", "")

	// Neither .config/heraut.yml nor .heraut.yml exists: return default.
	assert.Equal(t, ".heraut.yml", config.ResolvePath(""))
}

func TestResolvePath_dotConfigTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", "")
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

func TestResolvePath_envVarWhitespaceOnlyFallsThrough(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", "   ")
	// Whitespace-only is treated as unset; falls through to auto-discovery.
	assert.Equal(t, ".heraut.yml", config.ResolvePath(""))
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

func TestResolvePathWithSource(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T)
		explicit   string
		wantPath   string
		wantSource config.PathSource
	}{
		{
			name:       "explicit flag wins",
			setup:      func(t *testing.T) { t.Setenv("HERAUT_FILE", "") },
			explicit:   "/flag/path.yml",
			wantPath:   "/flag/path.yml",
			wantSource: config.SourceFlag,
		},
		{
			name: "flag wins over env var",
			setup: func(t *testing.T) {
				t.Setenv("HERAUT_FILE", "/env/path.yml")
			},
			explicit:   "/flag/path.yml",
			wantPath:   "/flag/path.yml",
			wantSource: config.SourceFlag,
		},
		{
			name: "env var",
			setup: func(t *testing.T) {
				t.Setenv("HERAUT_FILE", "/env/path.yml")
			},
			wantPath:   "/env/path.yml",
			wantSource: config.SourceEnvVar,
		},
		{
			name: "xdg config auto-discovery",
			setup: func(t *testing.T) {
				dir := t.TempDir()
				t.Chdir(dir)
				t.Setenv("HERAUT_FILE", "")
				require.NoError(t, os.MkdirAll(filepath.Join(dir, ".config"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, ".config", "heraut.yml"), []byte("version: '1'"), 0o644))
			},
			wantPath:   ".config/heraut.yml",
			wantSource: config.SourceXDGConfig,
		},
		{
			name: "default fallback when no files exist",
			setup: func(t *testing.T) {
				dir := t.TempDir()
				t.Chdir(dir)
				t.Setenv("HERAUT_FILE", "")
			},
			wantPath:   ".heraut.yml",
			wantSource: config.SourceDefault,
		},
		{
			name: "default fallback when only .heraut.yml exists",
			setup: func(t *testing.T) {
				dir := t.TempDir()
				t.Chdir(dir)
				t.Setenv("HERAUT_FILE", "")
				require.NoError(t, os.WriteFile(filepath.Join(dir, ".heraut.yml"), []byte("version: '1'"), 0o644))
			},
			wantPath:   ".heraut.yml",
			wantSource: config.SourceDefault,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			path, src := config.ResolvePathWithSource(tc.explicit)
			assert.Equal(t, tc.wantPath, path)
			assert.Equal(t, tc.wantSource, src)
		})
	}
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
