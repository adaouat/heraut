package selfupdate

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultCacheDir_XDGSet(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg")
	got, err := defaultCacheDir()
	require.NoError(t, err)
	assert.Equal(t, "/tmp/xdg/heraut", got)
}

func TestDefaultCacheDir_Default(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	got, err := defaultCacheDir()
	require.NoError(t, err)
	assert.Contains(t, got, "heraut")
}

func TestPermissionError_PermissionDenied(t *testing.T) {
	err := permissionError("/usr/local/bin/heraut", &os.PathError{Err: syscall.EACCES})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
	assert.Contains(t, err.Error(), "sudo")
}

func TestPermissionError_OtherError(t *testing.T) {
	err := permissionError("/usr/local/bin/heraut", os.ErrNotExist)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replacing binary")
}
