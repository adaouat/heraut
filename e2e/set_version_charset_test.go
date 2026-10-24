package e2e_test

import (
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

// ADR-0074 addendum: whatever the strategy, --set-version carries only [0-9A-Za-z._+/-].
func TestSetVersion_CharacterAllowlist(t *testing.T) {
	bin := harness.Binary(t)
	setVersion := func(value string) []string { return []string{"version", "next", "--set-version", value} }
	history := []step{commit("feat: a")}
	const wantRefusal = "only letters, digits"

	runScenarios(t, bin, nil, []scenario{
		{name: "a plain version is accepted", config: semverCfg(""),
			history: history, args: setVersion("1.2.3-rc.1"), wantOut: "v1.2.3-rc.1"},
		{name: "command substitution is refused", config: semverCfg(""),
			history: history, args: setVersion("1.2.3$(id)"), wantExit: exitConfig, wantText: []string{wantRefusal}},
		{name: "a command separator is refused", config: semverCfg(""),
			history: history, args: setVersion("1.2.3;id"), wantExit: exitConfig, wantText: []string{wantRefusal}},
		{name: "a glob character is refused", config: semverCfg(""),
			history: history, args: setVersion("1.2.*"), wantExit: exitConfig, wantText: []string{wantRefusal}},
	})
}
