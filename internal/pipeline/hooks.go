package pipeline

import (
	"fmt"

	"github.com/adaouat/heraut/internal/port"
)

// runHook executes cmd via `sh -c` through r, which should be an interactive runner
// (stdin/stdout/stderr connected to the real terminal, forge's CmdRunner.Interactive
// mode — see gitHelper.interactiveOrRunner) so hook output streams live rather than
// being captured. dir is always the repository root ("" — RunDir's current-dir default).
func runHook(r port.Runner, cmd string) error {
	if _, _, err := r.RunDir("", nil, "sh", "-c", cmd); err != nil {
		return fmt.Errorf("hook %q: %w", cmd, err)
	}
	return nil
}

// runHooks executes cmds in order via runHook, stopping at the first failure.
func runHooks(r port.Runner, cmds []string) error {
	for _, cmd := range cmds {
		if err := runHook(r, cmd); err != nil {
			return err
		}
	}
	return nil
}
