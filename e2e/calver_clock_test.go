package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/e2e/harness"
)

const calverCfg = `version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  tag_prefix: ""
`

func TestCalVer_SimulatedClock(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	history := []step{commit("feat: a"), tag("2026.10.3"), commit("feat: b")}
	at := func(instant string) []string { return []string{"HERAUT_TEST_NOW=" + instant} }

	runScenarios(t, bin, at("2026-10-20T12:00:00Z"), []scenario{
		{name: "same month increments PATCH", config: calverCfg, history: history,
			args: versionNext, wantOut: "2026.10.4"},
	})
	runScenarios(t, bin, at("2026-11-01T00:00:00Z"), []scenario{
		{name: "next month resets PATCH to 0", config: calverCfg, history: history,
			args: versionNext, wantOut: "2026.11.0"},
	})
}

func TestCalVer_InvalidTestClockFailsLoudly(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	repo := harness.NewRepo(t)
	repo.WriteConfig(calverCfg)
	repo.Commit("feat: a")

	res := repo.Run(bin, []string{"HERAUT_TEST_NOW=yesterday"}, "version", "next")

	assert.NotEqual(t, exitOK, res.ExitCode)
	assert.Contains(t, res.Stdout+res.Stderr, "HERAUT_TEST_NOW")
}

func TestCalVer_ShippedBinaryIgnoresTestClock(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.WriteConfig(calverCfg)
	repo.Commit("feat: a")

	res := repo.Run(bin, []string{"HERAUT_TEST_NOW=2000-01-01T00:00:00Z"}, "version", "next")

	assert.Equal(t, exitOK, res.ExitCode)
	assert.False(t, strings.HasPrefix(strings.TrimSpace(res.Stdout), "2000."),
		"the shipped binary must use the real clock, got %q", res.Stdout)
}
