package native

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// record builds one git-log record in the shape git emits for --format=logFormat:
// six \x01-delimited fields terminated by a NUL.
func record(hash, author, email, date, subject, body string) string {
	return strings.Join([]string{hash, author, email, date, subject, body}, "\x01") + "\x00"
}

func TestCollectCommits_ParsesRecords(t *testing.T) {
	mr := exectest.NewMockRunner()
	// git separates --format entries with a newline; each record ends with the NUL.
	stdout := record("abc1234", "Alice", "alice@example.com", "2026-06-25T10:00:00Z",
		"feat(parser): add thing", "Body line one\nBody line two") +
		"\n" +
		record("def5678", "Bob", "bob@example.com", "2026-06-26T11:30:00+02:00",
			"fix: a bug", "")
	mr.QueueResponse(stdout, "", nil)

	commits, err := collectCommits(mr, "v1.0.0..v1.1.0")
	require.NoError(t, err)
	require.Len(t, commits, 2)

	assert.Equal(t, "abc1234", commits[0].Hash)
	assert.Equal(t, "Alice", commits[0].Author)
	assert.Equal(t, "alice@example.com", commits[0].Email)
	assert.Equal(t, "feat(parser): add thing", commits[0].Subject)
	assert.Equal(t, "Body line one\nBody line two", commits[0].Body)
	assert.True(t, commits[0].Date.Equal(time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)),
		"date should parse as RFC3339 instant, got %s", commits[0].Date)

	assert.Equal(t, "def5678", commits[1].Hash)
	assert.Equal(t, "Bob", commits[1].Author)
	assert.Empty(t, commits[1].Body)

	// Exact git invocation (contract).
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"log", "v1.0.0..v1.1.0", "--reverse", "--format=" + logFormat}, mr.Calls[0].Args)
}

func TestCollectCommits_FullHistoryOmitsRange(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(record("h1", "A", "a@example.com", "2026-01-01T00:00:00Z", "feat: x", ""), "", nil)

	_, err := collectCommits(mr, "")
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"log", "--reverse", "--format=" + logFormat}, mr.Calls[0].Args)
}

func TestCollectCommits_EmptyOutput(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	commits, err := collectCommits(mr, "v1.0.0..HEAD")
	require.NoError(t, err)
	assert.Empty(t, commits)
}

func TestCollectCommits_GitError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "fatal: bad revision", errors.New("exit status 128"))

	_, err := collectCommits(mr, "bogus..range")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git log")
}

func TestCollectCommits_BadDateErrors(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(record("h1", "A", "a@example.com", "not-a-date", "feat: x", ""), "", nil)

	_, err := collectCommits(mr, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "date")
}

func TestPreviousTag_ReturnsEarlierTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)

	prev, err := previousTag(mr, "v1.1.0", "")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", prev)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"describe", "--tags", "--abbrev=0", "v1.1.0^"}, mr.Calls[0].Args)
}

func TestPreviousTag_WithGlobScopesMatch(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("prod/1.0.0\n", "", nil)

	prev, err := previousTag(mr, "prod/1.1.0", "prod/*")
	require.NoError(t, err)
	assert.Equal(t, "prod/1.0.0", prev)

	assert.Equal(t, []string{"describe", "--tags", "--abbrev=0", "--match", "prod/*", "prod/1.1.0^"},
		mr.Calls[0].Args)
}

func TestPreviousTag_FirstReleaseReturnsEmpty(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "fatal: No names found, cannot describe anything.", errors.New("exit status 128"))

	prev, err := previousTag(mr, "v1.0.0", "")
	require.NoError(t, err)
	assert.Empty(t, prev)
}

func TestPreviousTag_OtherErrorPropagates(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "fatal: not a git repository", errors.New("exit status 128"))

	_, err := previousTag(mr, "v1.0.0", "")
	require.Error(t, err)
}

// TestListMergedTags_ReturnsAncestorTags covers T334's fix: the oldest-in-scope fallback must
// only ever consider tags that are actual ancestors of ref, never an unrelated branch's tag.
// Called with ref directly (no "^"), using --no-contains ref to exclude tags on ref's own commit —
// see TestListMergedTags_RootCommitReturnsEmpty for why "^" is avoided.
func TestListMergedTags_ReturnsAncestorTags(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\nv0.9.0\n", "", nil)

	tags, err := listMergedTags(mr, "v1.1.0")
	require.NoError(t, err)
	assert.Equal(t, []string{"v1.0.0", "v0.9.0"}, tags)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"tag", "-l", "--merged", "v1.1.0", "--no-contains", "v1.1.0", "--sort=-version:refname"},
		mr.Calls[0].Args)
}

// TestListMergedTags_RootCommitReturnsEmpty covers the root-commit edge case: "<tag>^" would fail
// to resolve for a root commit with a locale-dependent stderr message that cannot be matched
// reliably. --merged ref --no-contains ref needs no "^": for a root-commit ref, git itself returns
// an empty list at exit 0 (confirmed in TestListMergedTags_RealGit_AncestryAndSelfExclusion), so
// there is no special stderr case to probe for.
func TestListMergedTags_RootCommitReturnsEmpty(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	tags, err := listMergedTags(mr, "v1.0.0")
	require.NoError(t, err)
	assert.Empty(t, tags)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"tag", "-l", "--merged", "v1.0.0", "--no-contains", "v1.0.0", "--sort=-version:refname"},
		mr.Calls[0].Args)
}

func TestListMergedTags_OtherErrorPropagates(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "fatal: not a git repository", errors.New("exit status 128"))

	_, err := listMergedTags(mr, "v1.0.0")
	require.Error(t, err)
}

func TestListMergedTags_EmptyOutput(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	tags, err := listMergedTags(mr, "v1.0.0")
	require.NoError(t, err)
	assert.Empty(t, tags)
}

// TestListMergedTags_RealGit_AncestryAndSelfExclusion confirms on a real git binary (not
// MockRunner) that `git tag -l --merged <t> --no-contains <t>` lists ancestors only, excludes tags
// on t's own commit, and returns an empty list at exit 0 for a root-commit tag — the three
// properties listMergedTags relies on.
func TestListMergedTags_RealGit_AncestryAndSelfExclusion(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	t.Chdir(dir)

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	commit := func(msg string) { git("commit", "--allow-empty", "-m", msg) }

	git("init")
	commit("feat: initial") // root commit, no parent
	git("tag", "v1.0.0")
	commit("feat: second")
	git("tag", "v1.1.0-rc.1") // same commit as v1.1.0 below
	git("tag", "v1.1.0")

	runner := execadapter.New(false, false)

	// Root-commit tag: no ancestor tags exist, and git must report that as an empty list at
	// exit 0, not an error — the case the old "<t>^" + stderr-probe shape existed to handle.
	tags, err := listMergedTags(runner, "v1.0.0")
	require.NoError(t, err)
	assert.Empty(t, tags, "a root-commit tag has no ancestors and must not error")

	// Ancestry + self-exclusion: v1.1.0's ancestor is v1.0.0 only; v1.1.0 and v1.1.0-rc.1 share
	// v1.1.0's own commit and must both be excluded by --no-contains, not just --merged.
	tags, err = listMergedTags(runner, "v1.1.0")
	require.NoError(t, err)
	assert.Equal(t, []string{"v1.0.0"}, tags)

	// listAncestorTags keeps the same-commit tag (v1.1.0-rc.1) and drops only v1.1.0 itself.
	tags, err = listAncestorTags(runner, "v1.1.0")
	require.NoError(t, err)
	assert.Equal(t, []string{"v1.1.0-rc.1", "v1.0.0"}, tags)
}

// TestListAncestorTags_Argv pins the in-scope ancestor listing (ADR-0065): no --no-contains, so
// tags on ref's own commit stay in, and ref itself is dropped by name.
func TestListAncestorTags_Argv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.1\nv1.4.0\nv1.3.0\n", "", nil)

	tags, err := listAncestorTags(mr, "v1.4.1")
	require.NoError(t, err)
	assert.Equal(t, []string{"v1.4.0", "v1.3.0"}, tags)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"tag", "-l", "--merged", "v1.4.1", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestListAncestorTags_ErrorPropagates(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "fatal: not a git repository", errors.New("exit status 128"))

	_, err := listAncestorTags(mr, "v1.0.0")
	require.ErrorContains(t, err, "listing tags merged into v1.0.0")
}
