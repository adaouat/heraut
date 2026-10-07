package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestCLI_VersionCurrentIgnoresPreReleases(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "latest final wins over a newer pre-release", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.0.0"), commit("feat: b"), tag("v1.1.0-rc.1")},
			args:    []string{"version", "current"}, wantOut: "v1.0.0"},
	})
}

func TestCLI_VersionFlagReportsTheInjectedVersion(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)

	res := repo.Run(bin, nil, "--version")

	assert.Equal(t, exitOK, res.ExitCode)
	assert.Contains(t, strings.Join(strings.Fields(res.Stdout), " "), "0.0.0-e2e")
}
