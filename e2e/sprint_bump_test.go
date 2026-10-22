package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// hookWithSprintLine puts a "sprint:" line in a hook script that sits before the versioning block.
const hookWithSprintLine = `hooks:
  post_bump:
    - run: |
        cat <<EOF
          sprint: 9
        EOF
`

// T405: the bump edits the top-level versioning block and nothing else in the file.
func TestSprintBump_StaysInsideTheVersioningBlock(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")

	bump := func(t *testing.T, config string) (harness.Result, string) {
		t.Helper()
		repo := harness.NewRepo(t)
		repo.WriteConfig(config)
		repo.Commit("feat: a")
		res := repo.Run(bin, at("2026-10-07T00:00:00Z"), "version", "sprint", "bump")
		return res, repo.ReadFile(".heraut.yml")
	}

	tests := []struct {
		name, before, after string
	}{
		{"trailing comments on the block and the key are kept",
			"version: \"1\"\nversioning: # the scheme\n  strategy: calver\n  format: \"YYYY.SPRINT.PATCH\"\n  sprint: 5 # current\n",
			"version: \"1\"\nversioning: # the scheme\n  strategy: calver\n  format: \"YYYY.SPRINT.PATCH\"\n  sprint: 6 # current\n"},
		{"a missing key is inserted with the block's indentation",
			"version: \"1\"\nversioning:\n    strategy: calver\n    format: \"YYYY.SPRINT.PATCH\"\n",
			"version: \"1\"\nversioning:\n    sprint: 1\n    strategy: calver\n    format: \"YYYY.SPRINT.PATCH\"\n"},
		{"a sprint line in a hook script elsewhere is untouched",
			hookWithSprintLine + "versioning:\n  strategy: calver\n  format: \"YYYY.SPRINT.PATCH\"\n  sprint: 5\n",
			hookWithSprintLine + "versioning:\n  strategy: calver\n  format: \"YYYY.SPRINT.PATCH\"\n  sprint: 6\n"},
		{"a missing key is inserted in the block, not next to a hook's sprint line",
			hookWithSprintLine + "versioning:\n  strategy: calver\n  format: \"YYYY.SPRINT.PATCH\"\n",
			hookWithSprintLine + "versioning:\n  sprint: 1\n  strategy: calver\n  format: \"YYYY.SPRINT.PATCH\"\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, got := bump(t, tc.before)
			require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			assert.Equal(t, tc.after, got)
		})
	}

	t.Run("an inline versioning mapping is refused and the file is unchanged", func(t *testing.T) {
		t.Parallel()
		before := "version: \"1\"\nversioning: {strategy: calver, format: \"YYYY.SPRINT.PATCH\", sprint: 5}\n"
		res, got := bump(t, before)
		require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "block")
		assert.Equal(t, before, got)
	})
}
