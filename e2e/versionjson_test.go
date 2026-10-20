package e2e_test

import (
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestVersionJSON(t *testing.T) {
	bin := harness.Binary(t)
	released := []step{commit("feat: a"), tag("v1.0.0"), commit("feat: b")}
	runScenarios(t, bin, nil, []scenario{
		{name: "next", config: semverCfg(""), history: released,
			args:    []string{"version", "next", "--json"},
			wantOut: `{"ok":true,"data":{"tag":"v1.1.0","version":"1.1.0","current_tag":"v1.0.0","bump":"minor"},"warnings":[]}`},
		{name: "next without a previous tag has neither current_tag nor bump", config: semverCfg(""),
			history: []step{commit("feat: a")},
			args:    []string{"version", "next", "--json"},
			wantOut: `{"ok":true,"data":{"tag":"v0.1.0","version":"0.1.0"},"warnings":[]}`},
		{name: "current", config: semverCfg(""), history: released,
			args:    []string{"version", "current", "--json"},
			wantOut: `{"ok":true,"data":{"tag":"v1.0.0","version":"1.0.0"},"warnings":[]}`},
		{name: "current with no tag is a runtime error", config: semverCfg(""),
			history:  []step{commit("feat: a")},
			args:     []string{"version", "current", "--json"},
			wantExit: exitRuntime, wantText: []string{`"ok":false`, `"code":"runtime"`}},
		{name: "next on a per-env strategy without --env is a config error", config: perEnvCfg,
			history:  released,
			args:     []string{"version", "next", "--json"},
			wantExit: exitConfig, wantText: []string{`"ok":false`, `"code":"config"`, "--env is required"}},
	})
}

// RULE-052: rendering.excludes hide commits from the changelog but not from the bump, so a
// dependency-only range still releases. The warning says the changelog section will be empty.
func TestVersionNext_WarnsWhenEveryReleasableCommitIsHiddenFromTheChangelog(t *testing.T) {
	bin := harness.Binary(t)
	withChangelog := semverCfg("") + "changelog:\n  output: CHANGELOG.md\n"
	base := []step{commit("feat: a"), tag("v1.0.0")}
	runScenarios(t, bin, nil, []scenario{
		{name: "only dependency noise: still a patch bump, with a warning", config: withChangelog,
			history: append(append([]step{}, base...), commit("chore(deps): bump x")),
			args:    []string{"version", "next", "--json"},
			wantOut: `{"ok":true,"data":{"tag":"v1.0.1","version":"1.0.1","current_tag":"v1.0.0","bump":"patch"},"warnings":["every releasable commit is hidden from the changelog by rendering.excludes: this release will have an empty changelog section\n  - chore(deps): bump x"]}`},
		{name: "a real change beside the noise: no warning", config: withChangelog,
			history: append(append([]step{}, base...), commit("chore(deps): bump x"), commit("fix: y")),
			args:    []string{"version", "next", "--json"},
			wantOut: `{"ok":true,"data":{"tag":"v1.0.1","version":"1.0.1","current_tag":"v1.0.0","bump":"patch"},"warnings":[]}`},
		{name: "no changelog configured: no warning", config: semverCfg(""),
			history: append(append([]step{}, base...), commit("chore(deps): bump x")),
			args:    []string{"version", "next", "--json"},
			wantOut: `{"ok":true,"data":{"tag":"v1.0.1","version":"1.0.1","current_tag":"v1.0.0","bump":"patch"},"warnings":[]}`},
	})
}
