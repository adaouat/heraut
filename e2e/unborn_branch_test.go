package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/e2e/harness"
)

// A repository with no commit yet has nothing to version: every command that would resolve a
// version says so, instead of answering v0.1.0 or failing on a git error.
func TestUnbornBranch_SaysNoCommitsYet(t *testing.T) {
	for _, args := range [][]string{
		{"version", "next"},
		{"changelog", "--offline"},
		{"changelog", "--dry-run", "--offline"},
		{"release", "--dry-run", "--offline"},
	} {
		t.Run(args[0]+" "+args[1], func(t *testing.T) {
			bin := harness.Binary(t)
			repo := harness.NewRepo(t)
			repo.WriteConfig(flowCfg)

			res := repo.Run(bin, nil, args...)

			assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			assert.Contains(t, res.Stderr, "no commits yet")
			assert.NotContains(t, res.Stderr, "malformed object name")
			assert.NotContains(t, res.Stdout, "v0.1.0")
			assert.NoFileExists(t, repo.Dir+"/CHANGELOG.md")
		})
	}
}
