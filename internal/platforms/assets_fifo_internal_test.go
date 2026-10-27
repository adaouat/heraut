//go:build unix

package platforms

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Opening a FIFO for reading blocks until a writer appears, so a FIFO among the assets must fail
// fast rather than hang the release after the tag is pushed.

func openWithin(t *testing.T, patterns []string) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- openAfterSwap(t, patterns) }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("OpenAssets blocked on a FIFO")
		return nil
	}
}

func TestOpenAssets_RefusesAFIFOBeforeOpeningAnything(t *testing.T) {
	repo, _ := swapRepo(t)
	require.NoError(t, syscall.Mkfifo(filepath.Join(repo, "dist", "pipe"), 0o600))
	swapBeforeOpen(t, func(path string) { t.Errorf("opened %q although an asset is not a regular file", path) })

	err := openWithin(t, []string{"dist/*"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dist/pipe")
	assert.Contains(t, err.Error(), "not a regular file")
}

func TestOpenAssets_FIFOSwappedInBeforeTheOpenFailsFast(t *testing.T) {
	repo, _ := swapRepo(t)
	target := filepath.Join(repo, "dist", "app.zip")
	swapBeforeOpen(t, func(string) {
		require.NoError(t, os.Remove(target))
		require.NoError(t, syscall.Mkfifo(target, 0o600))
	})

	err := openWithin(t, []string{"dist/app.zip"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed after it was checked")
}

// The pre-release check refuses a FIFO too, by its mode alone: it opens nothing, so it cannot block.
func TestCheckAssetGlobs_RefusesAFIFOWithoutBlocking(t *testing.T) {
	repo, _ := swapRepo(t)
	require.NoError(t, syscall.Mkfifo(filepath.Join(repo, "dist", "pipe"), 0o600))

	done := make(chan error, 1)
	go func() { done <- CheckAssetGlobs([]string{"dist/*"}, false) }()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a regular file")
		assert.NotErrorIs(t, err, port.ErrAssetUnmatched, "a dry run must refuse it too")
	case <-time.After(5 * time.Second):
		t.Fatal("CheckAssetGlobs blocked on a FIFO")
	}
}
