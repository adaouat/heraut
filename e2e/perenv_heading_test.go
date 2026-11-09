package e2e_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// An env's tag format reads the same in the headings whether the env is written {env} or spelled
// out: the changelog and the notes show the version alone.
func TestPerEnv_HeadingsShowTheVersionWhateverTheTagFormat(t *testing.T) {
	for _, format := range []string{"{env}/{version}", "prod/{version}"} {
		t.Run(format, func(t *testing.T) {
			bin := harness.Binary(t)
			repo := flowRepo(t, fmt.Sprintf(`version: "1"
versioning:
  strategy: semver-per-env
changelog:
  output: CHANGELOG.md
environments:
  prod:
    tag_format: %q
    bump: auto
`, format))
			repo.Commit("feat: one")

			runOK(t, repo, bin, "changelog", "--tag", "--env", "prod", "--offline")
			require.Equal(t, "prod/0.1.0", repo.Git("tag", "-l"))

			changelog := repo.ReadFile("CHANGELOG.md")
			assert.Contains(t, changelog, "## [0.1.0]")
			assert.NotContains(t, changelog, "[prod/0.1.0]")
			assert.FileExists(t, filepath.Join(repo.Dir, "CHANGELOG.md"))

			notes := runOK(t, repo, bin, "notes", "prod/0.1.0", "--env", "prod", "--offline")
			assert.NotContains(t, notes.Stdout, "[prod/0.1.0]", "notes:\n%s", notes.Stdout)
		})
	}
}
