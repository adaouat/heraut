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
