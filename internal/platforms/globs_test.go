package platforms_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/platforms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveGlobs_SinglePattern(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "a.zip")
	touch(t, dir, "b.zip")

	files, err := platforms.ResolveGlobs([]string{filepath.Join(dir, "*.zip")})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		filepath.Join(dir, "a.zip"),
		filepath.Join(dir, "b.zip"),
	}, files)
}

func TestResolveGlobs_MultiplePatterns(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "app.tar.gz")
	touch(t, dir, "app.zip")

	files, err := platforms.ResolveGlobs([]string{
		filepath.Join(dir, "*.tar.gz"),
		filepath.Join(dir, "*.zip"),
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		filepath.Join(dir, "app.tar.gz"),
		filepath.Join(dir, "app.zip"),
	}, files)
}

func TestResolveGlobs_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "file.bin")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))

	files, err := platforms.ResolveGlobs([]string{filepath.Join(dir, "*")})
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dir, "file.bin")}, files)
}

func TestResolveGlobs_NoMatches(t *testing.T) {
	dir := t.TempDir()

	_, err := platforms.ResolveGlobs([]string{filepath.Join(dir, "*.zip")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no files matched asset pattern")
}

func TestResolveGlobs_InvalidGlobSyntax(t *testing.T) {
	_, err := platforms.ResolveGlobs([]string{"[invalid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid glob pattern")
}

func touch(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}
