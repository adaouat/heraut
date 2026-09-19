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

func TestRenderHookCmd_SubstitutesVars(t *testing.T) {
	vars := hookVars{Version: "1.2.3", Tag: "v1.2.3", PreviousTag: "v1.2.2", Platform: "github", Env: "staging"}

	rendered, err := renderHookCmd(
		"echo {{ .Version }} {{ .Tag }} {{ .PreviousTag }} {{ .Platform }} {{ .Env }}", vars)
	require.NoError(t, err)
	assert.Equal(t, "echo 1.2.3 v1.2.3 v1.2.2 github staging", rendered)
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

func TestRenderHookStep_RendersRunAndStage(t *testing.T) {
	vars := hookVars{Version: "1.2.3"}
	step := HookStep{Run: "echo {{ .Version }}", Stage: []string{"dist/{{ .Version }}.tgz", "CHANGELOG.md"}}

	rendered, err := renderHookStep(step, vars)
	require.NoError(t, err)
	assert.Equal(t, renderedHookStep{
		Run:   "echo 1.2.3",
		Stage: []string{"dist/1.2.3.tgz", "CHANGELOG.md"},
	}, rendered)
}

func TestRenderHookStep_RunRenderErrorPropagates(t *testing.T) {
	_, err := renderHookStep(HookStep{Run: "echo {{ .Bad"}, hookVars{})
	require.Error(t, err)
}

func TestRenderHookStep_StageRenderErrorPropagates(t *testing.T) {
	_, err := renderHookStep(HookStep{Run: "echo ok", Stage: []string{"{{ .Bad"}}, hookVars{})
	require.Error(t, err)
}

func TestRenderHookSteps_RendersEachInOrder(t *testing.T) {
	vars := hookVars{Version: "1.2.3"}
	steps := []HookStep{
		{Run: "echo {{ .Version }}"},
		{Run: "echo done", Stage: []string{"out.txt"}},
	}

	rendered, err := renderHookSteps(steps, vars)
	require.NoError(t, err)
	assert.Equal(t, []renderedHookStep{
		{Run: "echo 1.2.3", Stage: []string{}},
		{Run: "echo done", Stage: []string{"out.txt"}},
	}, rendered)
}

func TestRenderHookSteps_StopsAtFirstError(t *testing.T) {
	steps := []HookStep{
		{Run: "echo ok"},
		{Run: "echo {{ .Bad"},
		{Run: "echo never reached"},
	}

	_, err := renderHookSteps(steps, hookVars{})
	require.Error(t, err)
}

func TestShouldRunHooks(t *testing.T) {
	tests := []struct {
		name    string
		dryRun  bool
		noHooks bool
		steps   []HookStep
		want    bool
	}{
		{"normal run with hooks configured", false, false, []HookStep{{Run: "echo hi"}}, true},
		{"dry-run skips even when configured", true, false, []HookStep{{Run: "echo hi"}}, false},
		{"--no-hooks skips even when configured", false, true, []HookStep{{Run: "echo hi"}}, false},
		{"nothing configured", false, false, nil, false},
		{"dry-run and no-hooks and nothing configured", true, true, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, shouldRunHooks(tc.dryRun, tc.noHooks, tc.steps))
		})
	}
}

func TestRunHookPointSteps_ReturnsRenderedStepsOnSuccess(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", nil)

	steps := []HookStep{
		{Run: "echo {{ .Version }}", Stage: []string{"dist/{{ .Version }}.tgz"}},
		{Run: "echo done"},
	}
	rendered, err := runHookPointSteps(mr, steps, hookVars{Version: "1.2.3"})
	require.NoError(t, err)

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"-c", "echo 1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"-c", "echo done"}, mr.Calls[1].Args)

	assert.Equal(t, []renderedHookStep{
		{Run: "echo 1.2.3", Stage: []string{"dist/1.2.3.tgz"}},
		{Run: "echo done", Stage: []string{}},
	}, rendered)
}

func TestRunHookPointSteps_StopsAtFirstExecutionFailure(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", errors.New("exit status 1"))

	steps := []HookStep{
		{Run: "echo a"},
		{Run: "false"},
		{Run: "echo never reached"},
	}
	rendered, err := runHookPointSteps(mr, steps, hookVars{})
	require.Error(t, err)
	assert.Nil(t, rendered)

	// "echo never reached" must never run: the second command's failure stops the list.
	require.Len(t, mr.Calls, 2)
}

func TestRunHookPointSteps_RenderErrorNeverReachesRunner(t *testing.T) {
	mr := exectest.NewMockRunner()

	_, err := runHookPointSteps(mr, []HookStep{{Run: "echo {{ .Bad"}}, hookVars{})
	require.Error(t, err)
	assert.Empty(t, mr.Calls)
}

func TestStagePatterns_FlattensInOrder(t *testing.T) {
	steps := []renderedHookStep{
		{Run: "echo a", Stage: []string{"a.txt", "b.txt"}},
		{Run: "echo b"},
		{Run: "echo c", Stage: []string{"c.txt"}},
	}
	assert.Equal(t, []string{"a.txt", "b.txt", "c.txt"}, stagePatterns(steps))
}

func TestStagePatterns_NoStageEntriesReturnsNil(t *testing.T) {
	steps := []renderedHookStep{{Run: "echo a"}, {Run: "echo b"}}
	assert.Nil(t, stagePatterns(steps))
}

func TestDryRunHookLines_EmptyReturnsNil(t *testing.T) {
	lines, err := dryRunHookLines(nil, hookVars{})
	require.NoError(t, err)
	assert.Nil(t, lines)
}

func TestDryRunHookLines_RendersEachCommand(t *testing.T) {
	vars := hookVars{Version: "1.2.3"}
	steps := []HookStep{{Run: "echo {{ .Version }}"}, {Run: "go build ./..."}}
	lines, err := dryRunHookLines(steps, vars)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"[dry-run] would run: echo 1.2.3",
		"[dry-run] would run: go build ./...",
	}, lines)
}

// TestDryRunHookLines_StageProducesWouldStageLines proves a stage-bearing step's dry-run output
// includes both the "would run:" and "would stage:" lines (Design §5, 2026-09-17).
func TestDryRunHookLines_StageProducesWouldStageLines(t *testing.T) {
	vars := hookVars{Version: "1.2.3"}
	steps := []HookStep{
		{Run: "echo {{ .Version }}", Stage: []string{"dist/{{ .Version }}.tgz", "CHANGELOG.md"}},
	}
	lines, err := dryRunHookLines(steps, vars)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"[dry-run] would run: echo 1.2.3",
		"[dry-run] would stage: dist/1.2.3.tgz",
		"[dry-run] would stage: CHANGELOG.md",
	}, lines)
}

func TestDryRunHookLines_RenderErrorPropagates(t *testing.T) {
	_, err := dryRunHookLines([]HookStep{{Run: "echo {{ .Bad"}}, hookVars{})
	require.Error(t, err)
}

func TestHookShellInvocation(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		cmd      string
		wantName string
		wantArgs []string
	}{
		{"linux uses sh -c", "linux", "echo hi", "sh", []string{"-c", "echo hi"}},
		{"darwin uses sh -c", "darwin", "echo hi", "sh", []string{"-c", "echo hi"}},
		{"windows uses cmd /D /C", "windows", "echo hi", "cmd", []string{"/D", "/C", "echo hi"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, args := hookShellInvocation(tc.goos, tc.cmd)
			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantArgs, args)
		})
	}
}
