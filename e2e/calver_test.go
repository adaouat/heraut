package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

func calverFmt(format, prefix, extra string) string {
	return `version: "1"
versioning:
  strategy: calver
  format: "` + format + `"
  tag_prefix: "` + prefix + `"
` + extra
}

func at(instant string) []string { return []string{"HERAUT_TEST_NOW=" + instant} }

func TestCalVer_PeriodBoundaries(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	runScenarios(t, bin, nil, []scenario{
		{name: "monthly: same month increments", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.05.2")}, args: versionNext,
			env: at("2026-05-20T10:00:00Z"), wantOut: "2026.05.3"},
		{name: "monthly: last minute of the month increments", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.05.2")}, args: versionNext,
			env: at("2026-05-31T23:59:00Z"), wantOut: "2026.05.3"},
		{name: "monthly: next month resets PATCH", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.05.2")}, args: versionNext,
			env: at("2026-06-01T00:00:00Z"), wantOut: "2026.06.0"},
		{name: "monthly: year rollover resets PATCH", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.12.4")}, args: versionNext,
			env: at("2027-01-01T00:00:00Z"), wantOut: "2027.01.0"},
		{name: "monthly: same month of another year resets PATCH", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.01.2")}, args: versionNext,
			env: at("2027-01-15T00:00:00Z"), wantOut: "2027.01.0"},
		{name: "monthly: untagged repo starts at PATCH 0", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: []step{commit("feat: a")}, args: versionNext,
			env: at("2026-10-07T00:00:00Z"), wantOut: "2026.10.0"},
		{name: "monthly: a tag prefix round-trips", config: calverFmt("YYYY.MM.PATCH", "v", ""),
			history: []step{commit("feat: a"), tag("v2026.05.2")}, args: versionNext,
			env: at("2026-05-20T10:00:00Z"), wantOut: "v2026.05.3"},
		{name: "daily: last minute of the day increments", config: calverFmt("YYYY.MM.DD.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.05.07.1")}, args: versionNext,
			env: at("2026-05-07T23:59:00Z"), wantOut: "2026.05.07.2"},
		{name: "daily: first minute of the next day resets", config: calverFmt("YYYY.MM.DD.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.05.07.1")}, args: versionNext,
			env: at("2026-05-08T00:01:00Z"), wantOut: "2026.05.08.0"},
		{name: "weekly: same ISO week increments", config: calverFmt("YYYY.WW.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.19.1")}, args: versionNext,
			env: at("2026-05-08T10:00:00Z"), wantOut: "2026.19.2"},
		{name: "weekly: Sunday night still belongs to the week", config: calverFmt("YYYY.WW.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.19.1")}, args: versionNext,
			env: at("2026-05-10T23:59:00Z"), wantOut: "2026.19.2"},
		{name: "weekly: Monday 00:00 starts a new ISO week", config: calverFmt("YYYY.WW.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.19.1")}, args: versionNext,
			env: at("2026-05-11T00:00:00Z"), wantOut: "2026.20.0"},
		{name: "weekly: Monday starts a new ISO week", config: calverFmt("YYYY.WW.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.19.1")}, args: versionNext,
			env: at("2026-05-11T10:00:00Z"), wantOut: "2026.20.0"},
		{name: "weekly: week 53 of a long year keeps counting", config: calverFmt("YYYY.WW.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.53.1")}, args: versionNext,
			env: at("2026-12-31T10:00:00Z"), wantOut: "2026.53.2"},
		{name: "quarterly: last minute of Q2 increments", config: calverFmt("YYYY.QQ.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.2.1")}, args: versionNext,
			env: at("2026-06-30T23:59:00Z"), wantOut: "2026.2.2"},
		{name: "quarterly: first minute of Q3 resets", config: calverFmt("YYYY.QQ.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.2.1")}, args: versionNext,
			env: at("2026-07-01T00:00:00Z"), wantOut: "2026.3.0"},
		{name: "semester: last minute of S1 increments", config: calverFmt("YYYY.SS.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.1.1")}, args: versionNext,
			env: at("2026-06-30T23:59:00Z"), wantOut: "2026.1.2"},
		{name: "semester: first minute of S2 resets", config: calverFmt("YYYY.SS.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.1.1")}, args: versionNext,
			env: at("2026-07-01T00:00:00Z"), wantOut: "2026.2.0"},
		{name: "yearly: last minute of the year increments", config: calverFmt("YYYY.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.4")}, args: versionNext,
			env: at("2026-12-31T23:59:00Z"), wantOut: "2026.5"},
		{name: "yearly: first minute of the next year resets", config: calverFmt("YYYY.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.4")}, args: versionNext,
			env: at("2027-01-01T00:00:00Z"), wantOut: "2027.0"},
		{name: "a breaking conventional commit does not change the result", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: []step{commit("feat: a"), tag("2026.10.0"), commit("feat!: x\n\nBREAKING CHANGE: y")},
			args:    versionNext, env: at("2026-10-20T00:00:00Z"), wantOut: "2026.10.1"},
		{name: "commit messages are ignored", config: calverFmt("YYYY.MM.PATCH", "", ""),
			history: []step{commit("update stuff"), tag("2026.10.0"), commit("also not conventional")},
			args:    versionNext, env: at("2026-10-20T00:00:00Z"), wantOut: "2026.10.1"},
	})
}

func TestCalVer_Sprint(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	sprintCfg := func(n string) string {
		return calverFmt("YYYY.SPRINT.PATCH", "", "  sprint: "+n+"\n")
	}
	runScenarios(t, bin, nil, []scenario{
		{name: "same sprint increments PATCH", config: sprintCfg("5"),
			history: []step{commit("feat: a"), tag("2026.5.1")}, args: versionNext,
			env: at("2026-10-07T00:00:00Z"), wantOut: "2026.5.2"},
		{name: "a bumped sprint resets PATCH", config: sprintCfg("6"),
			history: []step{commit("feat: a"), tag("2026.5.1")}, args: versionNext,
			env: at("2026-10-07T00:00:00Z"), wantOut: "2026.6.0"},
	})

	t.Run("version sprint bump rewrites the config and the next version follows", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteConfig(sprintCfg("5"))
		repo.Commit("feat: a")
		repo.Tag("2026.5.1")
		env := at("2026-10-07T00:00:00Z")

		dry := repo.Run(bin, env, "version", "sprint", "bump", "--dry-run")
		require.Equal(t, exitOK, dry.ExitCode, dry.Stderr)
		assert.Contains(t, dry.Stdout, "would bump sprint 5 -> 6")
		assert.Contains(t, repo.ReadFile(".heraut.yml"), "sprint: 5", "--dry-run must not rewrite the file")

		bump := repo.Run(bin, env, "version", "sprint", "bump")
		require.Equal(t, exitOK, bump.ExitCode, bump.Stderr)
		assert.Contains(t, bump.Stdout, "sprint bumped to 6")
		assert.Contains(t, repo.ReadFile(".heraut.yml"), "sprint: 6")

		next := repo.Run(bin, env, "version", "next")
		assert.Equal(t, "2026.6.0", strings.TrimSpace(next.Stdout))
	})
}

const calverPerEnvCfg = `version: "1"
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
  prod:
    tag_format: "prod/{version}"
    bump: promote
    source: dev
`

func TestCalVer_PerEnv(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	next := func(env string) []string { return []string{"version", "next", "--env", env} }
	runScenarios(t, bin, nil, []scenario{
		{name: "first dev tag", config: calverPerEnvCfg,
			history: []step{commit("feat: a")}, args: next("dev"),
			env: at("2026-10-07T00:00:00Z"), wantOut: "dev/2026.10.0"},
		{name: "dev increments within the month", config: calverPerEnvCfg,
			history: []step{commit("feat: a"), tag("dev/2026.10.0"), commit("feat: b")}, args: next("dev"),
			env: at("2026-10-20T00:00:00Z"), wantOut: "dev/2026.10.1"},
		{name: "dev resets in the next month", config: calverPerEnvCfg,
			history: []step{commit("feat: a"), tag("dev/2026.10.0"), commit("feat: b")}, args: next("dev"),
			env: at("2026-11-02T00:00:00Z"), wantOut: "dev/2026.11.0"},
		{name: "prod promotes the dev version", config: calverPerEnvCfg,
			history: []step{commit("feat: a"), tag("dev/2026.10.1")}, args: next("prod"),
			env: at("2026-10-07T00:00:00Z"), wantOut: "prod/2026.10.1"},
		{name: "promoting onto an existing prod tag is refused", config: calverPerEnvCfg,
			history: []step{commit("feat: a"), tag("dev/2026.10.1"), tag("prod/2026.10.1")}, args: next("prod"),
			env: at("2026-10-07T00:00:00Z"), wantExit: exitPromotion, wantText: []string{"e001", "tag already exists"}},
		{name: "version current reads the env's latest tag", config: calverPerEnvCfg,
			history: []step{commit("feat: a"), tag("dev/2026.10.0"), tag("dev/2026.10.1")},
			args:    []string{"version", "current", "--env", "dev"},
			env:     at("2026-10-07T00:00:00Z"), wantOut: "dev/2026.10.1"},
	})
}
