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

func TestRenderHookCmd_SubstitutesVars(t *testing.T) {
	vars := hookVars{Version: "1.2.3", Tag: "v1.2.3", PreviousTag: "v1.2.2", Platform: "github"}

	rendered, err := renderHookCmd(
		"echo {{ .Version }} {{ .Tag }} {{ .PreviousTag }} {{ .Platform }}", vars)
	require.NoError(t, err)
	assert.Equal(t, "echo 1.2.3 v1.2.3 v1.2.2 github", rendered)
}

func TestRenderHookCmd_NoTemplateSyntaxPassesThrough(t *testing.T) {
	rendered, err := renderHookCmd("go build ./...", hookVars{})
	require.NoError(t, err)
	assert.Equal(t, "go build ./...", rendered)
}

func TestRenderHookCmd_InvalidTemplateReturnsError(t *testing.T) {
	_, err := renderHookCmd("echo {{ .Version", hookVars{})
	require.Error(t, err)
}

func TestRenderHookCmd_UnknownFieldReturnsError(t *testing.T) {
	_, err := renderHookCmd("echo {{ .NotAField }}", hookVars{})
	require.Error(t, err)
}

func TestRenderHookCmds_RendersEachInOrder(t *testing.T) {
	vars := hookVars{Version: "1.2.3"}
	rendered, err := renderHookCmds([]string{"echo {{ .Version }}", "echo done"}, vars)
	require.NoError(t, err)
	assert.Equal(t, []string{"echo 1.2.3", "echo done"}, rendered)
}

func TestRenderHookCmds_StopsAtFirstRenderError(t *testing.T) {
	_, err := renderHookCmds([]string{"echo ok", "echo {{ .Bad"}, hookVars{})
	require.Error(t, err)
}

func TestShouldRunHooks(t *testing.T) {
	tests := []struct {
		name    string
		dryRun  bool
		noHooks bool
		cmds    []string
		want    bool
	}{
		{"normal run with hooks configured", false, false, []string{"echo hi"}, true},
		{"dry-run skips even when configured", true, false, []string{"echo hi"}, false},
		{"--no-hooks skips even when configured", false, true, []string{"echo hi"}, false},
		{"nothing configured", false, false, nil, false},
		{"dry-run and no-hooks and nothing configured", true, true, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, shouldRunHooks(tc.dryRun, tc.noHooks, tc.cmds))
		})
	}
}

func TestRunHookPoint_RendersAndExecutes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	err := runHookPoint(mr, []string{"echo {{ .Version }}"}, hookVars{Version: "1.2.3"})
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"-c", "echo 1.2.3"}, mr.Calls[0].Args)
}

func TestRunHookPoint_RenderErrorNeverReachesRunner(t *testing.T) {
	mr := exectest.NewMockRunner()

	err := runHookPoint(mr, []string{"echo {{ .Bad"}, hookVars{})
	require.Error(t, err)
	assert.Empty(t, mr.Calls)
}
