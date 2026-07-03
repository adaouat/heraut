# Unified Enrichment Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace native's three divergent per-platform first-timer mechanisms and its `prInfo` struct with one two-tier model — a git-derived local tier (authors + `is_first_time`) and a normalized remote tier (`PullRequest` with `title`/`labels`), rendered from a common schema.

**Architecture:** A new local tier computes contributors and first-timer status from git alone (`git log <prev> --format=%ae`, email membership), so it works offline and identically across platforms. The remote tier keeps today's fetch paths (`gh api` / `glab api` / Azure `net/http`) but normalizes each into a flat `PullRequest{Number,URL,Title,Labels,AuthorLogin,RefPrefix,Platforms}`. The generator merges both and passes a `[]Contributor` plus the PR map to the (unchanged fat-injection) renderer.

**Tech Stack:** Go, `text/template`, `internal/generators/native`, MockRunner (`forge/exec/exectest`), `httptest.Server` (Azure), golden snapshots under `internal/generators/native/testdata`.

## Global Constraints

- TDD: failing test first, then implementation. Every code change ships with tests.
- Layer rule: `internal/generators/native` may import only `internal/{port,config,conventionalcommit}` — **never** `internal/versioning/tagfmt` or `internal/platforms`.
- No new Go dependencies (stdlib + existing only).
- Determinism: no real network. Contract tests use `MockRunner`; Azure uses `httptest.Server`.
- Wrap every returned error: `fmt.Errorf("doing X: %w", err)`.
- Lint via `hk fix` (never `gofmt` directly). Commit-msg is validated by `heraut commit verify` (conventional commits, subject ≤72 chars).
- Commit trailer when Claude authored: `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`. Never add a `Claude-Session:` line.
- The build + full suite (`go test ./...`) must be green at the end of every task.
- Spec: `docs/superpowers/specs/2026-07-03-unified-enrichment-model-design.md`.

---

## File Structure

- Create: `internal/generators/native/model.go` — `Author`, `PullRequest`, `Contributor` structs (the normalized schema).
- Create: `internal/generators/native/contributors.go` — local tier: `authorsBefore`, `collectContributors`.
- Create: `internal/generators/native/contributors_internal_test.go` — local tier tests.
- Create: `docs/adr/0036-unified-enrichment-model.md` — the ADR.
- Modify: `internal/generators/native/enrich_github.go` — return `PullRequest`; +`title`/`labels`; −`authorAssociation`.
- Modify: `internal/generators/native/enrich_gitlab.go` — return `PullRequest`; +`title`/`labels`; delete `markGitLabFirstTimers`/`gitLabEarliestMergedMR` (T137).
- Modify: `internal/generators/native/enrich_azure.go` — return `PullRequest`; +`title`; labels best-effort.
- Modify: `internal/generators/native/enrich.go` — the `map[string]prInfo` return type becomes `map[string]PullRequest`.
- Modify: `internal/generators/native/render.go` — `prInfo`→`PullRequest`; the render contributors helper consumes `[]Contributor`.
- Modify: `internal/generators/native/generator.go` — compute `authorsBefore` + `collectContributors`, pass `[]Contributor` into the renderer.
- Modify: the `*_internal_test.go` files for the above; re-baseline goldens under `internal/generators/native/testdata`.
- Modify: `docs/tasks/native-generator-roadmap.md` — Phase 2.7 tasks + completion notes.
- Modify: `docs/specs/05-generators-and-platforms.md` — note the model / first_time source.

`prInfo` currently lives in `enrich_github.go`. Task 1 moves it to `model.go`, renames it to `PullRequest`, adds `Title`/`Labels`/`Platforms`, and keeps `FirstTimer` **temporarily** so the rename stays behavior-neutral. `FirstTimer` is deleted in Task 4 once the render no longer reads it.

---

### Task 1: Rename `prInfo` → `PullRequest` in a new `model.go` (behavior-neutral)

Mechanical rename + additive fields. No behavior change; all existing tests keep passing.

**Files:**
- Create: `internal/generators/native/model.go`
- Modify: `internal/generators/native/enrich_github.go` (remove the `prInfo` type decl; keep everything else)
- Modify: `internal/generators/native/enrich_gitlab.go`, `enrich_azure.go`, `enrich.go`, `render.go` (type references `prInfo` → `PullRequest`)
- Test: existing `*_internal_test.go` (references `prInfo{...}` → `PullRequest{...}`)

**Interfaces:**
- Produces: `PullRequest struct { Number int; URL string; AuthorLogin string; FirstTimer bool; RefPrefix string; Title string; Labels []string; Platforms map[string]any }`

- [ ] **Step 1: Create `model.go` with the renamed struct**

```go
package native

// Author is a contributor identity: git-first (Name/Email always present), with an optional
// platform handle resolved from remote enrichment. Email is the first_time identity key.
type Author struct {
	Name     string
	Email    string
	Username string // platform handle, e.g. "octocat"; empty offline
}

// PullRequest is the normalized PR/MR for a commit: flat common fields plus a per-platform
// escape hatch. RefPrefix is "#" (GitHub/Azure) or "!" (GitLab); it is derived at fetch time.
type PullRequest struct {
	Number      int
	URL         string
	AuthorLogin string // PR author handle (drives "by @login")
	FirstTimer  bool   // TRANSITIONAL — removed in Task 4 once the local tier owns first_time
	RefPrefix   string
	Title       string
	Labels      []string
	Platforms   map[string]any
}

// Contributor is a per-release contributor for the "New Contributors" block.
type Contributor struct {
	Author      Author
	IsFirstTime bool
	PR          *PullRequest // their first PR in this release; nil offline
}
```

- [ ] **Step 2: Delete the old `prInfo` type from `enrich_github.go`**

Remove this block from `enrich_github.go` (it now lives in `model.go` as `PullRequest`):

```go
// prInfo holds enrichment data for the pull request associated with a commit.
type prInfo struct {
	Number      int
	URL         string
	AuthorLogin string
	FirstTimer  bool
	RefPrefix   string
}
```

- [ ] **Step 3: Replace every `prInfo` reference with `PullRequest`**

Run: `cd /Users/bchatard/Developer/Adaouat/heraut && grep -rl 'prInfo' internal/generators/native/`
Then in each listed file (production and test), replace the identifier `prInfo` with `PullRequest` (type literals `prInfo{...}` → `PullRequest{...}`, map types `map[string]prInfo` → `map[string]PullRequest`, and the `prRef(pr prInfo)` param type). Do not change field names or values.

- [ ] **Step 4: Build and run the full native suite**

Run: `go build ./... && go test ./internal/generators/native/`
Expected: PASS (behavior-neutral rename; `Title`/`Labels`/`Platforms` are unused zero-values for now).

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/
git commit -m "refactor(generators/native): rename prInfo to PullRequest model

Move the enrichment struct to model.go as PullRequest, add Title/Labels/
Platforms (unused for now) and the Author/Contributor types. Behaviour-
neutral rename; FirstTimer kept transitionally.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: Local tier — `authorsBefore` + `collectContributors`

Add the git-only local tier. Additive (new functions + tests); nothing calls it yet.

**Files:**
- Create: `internal/generators/native/contributors.go`
- Test: `internal/generators/native/contributors_internal_test.go`

**Interfaces:**
- Consumes: `parsedCommit{ raw rawCommit }` where `rawCommit{ Hash, Author, Email string; Date time.Time; Subject, Body string }`; `PullRequest` (Task 1); `port.Runner`.
- Produces:
  - `func authorsBefore(runner port.Runner, prev string) (map[string]bool, error)` — set of author emails reachable from `prev`; empty map when `prev == ""`.
  - `func collectContributors(commits []parsedCommit, before map[string]bool, prs map[string]PullRequest) []Contributor` — distinct release authors (first-seen order, deduped by email), `IsFirstTime = email ∉ before`, PR overlaid from the author's first release commit that has one.

- [ ] **Step 1: Write the failing tests**

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/generators/native/ -run 'AuthorsBefore|CollectContributors'`
Expected: FAIL — `undefined: authorsBefore`, `undefined: collectContributors`.

- [ ] **Step 3: Implement `contributors.go`**

```go
package native

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// authorsBefore returns the set of git author emails reachable from prev (git log prev
// --format=%ae). An empty prev (first release) yields an empty set with no git call, so every
// release author counts as new. This is the local tier's first_time source (ADR-0036).
func authorsBefore(runner port.Runner, prev string) (map[string]bool, error) {
	set := make(map[string]bool)
	if prev == "" {
		return set, nil
	}
	stdout, _, err := runner.Run("git", "log", prev, "--format=%ae")
	if err != nil {
		return nil, fmt.Errorf("listing authors before %s: %w", prev, err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(stdout), "\n") {
		if e := strings.TrimSpace(line); e != "" {
			set[e] = true
		}
	}
	return set, nil
}

// collectContributors returns the release's distinct contributors (first-seen order, deduped by
// git author email). IsFirstTime is true when the email is absent from before. When a PR is known
// for the author's first contributing commit, its handle/number/url are overlaid. Only first-time
// contributors are returned — the "New Contributors" block renders exactly this list.
func collectContributors(commits []parsedCommit, before map[string]bool, prs map[string]PullRequest) []Contributor {
	seen := make(map[string]bool)
	var out []Contributor
	for _, c := range commits {
		email := c.raw.Email
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		if before[email] {
			continue // returning contributor
		}
		contrib := Contributor{
			Author:      Author{Name: c.raw.Author, Email: email},
			IsFirstTime: true,
		}
		if pr, ok := prs[c.raw.Hash]; ok {
			contrib.Author.Username = pr.AuthorLogin
			prCopy := pr
			contrib.PR = &prCopy
		}
		out = append(out, contrib)
	}
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/generators/native/ -run 'AuthorsBefore|CollectContributors'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/contributors.go internal/generators/native/contributors_internal_test.go
git commit -m "feat(generators/native): local git tier for contributors + first_time

authorsBefore (git log <prev> --format=%ae) + collectContributors compute
first-timers from git author email membership, independent of remote data.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: Render the New Contributors block from `[]Contributor`

Switch the renderer and generator to the git-based contributors. The old
`prInfo.FirstTimer`-scanning helper is removed; `FirstTimer` becomes dead (removed in Task 4).

**Files:**
- Modify: `internal/generators/native/render.go` (drop the old contributors helper; `buildNotesView` + `renderReleaseNotes` take `[]Contributor`)
- Modify: `internal/generators/native/generator.go` (`generateReleaseNotes` + `renderRelease` compute `authorsBefore` + `collectContributors`, pass the result down)
- Test: `internal/generators/native/enrich_internal_test.go` (New Contributors assertions), plus goldens

**Interfaces:**
- Consumes: `collectContributors`, `authorsBefore` (Task 2).
- Produces: `renderReleaseNotes(version, previousVersion string, releaseDate time.Time, groups []group, lc *port.LinkContext, tickets []config.Ticket, prevReleaseDate time.Time, typesHeadingLevel int, prs map[string]PullRequest, contributors []Contributor) (string, error)` — note the two new trailing params replace the enrichment-scanning behavior.

- [ ] **Step 1: Update the New Contributors test to expect git-based membership**

In `internal/generators/native/enrich_internal_test.go`, the existing `TestRenderReleaseNotes_NewContributors` calls `renderReleaseNotes(...)` and relies on `prInfo.FirstTimer`. Replace it with a version that passes an explicit `[]Contributor`:

```go
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
	got, err := renderReleaseNotes("v1.0.0", "", fixedDate1, groups, githubLC, nil, time.Time{}, 3, prs, contributors)
	require.NoError(t, err)

	assert.Contains(t, got, "### New Contributors ❤️")
	assert.Contains(t, got, "* @newbie made their first contribution in [#7](https://github.com/o/r/pull/7)")
	assert.Contains(t, got, "by @newbie in [#7]")
}
```

Also update `TestRenderReleaseNotes_NoFirstTimers_NoBlock` to pass `nil` for `contributors` and keep asserting the block is absent.

- [ ] **Step 2: Run to verify it fails to compile**

Run: `go test ./internal/generators/native/ -run 'NewContributors'`
Expected: FAIL — `renderReleaseNotes` signature mismatch (too many arguments) / old helper still referenced.

- [ ] **Step 3: Rewrite the render-side contributors path**

In `render.go`, delete the old `buildContributors(commits []parsedCommit, enrichment map[string]PullRequest) []contributorView` function. Add a builder from `[]Contributor`:

```go
// buildContributorViews renders the New Contributors lines from the local-tier contributors.
// Online, a contributor carries a Username (and usually a PR) → "* @user made their first
// contribution in [#N](url)"; offline the block is empty (no Username) so the template omits it.
func buildContributorViews(contributors []Contributor) []contributorView {
	var out []contributorView
	for _, c := range contributors {
		if c.Author.Username == "" {
			continue // built-in template shows the block only with a remote handle (ADR-0036)
		}
		line := "* @" + c.Author.Username + " made their first contribution"
		if c.PR != nil && c.PR.Number > 0 {
			line += fmt.Sprintf(" in [%s](%s)", prRef(*c.PR), c.PR.URL)
		}
		out = append(out, contributorView{Line: line})
	}
	return out
}
```

Change `buildNotesView` and `renderReleaseNotes` to accept `prs map[string]PullRequest` (the rename of the `enrichment` param) **and** `contributors []Contributor`, and set `Contributors: buildContributorViews(contributors)` in the returned `notesView`. Keep `buildCommitLine`/`buildCommitBlock` reading `prs` exactly as before.

- [ ] **Step 4: Wire the generator to compute contributors (release-notes only)**

The contributors block is **release-notes only** — `renderChangelogSection` has no contributors block, so **do not** touch `renderRelease`/`renderChangelogSection`. Only `generateReleaseNotes` changes. After `enrichForRelease` returns the PR map, add the local-tier calls:

```go
before, err := authorsBefore(g.runner, prev)
if err != nil {
	return "", err
}
groups := groupCommits(commits, g.cfg.Types, g.cfg.Excludes)
contributors := collectContributors(commits, before, enrichment)
return renderReleaseNotes(tag, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, prevDate, g.cfg.TypesHeadingLevel, enrichment, contributors)
```

**This adds a new git call** — `authorsBefore` runs `git log <prev> --format=%ae` on every release-notes generation with a predecessor (skipped when `prev == ""`). It executes *after* `enrichForRelease`, so the runner call order for a release-notes run is: `previousTag` → `tagDate` (T140) → `collectCommits` → *(platform enrich)* → `authorsBefore`.

- [ ] **Step 5: Fix the release-notes contract tests for the new `authorsBefore` call**

Exactly like the T140 `tagDate` ripple, every `MockRunner`-based release-notes test that reaches `generateReleaseNotes` with a non-empty `prev` needs one more queued response (the `git log <prev> --format=%ae`) at the end of its git sequence, plus updated `mr.Calls` length/index assertions. Affected tests (search `ModeReleaseNotes` in the package):

- `generator_internal_test.go`: `TestGenerator_GenerateReleaseNotes`, `_TagGlob`, `_TagPatternRegex`, `_DaysBetweenReleases`.
- `enrich_internal_test.go`: the `queueReleaseNotesGit` helper (append one `mr.QueueResponse("bob@x\n", "", nil) // authorsBefore` line — fixes all its users at once).
- `enrich_gitlab_internal_test.go` `TestGenerate_Enrich_GitLab`, `enrich_azure_internal_test.go` `TestGenerate_Enrich_Azure`: append the `authorsBefore` response after the enrich responses.

For each, add `mr.QueueResponse("<some-email>\n", "", nil) // authorsBefore: git log <prev> --format=%ae` as the LAST git response, bump the expected `mr.Calls` length by one, and assert the final call is `[]string{"log", "<prev>", "--format=%ae"}`. Pick an email that is NOT the release commit's author when you want the release author to remain a first-timer, or one that IS to exercise the returning-contributor path.

Run: `go test ./internal/generators/native/ -run 'NewContributors'` → Expected: PASS.

- [ ] **Step 6: Re-baseline goldens, then full suite**

Run: `go test ./internal/generators/native/` — if golden snapshot tests fail because online New-Contributors membership shifted (git-based vs `authorAssociation`), inspect the diff, confirm it is the intended change, and update the golden files under `internal/generators/native/testdata` (regenerate via the package's golden-update flag if one exists — grep the test files for `-update` / an `update` flag var; otherwise hand-edit the fixture to the reviewed output). Then:

Run: `go test ./...` → Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/generators/native/
git commit -m "feat(generators/native): render New Contributors from git-based local tier

The New Contributors block now derives from collectContributors (git email
first-appearance) instead of prInfo.FirstTimer. Built-in template shows the
block only with a remote handle; offline output unchanged.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: Delete `FirstTimer` — drop GitHub `authorAssociation` and GitLab T137

`FirstTimer` is now dead. Remove it and the two per-platform mechanisms that set it.

**Files:**
- Modify: `internal/generators/native/model.go` (remove `FirstTimer` field)
- Modify: `internal/generators/native/enrich_github.go` (drop `authorAssociation` from the GraphQL fragment + parsing)
- Modify: `internal/generators/native/enrich_gitlab.go` (delete `markGitLabFirstTimers`, `gitLabEarliestMergedMR`; the enrich func stops calling them)
- Test: `enrich_github_internal_test.go`, `enrich_gitlab_internal_test.go`

**Interfaces:**
- Produces: `PullRequest` without `FirstTimer`; `enrichGitLab` no longer issues per-author `merge_requests?author_username=…` calls.

- [ ] **Step 1: Update GitLab tests to drop the earliest-MR calls**

In `enrich_gitlab_internal_test.go`, delete `TestEnrichGitLab_FirstTimer`, `TestEnrichGitLab_ReturningContributor`, `TestEnrichGitLab_FirstTimer_DistinctAuthorQueriedOnce`, `TestEnrichGitLab_FirstTimer_MultiAuthor`, and `TestEnrichGitLab_FirstTimerQueryError`. In the remaining tests (`TestEnrichGitLab_MapsMR`, `_Subgroup`, `TestGenerate_Enrich_GitLab`), delete the queued earliest-MR responses (`mr.QueueResponse(\`[{"iid":...}]\`, ...)` lines commented as first-timer/earliest) and reduce the expected `mr.Calls` length to just the per-commit MR fetches. `TestGenerate_Enrich_GitLab` keeps its `by @alice in [!7]` assertion but drops the New-Contributors assertions (those move to git-based coverage in Task 2/3).

- [ ] **Step 2: Update GitHub test to drop authorAssociation/FirstTimer**

In `enrich_github_internal_test.go`, remove the `authorAssociation` fields from the canned GraphQL JSON and delete the `assert.*FirstTimer*` assertions. The `buildGitHubQuery` assertion will change once the fragment changes in Step 4 — update it to match the new fragment string.

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/generators/native/ -run 'GitLab|GitHub'`
Expected: FAIL — deleted funcs still referenced by production code / `FirstTimer` still set.

- [ ] **Step 4: Remove the code**

- In `model.go`, delete the `FirstTimer bool` line from `PullRequest`.
- In `enrich_github.go`: change `prFragment` to drop `authorAssociation`:
  ```go
  prFragment = "...on Commit{associatedPullRequests(first:1){nodes{number url title author{login}}}}"
  ```
  Remove `AuthorAssociation string \`json:"authorAssociation"\`` from `graphQLPR` and delete `FirstTimer: pr.AuthorAssociation == "FIRST_TIME_CONTRIBUTOR"` from the `PullRequest{...}` literal in `parseGitHubResponse`.
- In `enrich_gitlab.go`: delete `markGitLabFirstTimers` and `gitLabEarliestMergedMR` entirely, and remove the `if err := markGitLabFirstTimers(...); err != nil { return nil, err }` call from `enrichGitLab`. Drop the now-unused `sort` import.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/generators/native/` → Expected: PASS.
Run: `go test ./...` → Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/generators/native/
git commit -m "refactor(generators/native): remove per-platform first-timer paths

Delete PullRequest.FirstTimer, GitHub authorAssociation parsing, and the
GitLab T137 earliest-MR queries. first_time is now the local git tier only.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 5: Fetch `title` + `labels` — GitHub

**Files:**
- Modify: `internal/generators/native/enrich_github.go`
- Test: `internal/generators/native/enrich_github_internal_test.go`

**Interfaces:**
- Produces: GitHub `PullRequest` with `Title` and `Labels` populated.

- [ ] **Step 1: Write the failing test**

Add to `enrich_github_internal_test.go` a test that the canned response's title/labels land on the `PullRequest`:

```go
func TestEnrichGitHub_TitleAndLabels(t *testing.T) {
	mr := exectest.NewMockRunner()
	sha := "aa11bb22cc33dd44ee55ff6677889900aabbccdd"
	lc := makeGitHubLC("owner", "repo", "tok")
	mr.QueueResponse(`{"data":{"repository":{"s0":{"associatedPullRequests":{"nodes":[
		{"number":42,"url":"u","title":"Add OAuth","author":{"login":"alice"},
		 "labels":{"nodes":[{"name":"enhancement"},{"name":"area/auth"}]}}]}}}}}`, "", nil)

	got, err := enrichGitHub(mr, lc, []string{sha})
	require.NoError(t, err)
	assert.Equal(t, "Add OAuth", got[sha].Title)
	assert.Equal(t, []string{"enhancement", "area/auth"}, got[sha].Labels)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/generators/native/ -run 'TitleAndLabels'`
Expected: FAIL — `Title`/`Labels` empty (fragment doesn't request them).

- [ ] **Step 3: Implement**

- Extend `prFragment` to request title + labels:
  ```go
  prFragment = "...on Commit{associatedPullRequests(first:1){nodes{number url title author{login}labels(first:20){nodes{name}}}}}"
  ```
- Extend `graphQLPR` and parsing:
  ```go
  type graphQLPR struct {
  	Number int    `json:"number"`
  	URL    string `json:"url"`
  	Title  string `json:"title"`
  	Author struct {
  		Login string `json:"login"`
  	} `json:"author"`
  	Labels struct {
  		Nodes []struct {
  			Name string `json:"name"`
  		} `json:"nodes"`
  	} `json:"labels"`
  }
  ```
  In `parseGitHubResponse`, set `Title: pr.Title` and build `Labels` from `pr.Labels.Nodes`.
- Update the `buildGitHubQuery` argv assertion in the existing test to the new `prFragment`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/generators/native/ -run 'GitHub'` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich_github.go internal/generators/native/enrich_github_internal_test.go
git commit -m "feat(generators/native): fetch PR title + labels for GitHub

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6: Fetch `title` + `labels` — GitLab

**Files:**
- Modify: `internal/generators/native/enrich_gitlab.go`
- Test: `internal/generators/native/enrich_gitlab_internal_test.go`

**Interfaces:**
- Produces: GitLab `PullRequest` with `Title` and `Labels`.

- [ ] **Step 1: Write the failing test**

```go
func TestEnrichGitLab_TitleAndLabels(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"u","title":"Add OAuth","author":{"username":"alice"},"labels":["enhancement","area/auth"]}]`, "", nil)

	got, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	assert.Equal(t, "Add OAuth", got["abc123"].Title)
	assert.Equal(t, []string{"enhancement", "area/auth"}, got["abc123"].Labels)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/generators/native/ -run 'GitLab_TitleAndLabels'`
Expected: FAIL — fields empty.

- [ ] **Step 3: Implement**

Extend `gitLabMR` and the mapping in `enrichGitLab`:

```go
type gitLabMR struct {
	IID    int      `json:"iid"`
	WebURL string   `json:"web_url"`
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
}
```

In the `result[sha] = PullRequest{...}` literal, add `Title: mr.Title` and `Labels: mr.Labels`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/generators/native/ -run 'GitLab'` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich_gitlab.go internal/generators/native/enrich_gitlab_internal_test.go
git commit -m "feat(generators/native): fetch MR title + labels for GitLab

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 7: Fetch `title` (+ labels best-effort) — Azure

**Files:**
- Modify: `internal/generators/native/enrich_azure.go`
- Test: `internal/generators/native/enrich_azure_internal_test.go`

**Interfaces:**
- Produces: Azure `PullRequest` with `Title` (and `Labels` when present in the response).

- [ ] **Step 1: Write the failing test**

```go
func TestEnrichAzure_TitleAndLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"results":[{"abc123":[{"pullRequestId":42,"title":"Add OAuth",
			"createdBy":{"displayName":"Jane","uniqueName":"jane@x"},
			"labels":[{"name":"enhancement"},{"name":"area/auth"}]}]}]}`)
	}))
	defer srv.Close()

	got, err := enrichAzure(srv.Client(), azureLC(srv.URL), []string{"abc123"})
	require.NoError(t, err)
	assert.Equal(t, "Add OAuth", got["abc123"].Title)
	assert.Equal(t, []string{"enhancement", "area/auth"}, got["abc123"].Labels)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/generators/native/ -run 'Azure_TitleAndLabels'`
Expected: FAIL — fields empty.

- [ ] **Step 3: Implement**

Extend `azurePR` and the mapping in `enrichAzure`:

```go
type azurePR struct {
	PullRequestID int              `json:"pullRequestId"`
	Title         string           `json:"title"`
	CreatedBy     azureIdentityRef `json:"createdBy"`
	Labels        []struct {
		Name string `json:"name"`
	} `json:"labels"`
}
```

In the `result[sha] = PullRequest{...}` literal add `Title: pr.Title` and build `Labels` from `pr.Labels` (each `.Name`). Labels stay empty when the response omits them (best-effort — Azure may require an expand; that is acceptable per the spec).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/generators/native/ -run 'Azure'` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich_azure.go internal/generators/native/enrich_azure_internal_test.go
git commit -m "feat(generators/native): fetch PR title + labels for Azure DevOps

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 8: ADR, spec, and roadmap

Document the model and close the phase.

**Files:**
- Create: `docs/adr/0036-unified-enrichment-model.md`
- Modify: `docs/adr/README.md` (index row)
- Modify: `docs/specs/05-generators-and-platforms.md` (note the model + first_time source)
- Modify: `docs/tasks/native-generator-roadmap.md` (Phase 2.7 tasks marked `[x]` + notes; progress table row)

- [ ] **Step 1: Write ADR-0036**

Create `docs/adr/0036-unified-enrichment-model.md` recording: the two-tier model, the flat schema (`Author`/`PullRequest`/`Contributor`), `first_time` as a local git-email computation (superseding the `authorAssociation` [ADR-0034] and T137/T129 first-timer approaches), `title`/`labels` common + `Platforms.<x>` escape hatch, the contained rendering behavior (offline built-in output unchanged), and the fat-injection→exposed-model bridge to the future templates task. Reference the spec `docs/superpowers/specs/2026-07-03-unified-enrichment-model-design.md`.

- [ ] **Step 2: Update the ADR index + spec 05**

Add the `0036` row to `docs/adr/README.md`. In `docs/specs/05-generators-and-platforms.md`, update the native paragraph to state that PR/author attribution + New Contributors derive from a normalized model with `first_time` computed from git history (available offline).

- [ ] **Step 3: Update the roadmap**

In `docs/tasks/native-generator-roadmap.md`, add a `### Phase 2.7 — Unified enrichment model` section with the tasks from this plan marked `[x]` and a one-paragraph completion note each; add the progress-table row; and note that **T141 is resolved by the local tier** (Azure New Contributors now works via git-based first_time, no per-author API).

- [ ] **Step 4: Lint the docs**

Run: `git add -A && hk fix`
Expected: `typos`, `go_fmt`, `golangci_lint` all pass.

- [ ] **Step 5: Full suite + commit**

Run: `go test ./...` → Expected: PASS.

```bash
git add -A
git commit -m "docs(adr): 0036 unified enrichment model; close Phase 2.7

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Notes for the implementer

- **Golden snapshots (Task 3):** the only place output legitimately changes online is New-Contributors membership (git-based vs `authorAssociation`). Review that diff before re-baselining; do not blanket-accept.
- **Correlation:** `collectContributors` overlays the PR from *the author's first contributing commit that has a PR*. A first-timer with no PR (offline, or a direct push) still appears in the model but is skipped by the built-in template (`Username == ""`).
- **`RefPrefix`:** unchanged per platform — `#` GitHub/Azure, `!` GitLab — set at fetch time in each `enrich_*`.
- **No config/schema changes:** the model is internal + the future template contract; `remote_metadata` is untouched.
- **`Platforms` field:** defined as part of the schema but left **empty** by this plan — there is no consumer until user-customizable templates land. Populating platform-unique fields (`Platforms["github"] = {draft, review_decision, …}`, etc.) is deliberately deferred, not a gap.
