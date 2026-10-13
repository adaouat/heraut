package e2e_test

import (
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestInteractiveCommandsFailFastWithoutATerminal(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "init names --defaults", history: []step{commit("chore: x")},
			args:     []string{"init"},
			wantExit: exitUsage, wantText: []string{"interactive terminal", "--defaults"}},
		{name: "init with an existing config does not prompt", config: semverCfg(""),
			args:     []string{"init"},
			wantExit: exitUsage, wantText: []string{"interactive terminal", "--defaults", "--overwrite"}},
		{name: "commit create names the non-interactive route", config: semverCfg(""), history: []step{commit("chore: x")},
			args:     []string{"commit", "create"},
			wantExit: exitUsage, wantText: []string{"interactive terminal", "heraut commit verify"}},
	})
}
