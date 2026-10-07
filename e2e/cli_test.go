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

func TestCLI_ErrorTextKeepsIdentifiersVerbatim(t *testing.T) {
	bin := harness.Binary(t)
	tests := []struct {
		name    string
		history []step
		args    []string
		want    string
	}{
		{"leading flag", []step{commit("feat: a")},
			[]string{"version", "next", "--pre-release", "rc", "--set-version", "1.0.0"},
			"--pre-release cannot be combined with --set-version"},
		{"leading tag", []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.2"), commit("fix: c")},
			[]string{"version", "next", "--pre-release", "beta"},
			"v1.4.0-beta.1 would sort below existing v1.4.0-rc.2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := harness.NewRepo(t)
			repo.WriteConfig(semverCfg(""))
			for _, s := range tc.history {
				if s.tag != "" {
					repo.Tag(s.tag)
				} else {
					repo.Commit(s.commit)
				}
			}

			res := repo.Run(bin, nil, tc.args...)

			assert.NotEqual(t, exitOK, res.ExitCode)
			assert.Contains(t, strings.Join(strings.Fields(res.Stdout+" "+res.Stderr), " "), tc.want)
		})
	}
}
