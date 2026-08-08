package app_test

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMemoizingRunner_CachesIdenticalCalls pins T173: HasResolvablePublishTarget and
// BuildPipeline both resolve the forge from the same caller-supplied readRunner, and without
// memoization each spawns its own `git remote get-url origin` subprocess for a zero-config
// release. Wrapping readRunner in a memoizing runner once, before either call, is the fix.
func TestNewMemoizingRunner_CachesIdenticalCalls(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("https://github.com/acme/widget.git\n", "", nil) // only one response queued

	runner := app.NewMemoizingRunner(mr)

	out1, _, err1 := runner.Run("git", "remote", "get-url", "origin")
	require.NoError(t, err1)
	out2, _, err2 := runner.Run("git", "remote", "get-url", "origin")
	require.NoError(t, err2, "the second identical call must be served from cache, not spawn a second subprocess")

	assert.Equal(t, out1, out2)
	assert.Len(t, mr.Calls, 1, "the underlying runner must be invoked exactly once")
}

func TestNewMemoizingRunner_DistinctArgsNotConflated(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("origin-url\n", "", nil)
	mr.QueueResponse("v1.2.3\n", "", nil)

	runner := app.NewMemoizingRunner(mr)

	out1, _, err1 := runner.Run("git", "remote", "get-url", "origin")
	require.NoError(t, err1)
	out2, _, err2 := runner.Run("git", "describe", "--tags")
	require.NoError(t, err2)

	assert.NotEqual(t, out1, out2)
	assert.Len(t, mr.Calls, 2)
}

func TestNewMemoizingRunner_CachesErrors(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "exit status 1", errors.New("exit status 1"))

	runner := app.NewMemoizingRunner(mr)

	_, _, err1 := runner.Run("git", "remote", "get-url", "origin")
	require.Error(t, err1)
	_, _, err2 := runner.Run("git", "remote", "get-url", "origin")
	require.Error(t, err2, "a cached error must also be served from cache, not re-executed")
	assert.Len(t, mr.Calls, 1)
}
