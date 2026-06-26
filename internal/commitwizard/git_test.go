package commitwizard

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasStaged(t *testing.T) {
	t.Run("staged when name-only output is non-empty", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("internal/cmd/commit.go\n", "", nil)
		staged, err := hasStaged(mr)
		require.NoError(t, err)
		assert.True(t, staged)
		assert.Equal(t, "git", mr.Calls[0].Name)
		assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[0].Args)
	})
	t.Run("not staged when output is empty", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil)
		staged, err := hasStaged(mr)
		require.NoError(t, err)
		assert.False(t, staged)
	})
	t.Run("propagates runner error", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", errors.New("boom"))
		_, err := hasStaged(mr)
		require.Error(t, err)
	})
}

func TestHasWorkingTreeChanges(t *testing.T) {
	t.Run("dirty when porcelain output is non-empty", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse(" M internal/x.go\n?? new.go\n", "", nil)
		dirty, err := hasWorkingTreeChanges(mr)
		require.NoError(t, err)
		assert.True(t, dirty)
		assert.Equal(t, "git", mr.Calls[0].Name)
		assert.Equal(t, []string{"status", "--porcelain"}, mr.Calls[0].Args)
	})
	t.Run("clean when porcelain output is empty", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil)
		dirty, err := hasWorkingTreeChanges(mr)
		require.NoError(t, err)
		assert.False(t, dirty)
	})
	t.Run("propagates runner error", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", errors.New("boom"))
		_, err := hasWorkingTreeChanges(mr)
		require.Error(t, err)
	})
}

func TestStageAll(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)
	require.NoError(t, stageAll(mr))
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"add", "-A"}, mr.Calls[0].Args)
}

func TestCommit(t *testing.T) {
	t.Run("git commit -F <tmpfile>", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil)
		require.NoError(t, commit(mr, "feat: x", false))
		require.Len(t, mr.Calls, 1)
		args := mr.Calls[0].Args
		require.Len(t, args, 3)
		assert.Equal(t, "commit", args[0])
		assert.Equal(t, "-F", args[1])
		assert.NotEmpty(t, args[2]) // dynamic tmpfile path
	})
	t.Run("all adds -a", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil)
		require.NoError(t, commit(mr, "feat: x", true))
		args := mr.Calls[0].Args
		require.Len(t, args, 4)
		assert.Equal(t, []string{"commit", "-a", "-F"}, args[:3])
	})
	t.Run("propagates runner error", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "rejected by hook", errors.New("exit 1"))
		require.Error(t, commit(mr, "feat: x", false))
	})
}
