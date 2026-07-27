package native

import (
	"errors"
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
	pc.raw.AuthorHandle = "octocat" // commit author, resolved via enrichment overlay
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
	pc.raw.AuthorHandle = "octocat" // commit author, resolved via enrichment overlay
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

func TestCommitBlock_ByCommitAuthor_NoPR(t *testing.T) {
	pc := parsedFrom("abc1234def", "feat: add thing")
	pc.raw.AuthorHandle = "alice"
	line := renderCommitBlock(t, buildCommit(pc, "https://github.com/o/r/commit/", nil, nil))
	assert.Contains(t, line, " by @alice")
	assert.NotContains(t, line, "in [#", "no PR → no reference link")
}

func TestCommitBlock_ByCommitAuthor_WithPR(t *testing.T) {
	pc := parsedFrom("abc1234def", "feat: add thing")
	pc.raw.AuthorHandle = "alice" // commit author
	enrichment := map[string]PullRequest{
		"abc1234def": {Number: 42, URL: "https://github.com/o/r/pull/42", AuthorLogin: "maintainer"}, // PR opened by someone else
	}
	line := renderCommitBlock(t, buildCommit(pc, "https://github.com/o/r/commit/", nil, enrichment))
	assert.Contains(t, line, " by @alice in [#42](https://github.com/o/r/pull/42)",
		"commit author credited; PR only provides the link (not the PR author)")
	assert.NotContains(t, line, "@maintainer")
}

func TestCommitBlock_NoHandle_NoAttribution(t *testing.T) {
	pc := parsedFrom("abc1234def", "feat: add thing") // AuthorHandle empty
	line := renderCommitBlock(t, buildCommit(pc, "https://github.com/o/r/commit/", nil, nil))
	assert.NotContains(t, line, "by @")
}

// ─── render: New Contributors block ─────────────────────────────────────────────

func TestRenderReleaseNotes_NewContributors(t *testing.T) {
	pc := parsedFrom("aaaaaaa", "feat: add thing")
	pc.raw.AuthorHandle = "newbie" // commit author, resolved via enrichment overlay
	groups := []group{{name: "🚀 Features", order: 0, commits: []parsedCommit{pc}}}
	prs := map[string]PullRequest{
		"aaaaaaa": {Number: 7, URL: "https://github.com/o/r/pull/7", AuthorLogin: "newbie", RefPrefix: "#"},
	}
	contributors := []Contributor{{
		Author:      Author{Name: "New Bie", Email: "newbie@x", Username: "newbie"},
		IsFirstTime: true,
		PR:          &PullRequest{Number: 7, URL: "https://github.com/o/r/pull/7", AuthorLogin: "newbie", RefPrefix: "#"},
	}}
	got, err := renderReleaseNotes("v1.0.0", "", fixedDate1, groups, githubLC, nil, time.Time{}, 3, prs, contributors, tplHeraut{}, nil, "")
	require.NoError(t, err)

	assert.Contains(t, got, "### New Contributors ❤️")
	assert.Contains(t, got, "* @newbie made their first contribution in [#7](https://github.com/o/r/pull/7)")
	assert.Contains(t, got, "by @newbie in [#7]")
}

func TestRenderReleaseNotes_NoFirstTimers_NoBlock(t *testing.T) {
	pc := parsedFrom("bbbbbbb", "feat: x")
	pc.raw.AuthorHandle = "veteran" // commit author, resolved via enrichment overlay
	groups := []group{{name: "🚀 Features", order: 0, commits: []parsedCommit{pc}}}
	enrichment := map[string]PullRequest{
		"bbbbbbb": {Number: 9, URL: "https://github.com/o/r/pull/9", AuthorLogin: "veteran"},
	}
	got, err := renderReleaseNotes("v1.0.0", "", fixedDate1, groups, githubLC, nil, time.Time{}, 3, enrichment, nil, tplHeraut{}, nil, "")
	require.NoError(t, err)

	assert.NotContains(t, got, "New Contributors", "no first-timers → no block")
	assert.Contains(t, got, "by @veteran in [#9]")
}

// ─── Generate: remote_metadata policy branches (contract) ───────────────────────
//
// These exercise enrichForRelease's policy semantics (disabled / optional-degrade /
// required-fatal / --force downgrade) through an injected port.Forge (ADR-0043) rather than the
// retired per-platform CLI dispatch — the policy behaviour under test is unchanged, only the
// enrichment transport is (a stub satisfying port.Forge instead of `gh`/MockRunner responses).

func ghLC() *port.LinkContext {
	return &port.LinkContext{Platform: "github", BaseURL: "https://github.com", Owner: "o", Repo: "r", Token: "tok"}
}

func queueReleaseNotesGit(mr *exectest.MockRunner) {
	mr.QueueResponse("v1.0.0\n", "", nil)                                                                        // previousTag: git describe
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil)                                                          // tagDate: git log -1 --format=%cI
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: x", ""), "", nil) // collectCommits: git log
}

// countingForge records how many times Enrich was called and returns a canned result or error.
type countingForge struct {
	calls int
	en    port.Enrichment
	err   error
}

func (f *countingForge) Type() string                     { return "github" }
func (f *countingForge) Identity() port.ForgeIdentity     { return port.ForgeIdentity{Type: "github"} }
func (f *countingForge) CommitURL(sha string) string      { return "" }
func (f *countingForge) ChangeURL(int) string             { return "" }
func (f *countingForge) CompareURL(string, string) string { return "" }
func (f *countingForge) Enrich(c []port.Commit) (port.Enrichment, error) {
	f.calls++
	return f.en, f.err
}

func TestGenerate_Enrich_Disabled_NoAPICall(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae
	forge := &countingForge{en: port.Enrichment{
		PRs:     map[string]port.PullRequest{"abc1234567": {Number: 42, RefPrefix: "#"}},
		Authors: map[string]string{"abc1234567": "octocat"},
	}}
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "disabled"}, ModeReleaseNotes, WithForge(forge))

	out, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err)
	assert.NotContains(t, out, "by @")
	assert.Equal(t, 0, forge.calls, "disabled must never call the forge")
	assert.False(t, g.Degraded())

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[3].Args)
}

func TestGenerate_Enrich_OptionalSuccess(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	// authorsBefore runs after forge enrich, so its response is queued last (T140-style ripple).
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae
	forge := &countingForge{en: port.Enrichment{
		PRs:     map[string]port.PullRequest{"abc1234567": {Number: 42, URL: "https://github.com/o/r/pull/42", RefPrefix: "#"}},
		Authors: map[string]string{"abc1234567": "octocat"},
	}}
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes, WithForge(forge))

	out, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err)
	assert.Contains(t, out, "by @octocat in [#42]")
	assert.False(t, g.Degraded())
	assert.Equal(t, 1, forge.calls)

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[3].Args)
}

func TestGenerate_Enrich_OptionalFailure_Degrades(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	// optional degrades on the forge failure (no error returned) and proceeds to authorsBefore.
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae
	forge := &countingForge{err: errors.New("API rate limit exceeded")}
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes, WithForge(forge))

	out, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err, "optional degrades, does not fail")
	assert.NotContains(t, out, "by @", "rendered bare on degrade")
	assert.True(t, g.Degraded())

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[3].Args)
}

// On an optional degrade the failure reason is captured on the generator (surfaced by the
// pipeline as a step sub-result) rather than written straight to os.Stderr, where it collided
// with the live spinner line.
func TestGenerate_Enrich_OptionalFailure_CapturesReason(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore
	forge := &countingForge{err: errors.New("API rate limit exceeded")}
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes, WithForge(forge))

	_, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err)
	require.True(t, g.Degraded())
	assert.Contains(t, g.DegradedReason(), "remote enrichment unavailable")
	assert.Contains(t, g.DegradedReason(), "API rate limit exceeded", "reason carries the underlying failure detail")
}

func TestGenerate_Enrich_RequiredFailure_Errors(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	forge := &countingForge{err: errors.New("API rate limit exceeded")}
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "required"}, ModeReleaseNotes, WithForge(forge))

	_, err := g.Generate("v1.1.0", ghLC())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

// --force downgrades required to optional: a fetch failure degrades (warn + render bare) instead
// of erroring, so the user can still produce a changelog when the remote is unreachable.
func TestGenerate_Enrich_RequiredFailure_ForceDegrades(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore — reached because force degrades past the failure
	forge := &countingForge{err: errors.New("API rate limit exceeded")}
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "required", Force: true}, ModeReleaseNotes, WithForge(forge))

	out, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err, "--force downgrades required to optional")
	assert.NotContains(t, out, "by @", "rendered bare on forced degrade")
	assert.True(t, g.Degraded())
}

// --force also downgrades the nil-context / no-forge (unconfigured remote) case: render bare, no
// error.
func TestGenerate_Enrich_RequiredNilContext_ForceDegrades(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "required", Force: true}, ModeReleaseNotes)

	_, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err, "--force downgrades required even with no remote configured")
}

// required with no resolvable remote (nil LinkContext, no injected forge) cannot be satisfied —
// there is nothing to fetch from — so it must be a hard error, not a silent metadata-less render.
// Regression guard for the state native was permanently in before changelog.remote worked with
// the native generator.
func TestGenerate_Enrich_RequiredNilContext_Errors(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueReleaseNotesGit(mr)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore — reached only if required is (wrongly) not enforced
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "required"}, ModeReleaseNotes)

	_, err := g.Generate("v1.1.0", nil) // no remote / platform → required cannot be satisfied
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
	// M2: the message must name what's actually missing under the current forge-based
	// enrichment model — changelog.remote was removed and release.platforms no longer
	// participates in enrichment, so the old wording ("no changelog remote or release
	// platform configured") is stale and misleading.
	assert.Contains(t, err.Error(), "forge", "must name a resolvable forge as the missing piece")
	assert.NotContains(t, err.Error(), "changelog remote", "changelog.remote was removed and no longer participates in enrichment")
	assert.NotContains(t, err.Error(), "release platform", "release.platforms no longer participates in enrichment")
}

// Changelog mode enriches only the new (unreleased) section, not historical releases, so a
// full regeneration stays O(1) enrichment calls regardless of release count (ADR-0034 §5).
func TestGenerate_Enrich_ChangelogEnrichesOnlyNewRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)                                                                          // listTags (one existing tag)
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com", "2026-02-01T00:00:00Z", "feat: new", ""), "", nil) // new release commits
	mr.QueueResponse(record("bbb2222222", "B", "b@example.com", "2026-01-01T00:00:00Z", "fix: old", ""), "", nil)  // existing v1.0.0 commits (no enrich)
	forge := &countingForge{en: port.Enrichment{
		PRs:     map[string]port.PullRequest{"aaa1111111": {Number: 50, URL: "https://github.com/o/r/pull/50", RefPrefix: "#"}},
		Authors: map[string]string{"aaa1111111": "octocat"},
	}}
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeChangelog, WithForge(forge))

	body, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err)
	assert.Contains(t, body, "## [1.1.0]")
	assert.Contains(t, body, "by @octocat in [#50]", "new section is enriched")
	assert.Contains(t, body, "## [1.0.0]", "historical section present but bare")

	assert.Equal(t, 1, forge.calls, "only the new release triggers enrichment")
}
