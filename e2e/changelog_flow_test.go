package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

const flowCfg = `version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
changelog:
  output: CHANGELOG.md
`

// flowRepo is a repository with one commit, the given config (untracked) and a bare origin that
// already has main.
func flowRepo(t *testing.T, cfg string) *harness.Repo {
	t.Helper()
	repo := harness.NewRepo(t)
	repo.WriteConfig(cfg)
	repo.Commit("chore: init")
	repo.AddRemote()
	return repo
}

func runOK(t *testing.T, repo *harness.Repo, bin string, args ...string) harness.Result {
	t.Helper()
	res := repo.Run(bin, nil, args...)
	require.Equal(t, exitOK, res.ExitCode, "heraut %v\nstdout:\n%s\nstderr:\n%s", args, res.Stdout, res.Stderr)
	return res
}

func TestChangelogFlow_CommitTagAndPush(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, "chore(release): 0.1.0", repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, "tag", repo.Git("cat-file", "-t", "v0.1.0"), "tags are annotated by default")
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.Git("rev-list", "-n", "1", "v0.1.0"),
		"the tag points at the changelog commit")
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag reached the remote")
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.GitRemote("rev-parse", "main"), "the branch reached the remote")
	assert.Equal(t, []string{"0.1.0: One, Init"}, changelogOutline(repo.ReadFile("CHANGELOG.md")))
}

func TestChangelogFlow_NoPushKeepsTheRemoteUntouched(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	remoteHead := repo.GitRemote("rev-parse", "main")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--no-push", "--offline")

	assert.Equal(t, "chore(release): 0.1.0", repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, "v0.1.0", repo.Git("tag", "-l"))
	assert.Equal(t, remoteHead, repo.GitRemote("rev-parse", "main"), "the remote branch is untouched")
	assert.Empty(t, repo.GitRemote("tag", "-l"), "the remote has no tag")
}

func TestChangelogFlow_WithoutFlagsOnlyWritesTheFile(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	head := repo.Git("rev-parse", "HEAD")

	runOK(t, repo, bin, "changelog", "--offline")

	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "no commit")
	assert.Empty(t, repo.Git("tag", "-l"), "no tag")
	assert.Contains(t, repo.Git("status", "--porcelain"), "?? CHANGELOG.md")
	assert.Equal(t, []string{"0.1.0: One, Init"}, changelogOutline(repo.ReadFile("CHANGELOG.md")))
}

func TestChangelogFlow_TagImpliesCommitAndPush(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--tag", "--offline")

	assert.Equal(t, "chore(release): 0.1.0", repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"))
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.GitRemote("rev-parse", "main"), "the branch was pushed too")
}

func TestChangelogFlow_CommitWithoutTag(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--offline", "--set-version", "0.1.0")

	assert.Empty(t, repo.Git("tag", "-l"))
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.GitRemote("rev-parse", "main"), "the commit was pushed")
}

func TestChangelogFlow_AnIdenticalRerunSkipsTheCommit(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	runOK(t, repo, bin, "changelog", "--commit", "--offline", "--set-version", "0.1.0")
	head := repo.Git("rev-parse", "HEAD")

	res := runOK(t, repo, bin, "changelog", "--commit", "--offline", "--set-version", "0.1.0")

	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "nothing new is committed")
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "nothing to commit")
}

func TestChangelogFlow_IncrementalSpliceKeepsHandEditsRegenerateDropsThem(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	original := repo.ReadFile("CHANGELOG.md")
	edited := strings.Replace(original, "- One - ", "- One (edited by hand) - ", 1)
	require.NotEqual(t, original, edited, "the hand edit must change the file")
	repo.WriteFile("CHANGELOG.md", edited)
	repo.Git("commit", "-q", "-am", "docs: hand edit")
	repo.Commit("fix: two")
	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, []string{"0.1.1: Two, Hand edit", "0.1.0: One (edited by hand), Init"},
		changelogOutline(repo.ReadFile("CHANGELOG.md")), "an incremental release splices only the new section")

	repo.Commit("fix: three")
	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline", "--regenerate")

	assert.Equal(t, []string{"0.1.2: Three", "0.1.1: Two, Hand edit", "0.1.0: One, Init"},
		changelogOutline(repo.ReadFile("CHANGELOG.md")), "--regenerate rebuilds every section")
}

func TestChangelogFlow_LightweightTags(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, `version: "1"
versioning:
  strategy: semver
  tag_type: lightweight
changelog:
  output: CHANGELOG.md
`)
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, "commit", repo.Git("cat-file", "-t", "v0.1.0"), "a lightweight tag is a bare ref")
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"))
}

func TestChangelogFlow_PreReleaseTagsWithoutTouchingTheChangelog(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	head := repo.Git("rev-parse", "HEAD")

	res := runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline", "--set-version", "1.0.0-rc.1")

	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "pre-release: changelog.md not updated")
	assert.NoFileExists(t, filepath.Join(repo.Dir, "CHANGELOG.md"))
	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "no changelog commit")
	assert.Equal(t, "v1.0.0-rc.1", repo.GitRemote("tag", "-l"), "the tag is still pushed")
}

func TestChangelogFlow_DisabledChangelogStillTags(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, `version: "1"
versioning:
  strategy: semver-per-env
changelog:
  output: CHANGELOG.md
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
    disable_changelog: true
`)
	repo.Commit("feat: one")
	head := repo.Git("rev-parse", "HEAD")

	res := runOK(t, repo, bin, "changelog", "--tag", "--env", "dev", "--offline")

	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "changelog disabled")
	assert.NoFileExists(t, filepath.Join(repo.Dir, "CHANGELOG.md"))
	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"))
	assert.Equal(t, "dev/0.1.0", repo.GitRemote("tag", "-l"))
}

func TestChangelogFlow_AFailingPushIsARuntimeError(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	repo.RemoveRemote()

	res := repo.Run(bin, nil, "changelog", "--commit", "--tag", "--offline")

	require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "does not appear to be a git repository")
	assert.Empty(t, repo.Git("tag", "-l"), "the tag is only created after the commit was pushed")
}

// TestChangelogFlow_ARetryAfterAFailedPushPushesTheReleaseCommit covers T360: the first run commits
// locally and fails at the push; once the remote is back the identical changelog stages nothing, yet
// the retry must still push the release commit so the remote tag points at a commit on main.
func TestChangelogFlow_ARetryAfterAFailedPushPushesTheReleaseCommit(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	behind := repo.GitRemote("rev-parse", "main")
	repo.RemoveRemote()
	failed := repo.Run(bin, nil, "changelog", "--commit", "--tag", "--offline")
	require.Equal(t, exitRuntime, failed.ExitCode, "stdout:\n%s\nstderr:\n%s", failed.Stdout, failed.Stderr)
	repo.RestoreRemote(behind)

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, "chore(release): 0.1.0", repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.GitRemote("rev-parse", "main"),
		"the release commit reached the remote branch")
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.GitRemote("rev-list", "-n", "1", "v0.1.0"),
		"the remote tag points at a commit on the remote branch")
}

func TestChangelogFlow_OfflineLiftsARequiredEnrichmentPolicy(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver
changelog:
  output: CHANGELOG.md
commits:
  enrichment_policy: required
`
	t.Run("without --offline a required policy with no forge fails", func(t *testing.T) {
		repo := flowRepo(t, cfg)
		repo.Commit("feat: one")

		res := repo.Run(bin, nil, "changelog")

		require.NotEqual(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "remote enrichment (required): no forge resolved")
	})
	t.Run("--offline forces the policy to disabled", func(t *testing.T) {
		repo := flowRepo(t, cfg)
		repo.Commit("feat: one")

		runOK(t, repo, bin, "changelog", "--offline")

		assert.FileExists(t, filepath.Join(repo.Dir, "CHANGELOG.md"))
	})
}
