package e2e_test

import (
	"os"
	"path/filepath"
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

const calverChangelogCfg = `version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
changelog:
  output: CHANGELOG.md
rendering:
  templates:
    release.footer: "generated {{ date \"2006-01-02\" .Heraut.GeneratedAt }}"
`

func TestCalVer_ChangelogFollowsTheSimulatedClock(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	repo := harness.NewRepo(t)
	repo.WriteConfig(calverChangelogCfg)
	repo.Commit("feat: a")
	repo.Commit("fix: b")

	res := repo.Run(bin, []string{"HERAUT_TEST_NOW=2031-03-04T00:00:00Z"}, "changelog", "--offline")

	assert.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	changelog, err := os.ReadFile(filepath.Join(repo.Dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("reading CHANGELOG.md: %v", err)
	}
	assert.Contains(t, string(changelog), "## [2031.03.0] - ", "the version comes from the simulated clock")
	assert.Contains(t, string(changelog), "generated 2031-03-04", "a template sees the simulated clock")
	assert.Contains(t, string(changelog), "at 00:00 on 2031-03-04", "the default footer sees the simulated clock")
}
