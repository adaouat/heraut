package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

const pushOptionsGit = `git:
  push_options:
    commit: ["ci.skip"]
    tag: ["ci.variable=DEPLOY=true", "-x"]
`

func TestPushOptions_ChangelogReachesTheServerPerPush(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg+pushOptionsGit)
	repo.RecordPushOptions()
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, [][]string{{"ci.skip"}, {"ci.variable=DEPLOY=true", "-x"}}, repo.PushOptions(),
		"the commit push and the tag push each carry their own list")
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"))
}

func TestPushOptions_ReleaseReachesTheServerPerPush(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", pushOptionsGit))
	repo.RecordPushOptions()

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Equal(t, [][]string{{"ci.skip"}, {"ci.variable=DEPLOY=true", "-x"}}, repo.PushOptions())
	assert.Equal(t, "v0.1.0", repo.FakeGitHub().Release()["tag_name"])
}

func TestPushOptions_AnEnvironmentClearsTheRootList(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, `version: "1"
versioning:
  strategy: semver-per-env
  initial_version: "0.1.0"
changelog:
  output: CHANGELOG.md
`+pushOptionsGit+`environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
    git:
      push_options:
        commit: []
`)
	repo.RecordPushOptions()
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--env", "dev", "--offline")

	assert.Equal(t, [][]string{{}, {"ci.variable=DEPLOY=true", "-x"}}, repo.PushOptions(),
		"dev pushes its commit with no options and inherits the tag list")
}

func TestPushOptions_DryRunShowsThem(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg+pushOptionsGit)
	repo.Commit("feat: one")

	res := runOK(t, repo, bin, "changelog", "--commit", "--tag", "--dry-run", "--offline")

	assert.Contains(t, res.Stdout, "--push-option=ci.skip")
	assert.Contains(t, res.Stdout, "--push-option=ci.variable=DEPLOY=true --push-option=-x")
	assert.Empty(t, repo.GitRemote("tag", "-l"), "a dry run pushes nothing")
}

func TestPushOptions_ReleaseJSONPlanListsThem(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", pushOptionsGit))

	res := repo.Run(bin, tokens, "release", "--dry-run", "--json", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout, `{"action":"commit_changelog","target":"CHANGELOG.md","push_options":["ci.skip"]}`)
	assert.Contains(t, res.Stdout, `{"action":"push_tag","target":"v0.1.0","push_options":["ci.variable=DEPLOY=true","-x"]}`)
}

func TestPushOptions_ARemoteWithoutSupportRefusesThePush(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg+pushOptionsGit)
	repo.Commit("feat: one")
	remoteHead := repo.GitRemote("rev-parse", "main")

	res := repo.Run(bin, nil, "changelog", "--commit", "--tag", "--offline")

	require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "does not support push options")
	assert.Equal(t, remoteHead, repo.GitRemote("rev-parse", "main"), "nothing reached the remote")
	assert.Empty(t, repo.Git("tag", "-l"), "the run stopped at the commit push, before the tag")
}

func TestPushOptions_AnEmptyValueIsAConfigError(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg+"git:\n  push_options:\n    commit: [\"\"]\n")
	repo.Commit("feat: one")
	head := repo.Git("rev-parse", "HEAD")

	res := repo.Run(bin, nil, "changelog", "--commit", "--tag", "--offline")

	require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout+res.Stderr, "git.push_options.commit[0]")
	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "nothing was committed")
}
