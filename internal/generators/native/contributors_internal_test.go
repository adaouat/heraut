package native

import (
	"testing"
	"time"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pc(hash, name, email string) parsedCommit {
	return parsedCommit{raw: rawCommit{Hash: hash, Author: name, Email: email, Date: time.Unix(0, 0)}}
}

func TestAuthorsBefore_FirstRelease_Empty(t *testing.T) {
	mr := exectest.NewMockRunner()
	got, err := authorsBefore(mr, "")
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, mr.Calls, "no prev tag → no git call")
}

func TestAuthorsBefore_Set(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("bob@x\nalice@x\nbob@x\n", "", nil)

	got, err := authorsBefore(mr, "v1.0.0")
	require.NoError(t, err)
	assert.True(t, got["bob@x"])
	assert.True(t, got["alice@x"])
	assert.False(t, got["carol@x"])

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[0].Args)
}

func TestCollectContributors_FirstTimerFromGit(t *testing.T) {
	commits := []parsedCommit{pc("aaa", "Alice", "alice@x"), pc("bbb", "Bob", "bob@x")}
	before := map[string]bool{"bob@x": true} // bob contributed before; alice is new
	prs := map[string]PullRequest{
		"aaa": {Number: 7, URL: "u7", AuthorLogin: "alice-gh", RefPrefix: "#"},
	}

	got := collectContributors(commits, before, prs)

	require.Len(t, got, 1, "only first-timers are returned")
	c := got[0]
	assert.Equal(t, "alice@x", c.Author.Email)
	assert.Equal(t, "alice-gh", c.Author.Username, "username overlaid from the PR")
	assert.True(t, c.IsFirstTime)
	require.NotNil(t, c.PR)
	assert.Equal(t, 7, c.PR.Number)
}

func TestCollectContributors_DedupByEmail_OfflineNoPR(t *testing.T) {
	commits := []parsedCommit{pc("aaa", "Alice", "alice@x"), pc("bbb", "Alice", "alice@x")}
	got := collectContributors(commits, map[string]bool{}, nil)

	require.Len(t, got, 1, "same email deduped to one contributor")
	assert.Equal(t, "Alice", got[0].Author.Name)
	assert.Empty(t, got[0].Author.Username, "no PR → no handle offline")
	assert.Nil(t, got[0].PR)
	assert.True(t, got[0].IsFirstTime)
}
