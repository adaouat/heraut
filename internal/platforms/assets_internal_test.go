package platforms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests swap a file between the confinement check and the open (NEW-002, CWE-367). The
// beforeOpen hook stands in for the attacker's timing, so no test sleeps or races.

func swapBeforeOpen(t *testing.T, swap func(path string)) {
	t.Helper()
	prev := beforeOpen
	beforeOpen = swap
	t.Cleanup(func() { beforeOpen = prev })
}

func swapAfterStat(t *testing.T, swap func(path string)) {
	t.Helper()
	prev := afterStat
	afterStat = swap
	t.Cleanup(func() { afterStat = prev })
}

func swapRepo(t *testing.T) (repo, outside string) {
	t.Helper()
	repo = t.TempDir()
	t.Chdir(repo)
	outside = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(repo, "dist"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "dist", "app.zip"), []byte("zip bytes"), 0o600))
	return repo, outside
}

func openAfterSwap(t *testing.T, patterns []string) error {
	t.Helper()
	assets, err := OpenAssets(patterns, func(p string) { t.Errorf("unexpected no-match warning for %q", p) })
	CloseAssets(assets)
	assert.Nil(t, assets, "no handle is returned when one asset is refused")
	return err
}

func TestOpenAssets_FileSwappedForASymlinkLeavingTheRoot(t *testing.T) {
	repo, outside := swapRepo(t)
	target := filepath.Join(repo, "dist", "app.zip")
	swapBeforeOpen(t, func(string) {
		require.NoError(t, os.Remove(target))
		require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), target))
	})

	err := openAfterSwap(t, []string{"dist/app.zip"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dist/app.zip")
}

func TestOpenAssets_ParentDirectorySwappedForASymlinkLeavingTheRoot(t *testing.T) {
	repo, outside := swapRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(outside, "app.zip"), []byte("secret"), 0o600))
	swapBeforeOpen(t, func(string) {
		require.NoError(t, os.RemoveAll(filepath.Join(repo, "dist")))
		require.NoError(t, os.Symlink(outside, filepath.Join(repo, "dist")))
	})

	err := openAfterSwap(t, []string{"dist/app.zip"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dist/app.zip")
}

func TestOpenAssets_SymlinkRetargetedInsideTheRoot(t *testing.T) {
	repo, _ := swapRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "other.zip"), []byte("other"), 0o600))
	target := filepath.Join(repo, "dist", "app.zip")
	swapBeforeOpen(t, func(string) {
		require.NoError(t, os.Remove(target))
		// Relative: os.Root refuses every absolute symlink target, so only a relative one reaches SameFile.
		require.NoError(t, os.Symlink(filepath.Join("..", "other.zip"), target))
	})

	err := openAfterSwap(t, []string{"dist/app.zip"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed after it was checked")
}

func TestOpenAssets_FileReplacedByAnotherFile(t *testing.T) {
	repo, _ := swapRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "other.zip"), []byte("other"), 0o600))
	swapBeforeOpen(t, func(string) {
		require.NoError(t, os.Rename(filepath.Join(repo, "other.zip"), filepath.Join(repo, "dist", "app.zip")))
	})

	err := openAfterSwap(t, []string{"dist/app.zip"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed after it was checked")
}

func TestOpenAssets_FileReplacedByADirectory(t *testing.T) {
	repo, _ := swapRepo(t)
	target := filepath.Join(repo, "dist", "app.zip")
	swapBeforeOpen(t, func(string) {
		require.NoError(t, os.Remove(target))
		require.NoError(t, os.Mkdir(target, 0o755))
	})

	err := openAfterSwap(t, []string{"dist/app.zip"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dist/app.zip")
}

func TestOpenAssets_SwapOfALaterAssetClosesTheEarlierOnes(t *testing.T) {
	repo, outside := swapRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "dist", "b.zip"), []byte("b"), 0o600))
	target := filepath.Join(repo, "dist", "b.zip")
	var opened []string
	swapBeforeOpen(t, func(path string) {
		opened = append(opened, path)
		if path == "dist/b.zip" {
			require.NoError(t, os.Remove(target))
			require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), target))
		}
	})

	var files []*os.File
	prevAfterOpen := afterOpen
	afterOpen = func(f *os.File) { files = append(files, f) }
	t.Cleanup(func() { afterOpen = prevAfterOpen })

	err := openAfterSwap(t, []string{"dist/app.zip", "dist/b.zip"})
	require.Error(t, err)
	assert.Equal(t, []string{"dist/app.zip", "dist/b.zip"}, opened, "the first asset was opened before the second was refused")
	require.NotEmpty(t, files, "the first asset was opened")
	_, statErr := files[0].Stat()
	assert.ErrorIs(t, statErr, os.ErrClosed, "the first asset is closed when a later one is refused")
}

// Outside when stat'ed and when opened, inside only for the confinement check: SameFile alone would
// pass (both see the outside file), so this is the case the os.Root open exists for.
func TestOpenAssets_OutsideAtStatAndOpenInsideAtTheCheck(t *testing.T) {
	repo, outside := swapRepo(t)
	target := filepath.Join(repo, "dist", "app.zip")
	pointOutside := func() {
		require.NoError(t, os.Remove(target))
		require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), target))
	}
	pointOutside()
	swapAfterStat(t, func(string) {
		require.NoError(t, os.Remove(target))
		require.NoError(t, os.WriteFile(target, []byte("zip bytes"), 0o600))
	})
	swapBeforeOpen(t, func(string) { pointOutside() })

	err := openAfterSwap(t, []string{"dist/app.zip"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dist/app.zip")
}
