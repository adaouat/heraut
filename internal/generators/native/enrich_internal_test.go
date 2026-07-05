package native

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/adaouat/heraut/internal/port"
)

// ghGraphQLResponse builds a one-PR gh api graphql JSON body for alias s0.
func ghGraphQLResponse(number int, url, login string) string {
	return fmt.Sprintf(
		`{"data":{"repository":{"s0":{"associatedPullRequests":{"nodes":[{"number":%d,"url":%q,"author":{"login":%q}}]}}}}}`,
		number, url, login)
}

func parsedFrom(hash, subject string) parsedCommit {
	pc := parsedCommit{raw: rawCommit{Hash: hash, Subject: subject, Date: fixedDate1}}
	pc.parsed, _ = conventionalcommit.Parse(subject)
	return pc
}

// ─── render: commit-line PR suffix ──────────────────────────────────────────────

// renderCommitBlock renders the built-in "commit" block for one tplCommit — the successor to
// buildCommitLine now that the commit line lives in a template block (ADR-0037).
func renderCommitBlock(t *testing.T, c tplCommit) string {
	t.Helper()
	tt, err := template.New("native").Funcs(templateFuncs()).Parse(blocksTmpl)
	require.NoError(t, err)
	var sb strings.Builder
	require.NoError(t, tt.ExecuteTemplate(&sb, "commit", c))
	return sb.String()
}

func TestCommitBlock_Enriched(t *testing.T) {
	pc := parsedFrom("abc1234def", "feat: add thing")
	enrichment := map[string]PullRequest{
		"abc1234def": {Number: 42, URL: "https://github.com/o/r/pull/42", AuthorLogin: "octocat"},
	}
	line := renderCommitBlock(t, buildCommit(pc, "https://github.com/o/r/commit/", nil, enrichment))

	assert.Contains(t, line, " by @octocat in [#42](https://github.com/o/r/pull/42)")
	assert.Less(t, strings.Index(line, "abc1234"), strings.Index(line, "by @octocat"),
		"PR suffix comes after the commit hash link")
}

func TestCommitBlock_NoEnrichment(t *testing.T) {
	pc := parsedFrom("abc1234def", "feat: add thing")
	assert.NotContains(t, renderCommitBlock(t, buildCommit(pc, "", nil, nil)), "by @")
}

func TestCommitBlock_EnrichedBeforeTickets(t *testing.T) {
	pc := parsedFrom("abc1234def", "fix: resolve PROJ-7")
	enrichment := map[string]PullRequest{
		"abc1234def": {Number: 42, URL: "https://github.com/o/r/pull/42", AuthorLogin: "octocat"},
	}
	tickets := []config.Ticket{{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/PROJ-{ticket}"}}
	line := renderCommitBlock(t, buildCommit(pc, "https://github.com/o/r/commit/", tickets, enrichment))

	assert.Contains(t, line, "by @octocat in [#42]")
	assert.Contains(t, line, "([PROJ-7]")
	assert.Less(t, strings.Index(line, "by @octocat"), strings.Index(line, "([PROJ-7]"),
		"PR suffix comes before ticket links")
}

// ─── render: New Contributors block ─────────────────────────────────────────────

func TestRenderReleaseNotes_NewContributors(t *testing.T) {
	groups := []group{{name: "🚀 Features", order: 0, commits: []parsedCommit{parsedFrom("aaaaaaa", "feat: add thing")}}}
	prs := map[string]PullRequest{
		"aaaaaaa": {Number: 7, URL: "https://github.com/o/r/pull/7", AuthorLogin: "newbie", RefPrefix: "#"},
	}
	contributors := []Contributor{{
		Author:      Author{Name: "New Bie", Email: "newbie@x", Username: "newbie"},
		IsFirstTime: true,
		PR:          &PullRequest{Number: 7, URL: "https://github.com/o/r/pull/7", AuthorLogin: "newbie", RefPrefix: "#"},
	}}
	got, err := renderReleaseNotes("v1.0.0", "", fixedDate1, groups, githubLC, nil, time.Time{}, 3, prs, contributors, tplHeraut{})
	require.NoError(t, err)

	assert.Contains(t, got, "### New Contributors ❤️")
	assert.Contains(t, got, "* @newbie made their first contribution in [#7](https://github.com/o/r/pull/7)")
	assert.Contains(t, got, "by @newbie in [#7]")
}

func TestRenderReleaseNotes_NoFirstTimers_NoBlock(t *testing.T) {
	groups := []group{{name: "🚀 Features", order: 0, commits: []parsedCommit{parsedFrom("bbbbbbb", "feat: x")}}}
	enrichment := map[string]PullRequest{
		"bbbbbbb": {Number: 9, URL: "https://github.com/o/r/pull/9", AuthorLogin: "veteran"},
	}
	got, err := renderReleaseNotes("v1.0.0", "", fixedDate1, groups, githubLC, nil, time.Time{}, 3, enrichment, nil, tplHeraut{})
	require.NoError(t, err)

	assert.NotContains(t, got, "New Contributors", "no first-timers → no block")
	assert.Contains(t, got, "by @veteran in [#9]")
}

// ─── Generate: remote_metadata policy branches (contract) ───────────────────────

func ghLC() *port.LinkContext {
	return &port.LinkContext{Platform: "github", BaseURL: "https://github.com", Owner: "o", Repo: "r", Token: "tok"}
}

func queueReleaseNotesGit(mr *exectest.MockRunner) {
	mr.QueueResponse("v1.0.0\n", "", nil)                                                                        // previousTag: git describe
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil)                                                          // tagDate: git log -1 --format=%cI
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: x", ""), "", nil) // collectCommits: git log
}

func TestGenerate_Enrich_Disabled_NoAPICall(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "disabled"}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err)
	assert.NotContains(t, out, "by @")
	for _, c := range mr.Calls {
		assert.NotEqual(t, "gh", c.Name, "disabled must never call gh")
	}
	assert.False(t, g.Degraded())

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[3].Args)
}

func TestGenerate_Enrich_OptionalSuccess(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	mr.QueueResponse(ghGraphQLResponse(42, "https://github.com/o/r/pull/42", "octocat"), "", nil)
	// authorsBefore runs after platform enrich, so its response is queued last (T140-style ripple).
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err)
	assert.Contains(t, out, "by @octocat in [#42]")
	assert.False(t, g.Degraded())

	require.Len(t, mr.Calls, 5)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[4].Args)
}

func TestGenerate_Enrich_OptionalFailure_Degrades(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	mr.QueueResponse("", "API rate limit exceeded", errors.New("exit status 1"))
	// optional degrades on the gh failure (no error returned) and proceeds to authorsBefore.
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err, "optional degrades, does not fail")
	assert.NotContains(t, out, "by @", "rendered bare on degrade")
	assert.True(t, g.Degraded())

	require.Len(t, mr.Calls, 5)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[4].Args)
}

func TestGenerate_Enrich_RequiredFailure_Errors(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	mr.QueueResponse("", "API rate limit exceeded", errors.New("exit status 1"))
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "required"}, ModeReleaseNotes)

	_, err := g.Generate("v1.1.0", ghLC())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

// Changelog mode enriches only the new (unreleased) section, not historical releases, so a
// full regeneration stays O(1) API calls regardless of release count (ADR-0034 §5).
func TestGenerate_Enrich_ChangelogEnrichesOnlyNewRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)                                                                          // listTags (one existing tag)
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com", "2026-02-01T00:00:00Z", "feat: new", ""), "", nil) // new release commits
	mr.QueueResponse(ghGraphQLResponse(50, "https://github.com/o/r/pull/50", "octocat"), "", nil)                  // new release enrich
	mr.QueueResponse(record("bbb2222222", "B", "b@example.com", "2026-01-01T00:00:00Z", "fix: old", ""), "", nil)  // existing v1.0.0 commits (no enrich)
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeChangelog)

	body, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err)
	assert.Contains(t, body, "## [1.1.0]")
	assert.Contains(t, body, "by @octocat in [#50]", "new section is enriched")
	assert.Contains(t, body, "## [1.0.0]", "historical section present but bare")

	ghCalls := 0
	for _, c := range mr.Calls {
		if c.Name == "gh" {
			ghCalls++
		}
	}
	assert.Equal(t, 1, ghCalls, "only the new release triggers enrichment")
}
