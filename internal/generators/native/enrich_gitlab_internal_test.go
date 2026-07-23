package native

import (
	"errors"
	"testing"
	"time"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

func TestEnrichGitLab_ReviewFields(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"u","title":"t","author":{"username":"alice"},"labels":[],
		"created_at":"2026-01-01T00:00:00Z","merged_at":"2026-01-02T00:00:00Z",
		"merged_by":{"username":"maint"}}]`, "", nil)

	got, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	pr := got["abc123"]
	assert.Equal(t, "2026-01-01T00:00:00Z", pr.CreatedAt.UTC().Format(time.RFC3339))
	assert.Equal(t, "2026-01-02T00:00:00Z", pr.MergedAt.UTC().Format(time.RFC3339))
	assert.Equal(t, "maint", pr.MergedBy.Username)
	assert.Nil(t, pr.Approvers, "GitLab approvers are best-effort empty (no extra call)")
}

func gitlabLC() *port.LinkContext {
	return &port.LinkContext{Platform: "gitlab", BaseURL: "https://gitlab.com", Owner: "g", Repo: "p", Token: "tok"}
}

func TestEnrichGitLab_MapsMR(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"https://gitlab.com/g/p/-/merge_requests/7","author":{"username":"alice"}}]`, "", nil)

	result, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, PullRequest{Number: 7, URL: "https://gitlab.com/g/p/-/merge_requests/7", AuthorLogin: "alice", RefPrefix: "!"}, result["abc123"])

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "glab", mr.Calls[0].Name)
	assert.Equal(t, []string{"api", "projects/g%2Fp/repository/commits/abc123/merge_requests"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"GITLAB_TOKEN=tok"}, mr.Calls[0].Env)
}

func TestEnrichGitLab_TitleAndLabels(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"u","title":"Add OAuth","author":{"username":"alice"},"labels":["enhancement","area/auth"]}]`, "", nil)

	got, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	assert.Equal(t, "Add OAuth", got["abc123"].Title)
	assert.Equal(t, []string{"enhancement", "area/auth"}, got["abc123"].Labels)
}

func TestEnrichGitLab_NoMR_Absent(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[]`, "", nil)

	result, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestEnrichGitLab_ErrorWrapped(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "401 Unauthorized", errors.New("exit status 1"))

	_, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge_requests")
}

func TestEnrichGitLab_MalformedJSON(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("not json", "", nil)

	_, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing glab api merge_requests response")
}

func TestEnrichGitLab_Subgroup(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":3,"web_url":"u","author":{"username":"bob"}}]`, "", nil)
	lc := &port.LinkContext{Platform: "gitlab", BaseURL: "https://gitlab.com", Owner: "group/subgroup", Repo: "project", Token: "tok"}

	_, err := enrichGitLab(mr, lc, []string{"deadbeef"})
	require.NoError(t, err)
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"api", "projects/group%2Fsubgroup%2Fproject/repository/commits/deadbeef/merge_requests"}, mr.Calls[0].Args)
}

func TestEnrichGitLab_SelfHostedHostInAPIEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"https://git.example.com/g/p/-/merge_requests/7","author":{"username":"alice"}}]`, "", nil)
	lc := &port.LinkContext{Platform: "gitlab", BaseURL: "https://git.example.com", Owner: "g", Repo: "p", Token: "tok"}

	_, err := enrichGitLab(mr, lc, []string{"abc123"})
	require.NoError(t, err)
	require.Len(t, mr.Calls, 1)
	assert.Contains(t, mr.Calls[0].Env, "GITLAB_TOKEN=tok")
	assert.Contains(t, mr.Calls[0].Env, "GITLAB_HOST=git.example.com")
}

func TestFetchGitLabAuthors_MapsAndOmitsNull(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"data":{"project":{"repository":{"commits":{"nodes":[
		{"sha":"aaa","author":{"username":"alice"}},
		{"sha":"bbb","author":null},
		{"sha":"ccc","author":{"username":"carol"}}
	],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}}`, "", nil)

	got, err := fetchGitLabAuthors(mr, gitlabLC(), "tagsha", "2026-01-01T00:00:00Z", map[string]bool{"aaa": true, "bbb": true, "ccc": true})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"aaa": "alice", "ccc": "carol"}, got)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "glab", mr.Calls[0].Name)
	assert.Equal(t, "graphql", mr.Calls[0].Args[1])
	assert.Contains(t, mr.Calls[0].Args[3], `commits(ref:"tagsha",committedAfter:"2026-01-01T00:00:00Z",first:100`)
	assert.Contains(t, mr.Calls[0].Env, "GITLAB_TOKEN=tok")
}

func TestFetchGitLabAuthors_Paginates(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"data":{"project":{"repository":{"commits":{"nodes":[{"sha":"aaa","author":{"username":"alice"}}],"pageInfo":{"endCursor":"C1","hasNextPage":true}}}}}}`, "", nil)
	mr.QueueResponse(`{"data":{"project":{"repository":{"commits":{"nodes":[{"sha":"bbb","author":{"username":"bob"}}],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}}`, "", nil)

	got, err := fetchGitLabAuthors(mr, gitlabLC(), "tagsha", "", map[string]bool{"aaa": true, "bbb": true})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"aaa": "alice", "bbb": "bob"}, got)
	require.Len(t, mr.Calls, 2)
	assert.Contains(t, mr.Calls[1].Args[3], `after:"C1"`)
}

func TestFetchGitLabAuthors_ErrorsArray(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"errors":[{"message":"boom"}]}`, "", nil)
	_, err := fetchGitLabAuthors(mr, gitlabLC(), "tagsha", "", map[string]bool{"aaa": true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestFetchGitLabMRs_InvertsMergeAndSourceShas(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"data":{"project":{"mergeRequests":{"nodes":[
		{"iid":"7","webUrl":"https://gitlab.com/g/p/-/merge_requests/7","title":"Add x",
		 "author":{"username":"alice"},"mergedAt":"2026-01-02T00:00:00Z","mergeUser":{"username":"maint"},
		 "labels":{"nodes":[{"title":"enhancement"}]},
		 "mergeCommitSha":"merge1","commits":{"nodes":[{"sha":"src1"},{"sha":"src2"}]}}
	],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}`, "", nil)

	got, err := fetchGitLabMRs(mr, gitlabLC(), "2026-01-01T00:00:00Z", map[string]bool{"merge1": true, "src1": true, "nope": true})
	require.NoError(t, err)

	want := PullRequest{Number: 7, URL: "https://gitlab.com/g/p/-/merge_requests/7", Title: "Add x",
		AuthorLogin: "alice", Labels: []string{"enhancement"}, RefPrefix: "!",
		MergedAt: got["merge1"].MergedAt, MergedBy: Author{Username: "maint"}}
	assert.Equal(t, want, got["merge1"], "merge commit sha maps to the MR (iid 7 parsed from string)")
	assert.Equal(t, 7, got["src1"].Number, "source commit sha maps to the MR")
	assert.NotContains(t, got, "src2", "src2 not in want")
	assert.NotContains(t, got, "nope", "no MR → absent")
	assert.Equal(t, "2026-01-02T00:00:00Z", got["merge1"].MergedAt.UTC().Format(time.RFC3339))

	require.Len(t, mr.Calls, 1)
	assert.Contains(t, mr.Calls[0].Args[3], `mergeRequests(state:merged,mergedAfter:"2026-01-01T00:00:00Z",first:100`)
}

func TestFetchGitLabMRs_Paginates(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"data":{"project":{"mergeRequests":{"nodes":[{"iid":"1","webUrl":"u1","author":{"username":"a"},"mergeCommitSha":"m1","commits":{"nodes":[]}}],"pageInfo":{"endCursor":"C1","hasNextPage":true}}}}}`, "", nil)
	mr.QueueResponse(`{"data":{"project":{"mergeRequests":{"nodes":[{"iid":"2","webUrl":"u2","author":{"username":"b"},"mergeCommitSha":"m2","commits":{"nodes":[]}}],"pageInfo":{"endCursor":"","hasNextPage":false}}}}}`, "", nil)

	got, err := fetchGitLabMRs(mr, gitlabLC(), "", map[string]bool{"m1": true, "m2": true})
	require.NoError(t, err)
	assert.Equal(t, 1, got["m1"].Number)
	assert.Equal(t, 2, got["m2"].Number)
	require.Len(t, mr.Calls, 2)
	assert.Contains(t, mr.Calls[1].Args[3], `after:"C1"`)
}

func TestFetchGitLabMRs_ErrorsArray(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`{"errors":[{"message":"bad mr query"}]}`, "", nil)
	_, err := fetchGitLabMRs(mr, gitlabLC(), "", map[string]bool{"m1": true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad mr query")
}

// End-to-end: GitLab release-notes enrichment renders "!N" (not "#N"). GitLab does not yet
// resolve the commit-author handle (GitHub-only in this cut), so the commit line carries only
// the MR reference link — no "by @" — even though the MR itself has an author.
func TestGenerate_Enrich_GitLab(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)                                                                        // previousTag
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil)                                                          // tagDate
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: x", ""), "", nil) // collectCommits
	mr.QueueResponse(`[{"iid":7,"web_url":"https://gitlab.com/g/p/-/merge_requests/7","author":{"username":"alice"}}]`, "", nil)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", gitlabLC())
	require.NoError(t, err)
	assert.Contains(t, out, "in [!7](https://gitlab.com/g/p/-/merge_requests/7)")
	assert.NotContains(t, out, "by @", "GitLab commit-author handle resolution is not yet implemented")

	require.Len(t, mr.Calls, 5)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[4].Args)
}
