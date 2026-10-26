package platforms_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/platforms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func openAssets(t *testing.T, patterns []string, warn func(string)) ([]platforms.Asset, error) {
	t.Helper()
	assets, err := platforms.OpenAssets(patterns, warn)
	t.Cleanup(func() { platforms.CloseAssets(assets) })
	return assets, err
}

func readAll(t *testing.T, a platforms.Asset) string {
	t.Helper()
	b, err := io.ReadAll(a.File)
	require.NoError(t, err)
	return string(b)
}

func TestOpenAssets_OpensEveryMatch(t *testing.T) {
	repo := inRepo(t)
	require.NoError(t, os.Mkdir(filepath.Join(repo, "dist"), 0o755))
	writeFile(t, filepath.Join(repo, "dist"), "app.zip", "zip bytes")
	writeFile(t, repo, "notes.txt", "hello")

	assets, err := openAssets(t, []string{"dist/*.zip", filepath.Join(repo, "*.txt")}, noWarn(t))
	require.NoError(t, err)
	require.Len(t, assets, 2)

	assert.Equal(t, "dist/app.zip", assets[0].Path)
	assert.Equal(t, "app.zip", assets[0].Name)
	assert.Equal(t, int64(len("zip bytes")), assets[0].Size)
	assert.Equal(t, "zip bytes", readAll(t, assets[0]))

	assert.Equal(t, filepath.Join(repo, "notes.txt"), assets[1].Path)
	assert.Equal(t, "notes.txt", assets[1].Name)
	assert.Equal(t, int64(5), assets[1].Size)
	assert.Equal(t, "hello", readAll(t, assets[1]))
}

func TestOpenAssets_SymlinkInsideTheRootIsNamedAfterTheLink(t *testing.T) {
	repo := inRepo(t)
	require.NoError(t, os.Mkdir(filepath.Join(repo, "dist"), 0o755))
	writeFile(t, filepath.Join(repo, "dist"), "app-1.2.3.zip", "zip bytes")
	require.NoError(t, os.Symlink(filepath.Join("dist", "app-1.2.3.zip"), filepath.Join(repo, "latest.zip")))

	assets, err := openAssets(t, []string{"latest.zip"}, noWarn(t))
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, "latest.zip", assets[0].Name)
	assert.Equal(t, "zip bytes", readAll(t, assets[0]))
}

func TestOpenAssets_LeadingDashNeedsNoPrefix(t *testing.T) {
	repo := inRepo(t)
	writeFile(t, repo, "-rf.zip", "x")

	assets, err := openAssets(t, []string{"*.zip"}, noWarn(t))
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, "-rf.zip", assets[0].Path, "the file is opened, never passed to a tool as an argument")
	assert.Equal(t, "-rf.zip", assets[0].Name)
}

func TestOpenAssets_SkipsDirectoriesAndWarnsOnNoMatch(t *testing.T) {
	repo := inRepo(t)
	require.NoError(t, os.Mkdir(filepath.Join(repo, "dist"), 0o755))
	writeFile(t, repo, "app.zip", "x")

	var warned []string
	assets, err := openAssets(t, []string{"*", "missing/*.tar.gz"}, func(p string) { warned = append(warned, p) })
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, "app.zip", assets[0].Name)
	assert.Equal(t, []string{"missing/*.tar.gz"}, warned)
}

func TestOpenAssets_ADirectoryOnlyMatchWarns(t *testing.T) {
	repo := inRepo(t)
	require.NoError(t, os.Mkdir(filepath.Join(repo, "dist"), 0o755))

	var warned []string
	assets, err := openAssets(t, []string{"dist"}, func(p string) { warned = append(warned, p) })
	require.NoError(t, err)
	assert.Empty(t, assets)
	assert.Equal(t, []string{"dist"}, warned)
}

func TestOpenAssets_InvalidGlobErrors(t *testing.T) {
	inRepo(t)
	_, err := openAssets(t, []string{"[invalid"}, noWarn(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid glob pattern")
}

func TestOpenAssets_RefusesAbsolutePathOutsideTheRoot(t *testing.T) {
	inRepo(t)
	outside := t.TempDir()
	writeFile(t, outside, "secret.txt", "secret")

	assets, err := openAssets(t, []string{filepath.Join(outside, "*.txt")}, noWarn(t))
	require.Error(t, err)
	assert.Nil(t, assets)
	assert.Contains(t, err.Error(), "outside the repository")
	assert.Contains(t, err.Error(), "secret.txt")
}

func TestOpenAssets_RefusesParentTraversal(t *testing.T) {
	repo := inRepo(t)
	writeFile(t, filepath.Dir(repo), "sibling.bin", "x")
	t.Cleanup(func() { _ = os.Remove(filepath.Join(filepath.Dir(repo), "sibling.bin")) })

	_, err := openAssets(t, []string{"../sibling.bin"}, noWarn(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
}

func TestOpenAssets_RefusesSymlinkLeavingTheRoot(t *testing.T) {
	repo := inRepo(t)
	outside := t.TempDir()
	writeFile(t, outside, "secret.txt", "secret")
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(repo, "innocent.txt")))

	_, err := openAssets(t, []string{"innocent.txt"}, noWarn(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
}

func TestOpenAssets_RefusesDuplicateBasenames(t *testing.T) {
	repo := inRepo(t)
	for _, d := range []string{"linux", "darwin"} {
		require.NoError(t, os.Mkdir(filepath.Join(repo, d), 0o755))
		writeFile(t, filepath.Join(repo, d), "app.zip", d)
	}

	assets, err := openAssets(t, []string{"linux/*.zip", "darwin/*.zip"}, noWarn(t))
	require.Error(t, err)
	assert.Nil(t, assets)
	assert.Contains(t, err.Error(), `"app.zip"`)
	assert.Contains(t, err.Error(), "linux/app.zip")
	assert.Contains(t, err.Error(), "darwin/app.zip")
}

func TestOpenAssets_RefusesTheSameFileMatchedTwice(t *testing.T) {
	repo := inRepo(t)
	writeFile(t, repo, "app.zip", "x")

	_, err := openAssets(t, []string{"*.zip", "app.zip"}, noWarn(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"app.zip"`)
}

func TestCloseAssets_ToleratesNil(t *testing.T) {
	platforms.CloseAssets(nil)
}
