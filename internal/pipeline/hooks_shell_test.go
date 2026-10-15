package pipeline

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The hook variable contract (Spec 02 § Template variables): a value is inserted into the command
// text as-is, and the shell then parses that text. These tests run a real sh so the contract is
// pinned end to end, not just at the rendering step.
func TestHookVariables_AreInsertedUnquoted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the contract is stated for sh; cmd.exe has its own syntax (ADR-0054)")
	}
	const prev = "v1.0.0$(touch pwned)"

	tests := []struct {
		name       string
		template   string
		wantExpand bool
	}{
		{"unquoted: the shell expands the value", "true {{ .PreviousTag }}", true},
		{"double quotes still expand a command substitution", `true "{{ .PreviousTag }}"`, true},
		{"single quotes keep the value inert", "true '{{ .PreviousTag }}'", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			steps := []HookStep{{Run: tc.template}}
			_, err := runHookPointSteps(execadapter.New(false, false), steps, hookVars{PreviousTag: prev})
			require.NoError(t, err)

			_, statErr := os.Stat(filepath.Join(dir, "pwned"))
			if tc.wantExpand {
				assert.NoError(t, statErr, "the substitution ran")
			} else {
				assert.True(t, os.IsNotExist(statErr), "the substitution did not run")
			}
		})
	}
}

func TestHookVariables_DryRunShowsTheTextTheShellWouldParse(t *testing.T) {
	lines, err := dryRunHookLines([]HookStep{{Run: "notify {{ .Tag }}"}}, hookVars{Tag: "v1.2.3"})
	require.NoError(t, err)
	assert.Equal(t, []string{"[dry-run] would run: notify v1.2.3"}, lines)
}
