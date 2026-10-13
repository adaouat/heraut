package app

import (
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

// versionBranchFixture: main has v7.7.0; release/7.8.0 is cut from it and carries one commit.
// release/7.8.0 is left checked out.
func versionBranchFixture(t *testing.T) (git func(args ...string), commit func(msg string), tag func(name string), cfg *config.Config) {
	t.Helper()
	git = historyRepo(t)
	commit = func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag = func(name string) { git("tag", "-a", "-m", name, name) }

	commit("feat: initial")
	tag("v7.7.0")
	git("checkout", "-b", "release/7.8.0")
	commit("feat: client scope")

	cfg = &config.Config{Version: "1", Versioning: config.Versioning{
		Strategy: "semver",
		Branches: []config.BranchRule{{Name: "main"}, {Name: "release/*"}},
	}}
	return git, commit, tag, cfg
}

func TestVersionBranch_RealRepo_ReleaseLifecycle(t *testing.T) {
	_, commit, tag, cfg := versionBranchFixture(t)
	runner := execadapter.New(false, false)
	resolve := func(label string) (string, error) {
		var opts []ResolverOption
		if label != "" {
			opts = append(opts, WithPreRelease(label))
		}
		r, err := NewResolver(cfg, "", false, "", "", runner, opts...)
		require.NoError(t, err)
		res, err := r.Resolve()
		return res.Tag, err
	}

	commit("feat!: breaking, but the branch is the version")
	got, err := resolve("")
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0", got, "a breaking commit never bumps a version branch")

	got, err = resolve("rc")
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0-rc.1", got)
	tag(got)

	commit("fix: stabilise")
	got, err = resolve("rc")
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0-rc.2", got)
	tag(got)

	got, err = resolve("")
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0", got, "the final follows the release candidates")
	tag(got)

	_, err = resolve("")
	require.ErrorIs(t, err, semver.ErrTagExists)

	_, err = resolve("rc")
	require.ErrorIs(t, err, semver.ErrPreReleaseRegression, "no pre-release after the final")

	r, err := NewResolver(cfg, "", false, "7.8.0", "158404", runner)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0+158404", res.Tag, "repeat builds go through --set-version/--set-build-id")
	assert.Empty(t, res.Warnings, "the override equals the branch version")
}

func TestVersionBranch_RealRepo_BuildMetadataTagIsTheRelease(t *testing.T) {
	_, _, tag, cfg := versionBranchFixture(t)
	tag("v7.8.0+3")

	r, err := NewResolver(cfg, "", false, "", "", execadapter.New(false, false))
	require.NoError(t, err)
	_, err = r.Resolve()

	require.ErrorIs(t, err, semver.ErrTagExists)
}

func TestVersionBranch_RealRepo_OrderingWarningOnlyWhenMerged(t *testing.T) {
	git, commit, tag, cfg := versionBranchFixture(t)
	git("checkout", "main")
	commit("feat: later")
	tag("v7.9.0")
	git("checkout", "release/7.8.0")
	runner := execadapter.New(false, false)
	resolve := func() []string {
		r, err := NewResolver(cfg, "", false, "", "", runner)
		require.NoError(t, err)
		res, err := r.Resolve()
		require.NoError(t, err)
		assert.Equal(t, "v7.8.0", res.Tag)
		return res.Warnings
	}

	assert.Empty(t, resolve(), "a higher tag on another line is legitimate")

	git("merge", "--no-ff", "-m", "chore: merge main", "main")
	assert.Equal(t, []string{"branch version 7.8.0 is lower than v7.9.0, already in this branch's history"}, resolve())
}

func TestVersionBranch_RealRepo_SetVersionMismatchWarns(t *testing.T) {
	_, _, _, cfg := versionBranchFixture(t)

	r, err := NewResolver(cfg, "", false, "7.9.1", "", execadapter.New(false, false))
	require.NoError(t, err)
	res, err := r.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "v7.9.1", res.Tag, "--set-version stays an explicit override")
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "--set-version 7.9.1 differs from the version of branch release/7.8.0 (7.8.0)")
}

func TestVersionBranch_RealRepo_VersionCurrent(t *testing.T) {
	_, _, tag, cfg := versionBranchFixture(t)
	runner := execadapter.New(false, false)

	_, err := CurrentTag(runner, cfg, "", false)
	require.ErrorIs(t, err, errNoTagsFound, "nothing is tagged on this version yet")

	tag("v7.8.0-rc.1")
	_, err = CurrentTag(runner, cfg, "", false)
	require.ErrorIs(t, err, errNoTagsFound)
	assert.Contains(t, err.Error(), "--include-pre-release")
	cur, err := CurrentTag(runner, cfg, "", true)
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0-rc.1", cur)

	tag("v7.8.0")
	cur, err = CurrentTag(runner, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0", cur)
}

// TestVersionBranch_RealRepo_CommitCheckFromLatestTag: linting a fresh version branch ranges from
// the highest tag in its history, not from the start of the repository.
func TestVersionBranch_RealRepo_CommitCheckFromLatestTag(t *testing.T) {
	_, _, tag, cfg := versionBranchFixture(t)
	runner := execadapter.New(false, false)

	rng, noTags, err := ResolveFromLatestTag(runner, cfg, "")
	require.NoError(t, err)
	assert.False(t, noTags)
	assert.Equal(t, "v7.7.0..HEAD", rng, "nothing is tagged on 7.8.0 yet: fall back to the highest tag in history")

	tag("v7.8.0-rc.1")
	rng, _, err = ResolveFromLatestTag(runner, cfg, "")
	require.NoError(t, err)
	assert.Equal(t, "v7.8.0-rc.1..HEAD", rng)
}
