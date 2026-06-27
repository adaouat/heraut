package native

import (
	"errors"
	"strings"
	"testing"
	"time"

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
	assert.Equal(t, []string{"log", "v1.0.0..v1.1.0", "--format=" + logFormat}, mr.Calls[0].Args)
}

func TestCollectCommits_FullHistoryOmitsRange(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(record("h1", "A", "a@example.com", "2026-01-01T00:00:00Z", "feat: x", ""), "", nil)

	_, err := collectCommits(mr, "")
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"log", "--format=" + logFormat}, mr.Calls[0].Args)
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
