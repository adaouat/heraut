package pipeline

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHook_Success(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	err := runHook(mr, "echo hi")
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "sh", mr.Calls[0].Name)
	assert.Equal(t, []string{"-c", "echo hi"}, mr.Calls[0].Args)
	assert.Equal(t, "", mr.Calls[0].Dir)
	assert.Nil(t, mr.Calls[0].Env)
}

func TestRunHook_CommandFails(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("exit status 1"))

	err := runHook(mr, "false")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "false")
}

func TestRunHooks_RunsAllOnSuccess(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", nil)

	err := runHooks(mr, []string{"a", "b", "c"})
	require.NoError(t, err)

	require.Len(t, mr.Calls, 3)
	assert.Equal(t, []string{"-c", "a"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"-c", "b"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"-c", "c"}, mr.Calls[2].Args)
}

func TestRunHooks_StopsAtFirstFailure(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", errors.New("exit status 1"))

	err := runHooks(mr, []string{"a", "b", "c"})
	require.Error(t, err)

	// "c" must never run: the second command's failure stops the list.
	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"-c", "a"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"-c", "b"}, mr.Calls[1].Args)
}

func TestRunHooks_EmptyListIsNoOp(t *testing.T) {
	mr := exectest.NewMockRunner()

	err := runHooks(mr, nil)
	require.NoError(t, err)
	assert.Empty(t, mr.Calls)
}
