package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// GitLab CI checks out the pipeline's commit, not its branch. The changelog commit then goes to
// the CI job's branch by name, since `git push origin HEAD` has no branch to update.
func TestDetachedHEAD_PushesTheCommitToTheCIBranch(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	repo.Detach()

	res := repo.Run(bin, []string{"CI_COMMIT_BRANCH=main"}, "changelog", "--commit", "--tag", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Equal(t, "chore(release): 0.1.0", repo.GitRemote("log", "-1", "--format=%s", "main"))
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"))
	assert.Equal(t, repo.GitRemote("rev-parse", "main"), repo.GitRemote("rev-parse", "v0.1.0^{commit}"),
		"the tag points at the pushed release commit")
}

// Without a branch to push to, the run stops before it writes, commits or tags anything.
func TestDetachedHEAD_WithoutABranchStopsBeforeWriting(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want string
	}{
		{"no CI branch variable", nil, "no CI branch variable names the branch"},
		{"CI branch not fetched", []string{"CI_COMMIT_BRANCH=feature"}, "origin/feature was not fetched"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bin := harness.Binary(t)
			repo := flowRepo(t, flowCfg)
			repo.Commit("feat: one")
			repo.Detach()
			remoteMain := repo.GitRemote("rev-parse", "main")

			res := repo.Run(bin, tc.env, "changelog", "--commit", "--tag", "--offline")

			assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			assert.Contains(t, res.Stderr, tc.want)
			assert.Contains(t, res.Stderr, "git checkout -B")
			assert.Equal(t, "feat: one", repo.Git("log", "-1", "--format=%s"), "nothing committed")
			assert.Empty(t, repo.Git("tag", "-l"), "nothing tagged")
			assert.NoFileExists(t, repo.Dir+"/CHANGELOG.md")
			assert.Equal(t, remoteMain, repo.GitRemote("rev-parse", "main"), "nothing pushed")
		})
	}
}

// An environment linked to a branch is released from a detached CI checkout of that branch, and
// --env auto finds it the same way.
func TestDetachedHEAD_PerEnvBranchGuardReadsTheCIBranch(t *testing.T) {
	const cfg = `version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  prod:
    branch: main
    bump: auto
changelog:
  output: CHANGELOG.md
`
	bin := harness.Binary(t)
	for _, env := range []string{"prod", "auto"} {
		t.Run("--env "+env, func(t *testing.T) {
			repo := flowRepo(t, cfg)
			repo.Commit("feat: one")
			repo.Detach()

			res := repo.Run(bin, []string{"GITLAB_CI=true", "CI_COMMIT_BRANCH=main"}, "changelog", "--tag", "--env", env, "--offline")

			require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			assert.Equal(t, "prod/0.1.0", repo.GitRemote("tag", "-l"))
		})
	}
	t.Run("another CI branch is refused", func(t *testing.T) {
		repo := flowRepo(t, cfg)
		repo.Commit("feat: one")
		repo.Detach()

		res := repo.Run(bin, []string{"GITLAB_CI=true", "CI_COMMIT_BRANCH=develop"}, "changelog", "--tag", "--env", "prod", "--offline")

		assert.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, res.Stderr, `the current branch is "develop"`)
		assert.Empty(t, repo.GitRemote("tag", "-l"))
	})
}
