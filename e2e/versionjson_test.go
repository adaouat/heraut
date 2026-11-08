package e2e_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// A failed command's envelope carries no data: scripts read ok and error.code (ADR-0080's stable
// surface). heraut check is the exception, since its data is the report of what failed.
func TestJSON_FailuresCarryNoData(t *testing.T) {
	bin := harness.Binary(t)
	failures := [][]string{
		{"version", "current", "--json"},
		{"version", "next", "--json", "--set-build-id", "5"},
		{"commit", "verify", "not conventional", "--json"},
		{"release", "--dry-run", "--json", "--set-version", "bad version"},
	}
	for _, args := range failures {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			repo := harness.NewRepo(t)
			repo.WriteConfig(semverCfg(""))
			repo.Commit("feat: a")

			res := repo.Run(bin, nil, args...)

			require.NotEqual(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			var env map[string]any
			require.NoError(t, json.Unmarshal([]byte(res.Stdout), &env), res.Stdout)
			assert.Equal(t, false, env["ok"])
			assert.NotContains(t, env, "data", res.Stdout)
			assert.Contains(t, env, "error")
		})
	}

	t.Run("check keeps its report", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteConfig("version: \"1\"\nversioning:\n  strategy: semvr\n")

		res := repo.Run(bin, nil, "check", "config", "--json")

		require.Equal(t, exitConfig, res.ExitCode, res.Stdout)
		assert.Contains(t, res.Stdout, `"data":{`)
		assert.Contains(t, res.Stdout, `"ok":false`)
	})
}
