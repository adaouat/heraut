# User-Customizable Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the native generator a public template API — inline `rendering.templates.<block>` snippets and a `<driver>.template` file — over a documented Go `text/template` data contract, with the built-in templates rewritten onto that same contract (dogfooded).

**Architecture:** The built-in `changelog.tmpl` / `release_notes.tmpl` become named-block templates that consume a structured **template model** (`tplRelease`/`tplGroup`/`tplCommit`/…) built by `buildRelease`/`buildChangelog` from the collected commits + enrichment + stats. User inline snippets and the `template` file are parsed on top of the built-in blocks (last definition wins). The fat-injection line-builders in `render.go` become template blocks + a small func map. Output stays byte-identical (golden snapshots re-baselined).

**Tech Stack:** Go `text/template` (existing engine), `internal/generators/native`, `internal/config` (Rendering/ContentDriver + merge + validator), `internal/app` (withEnvDerivations), MockRunner/httptest, golden snapshots under `internal/generators/native/testdata`.

**Design spec:** `docs/superpowers/specs/2026-07-04-user-customizable-templates-design.md`.

## Global Constraints

- TDD: failing test first, then implementation. Every code change ships tests.
- Layer rule: `internal/generators/native` imports only `internal/{port,config,conventionalcommit}` + stdlib. `internal/config` imports nothing from heraut. No new Go dependencies.
- Wrap errors with `fmt.Errorf("…: %w", err)`.
- All `.PR.*` fields are **remote-only** (empty offline). `Approvers` is **best-effort**: populated on GitHub + Azure, **empty on GitLab** (no extra `/approvals` call).
- **Built-in output must stay byte-identical.** Golden snapshots under `internal/generators/native/testdata` are re-baselined only after confirming the diff is the intended one (for this feature: empty).
- Lint via `hk fix` (never gofmt/markdown tools directly). Never bypass hooks. Conventional-commit subject ≤72 chars.
- Commit trailer (subagent): `Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>`. Never a `Claude-Session:` line.
- Config field changes sync `schema.json` + `docs/heraut.sample.yml`.
- Full suite (`go test ./...`) green at the end of every task.

---

## File Structure

- Modify `internal/generators/native/model.go` — enrichment `PullRequest` gains `CreatedAt`, `MergedAt`, `MergedBy`, `Approvers`.
- Modify `internal/generators/native/enrich_github.go` / `enrich_gitlab.go` / `enrich_azure.go` — populate the new PR fields (best-effort per the table).
- Create `internal/generators/native/templatemodel.go` — the public template model structs (`tplChangelog`, `tplRelease`, `tplGroup`, `tplCommit`, `tplPR`, `tplHeraut`, `tplStats`, `tplLink`, `tplStatTicket`, `tplFooter`) + the `buildRelease` / `buildChangelog` builders.
- Create `internal/generators/native/funcs.go` — the template func map (`upperFirst`, `date`, `join`, `indent`, `trim`).
- Create `internal/generators/native/templateset.go` — assemble the block template set from built-in + inline snippets + file, in precedence order.
- Modify `internal/generators/native/render.go` — `renderChangelogSection`/`renderReleaseNotes` build the model + execute the block set; remove the fat-injection view-model + line-builders.
- Modify `internal/generators/native/changelog.tmpl` / `release_notes.tmpl` — named-block, model-driven templates.
- Modify `internal/generators/native/generator.go` — thread the injected clock + heraut version + user template config into the render calls.
- Modify `internal/config/config.go` — `Rendering.Templates map[string]string`; `ContentDriver.Rendering *Rendering`; `ContentDriver.Template` already exists.
- Modify `internal/config/merge.go` — merge `Rendering` (templates + excludes) per-driver/per-env.
- Modify `internal/config/validator.go` — `rendering.templates`/`template` require `generator: native`; snippets + file parse; file exists.
- Modify `internal/app/pipeline.go` — `withEnvDerivations` propagates effective templates + file onto the driver.
- Modify `schema.json`, `docs/heraut.sample.yml`, `docs/specs/05-generators-and-platforms.md`; create `docs/adr/0037-native-template-api.md`.

`ContentDriver` already has `Template string \`yaml:"template,omitempty"\`` (currently inert) and app-computed fields (`Types`, `Excludes`, `TagGlob`, …). This plan gives `Template` (file) meaning and adds effective template snippets.

---

### Task 1: Extend the enrichment `PullRequest` with PR dates / mergedBy / approvers

Additive struct fields; nothing populates or reads them yet.

**Files:**
- Modify: `internal/generators/native/model.go`
- Test: `internal/generators/native/model_internal_test.go` (create — a compile/shape guard)

**Interfaces:**
- Produces: `PullRequest` gains `CreatedAt time.Time`, `MergedAt time.Time`, `MergedBy Author`, `Approvers []Author`.

- [ ] **Step 1: Write the failing test**

```go
package native

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_HasReviewFields(t *testing.T) {
	pr := PullRequest{
		Number:    1,
		CreatedAt: time.Unix(100, 0),
		MergedAt:  time.Unix(200, 0),
		MergedBy:  Author{Username: "maintainer"},
		Approvers: []Author{{Username: "alice"}, {Username: "bob"}},
	}
	assert.Equal(t, "maintainer", pr.MergedBy.Username)
	assert.Len(t, pr.Approvers, 2)
	assert.False(t, pr.CreatedAt.IsZero())
	assert.False(t, pr.MergedAt.IsZero())
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'PullRequest_HasReviewFields'`
Expected: FAIL — `unknown field CreatedAt/MergedAt/MergedBy/Approvers in struct literal`.

- [ ] **Step 3: Add the fields to `PullRequest`**

In `model.go`, add to the `PullRequest` struct (after `Platforms`), and ensure `time` is imported:

```go
	// CreatedAt / MergedAt are the PR/MR creation and merge timestamps (remote-only, zero offline).
	CreatedAt time.Time
	MergedAt  time.Time
	// MergedBy is the actor who merged the PR/MR (remote-only; empty Author when unknown).
	MergedBy Author
	// Approvers are the reviewers who approved (best-effort: GitHub + Azure; empty on GitLab).
	Approvers []Author
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run 'PullRequest_HasReviewFields'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/model.go internal/generators/native/model_internal_test.go
git commit -m "feat(generators/native): add PR review fields to the model

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Populate PR review fields — GitHub

**Files:**
- Modify: `internal/generators/native/enrich_github.go`
- Test: `internal/generators/native/enrich_github_internal_test.go`

**Interfaces:**
- Consumes: `PullRequest` fields from Task 1.
- Produces: GitHub `PullRequest` with `CreatedAt`, `MergedAt`, `MergedBy`, `Approvers` populated.

- [ ] **Step 1: Write the failing test**

```go
func TestEnrichGitHub_ReviewFields(t *testing.T) {
	mr := exectest.NewMockRunner()
	sha := "aa11bb22cc33dd44ee55ff6677889900aabbccdd"
	lc := makeGitHubLC("owner", "repo", "tok")
	mr.QueueResponse(`{"data":{"repository":{"s0":{"associatedPullRequests":{"nodes":[
		{"number":42,"url":"u","title":"t","author":{"login":"alice"},
		 "createdAt":"2026-01-01T00:00:00Z","mergedAt":"2026-01-02T00:00:00Z",
		 "mergedBy":{"login":"maint"},
		 "labels":{"nodes":[]},
		 "latestReviews":{"nodes":[{"state":"APPROVED","author":{"login":"rev1"}},
		                           {"state":"CHANGES_REQUESTED","author":{"login":"rev2"}}]}}]}}}}}`, "", nil)

	got, err := enrichGitHub(mr, lc, []string{sha})
	require.NoError(t, err)
	pr := got[sha]
	assert.Equal(t, "2026-01-01T00:00:00Z", pr.CreatedAt.UTC().Format(time.RFC3339))
	assert.Equal(t, "2026-01-02T00:00:00Z", pr.MergedAt.UTC().Format(time.RFC3339))
	assert.Equal(t, "maint", pr.MergedBy.Username)
	require.Len(t, pr.Approvers, 1, "only APPROVED reviews count")
	assert.Equal(t, "rev1", pr.Approvers[0].Username)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'GitHub_ReviewFields'`
Expected: FAIL — fields zero (fragment doesn't request them).

- [ ] **Step 3: Extend the fragment, structs, and parse**

- Extend `prFragment` to also request the new fields:
  ```go
  prFragment = "...on Commit{associatedPullRequests(first:1){nodes{number url title author{login}labels(first:20){nodes{name}}createdAt mergedAt mergedBy{login}latestReviews(first:20){nodes{state author{login}}}}}}"
  ```
- Extend `graphQLPR`:
  ```go
  type graphQLPR struct {
  	Number    int       `json:"number"`
  	URL       string    `json:"url"`
  	Title     string    `json:"title"`
  	Author    struct{ Login string `json:"login"` } `json:"author"`
  	Labels    struct{ Nodes []struct{ Name string `json:"name"` } `json:"nodes"` } `json:"labels"`
  	CreatedAt time.Time `json:"createdAt"`
  	MergedAt  time.Time `json:"mergedAt"`
  	MergedBy  struct{ Login string `json:"login"` } `json:"mergedBy"`
  	LatestReviews struct {
  		Nodes []struct {
  			State  string `json:"state"`
  			Author struct{ Login string `json:"login"` } `json:"author"`
  		} `json:"nodes"`
  	} `json:"latestReviews"`
  }
  ```
- In `parseGitHubResponse`, when building the `PullRequest{...}` add:
  ```go
  CreatedAt: pr.CreatedAt,
  MergedAt:  pr.MergedAt,
  MergedBy:  Author{Username: pr.MergedBy.Login},
  ```
  and after constructing it, set approvers:
  ```go
  var approvers []Author
  for _, r := range pr.LatestReviews.Nodes {
  	if r.State == "APPROVED" && r.Author.Login != "" {
  		approvers = append(approvers, Author{Username: r.Author.Login})
  	}
  }
  ```
  assign `Approvers: approvers` in the literal (or set the field before inserting into the map).
- Update the existing `buildGitHubQuery` argv assertion to the new `prFragment`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run 'GitHub'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich_github.go internal/generators/native/enrich_github_internal_test.go
git commit -m "feat(generators/native): fetch PR dates/mergedBy/approvers (GitHub)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Populate PR review fields — GitLab (approvers stay empty)

**Files:**
- Modify: `internal/generators/native/enrich_gitlab.go`
- Test: `internal/generators/native/enrich_gitlab_internal_test.go`

**Interfaces:**
- Produces: GitLab `PullRequest` with `CreatedAt`, `MergedAt`, `MergedBy` populated; `Approvers` nil.

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'GitLab_ReviewFields'`
Expected: FAIL — fields zero.

- [ ] **Step 3: Extend `gitLabMR` and the mapping**

```go
type gitLabMR struct {
	IID       int      `json:"iid"`
	WebURL    string   `json:"web_url"`
	Title     string   `json:"title"`
	Labels    []string `json:"labels"`
	Author    struct{ Username string `json:"username"` } `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	MergedAt  time.Time `json:"merged_at"`
	MergedBy  struct{ Username string `json:"username"` } `json:"merged_by"`
}
```
In `enrichGitLab`'s `PullRequest{...}` literal add:
```go
CreatedAt: mr.CreatedAt,
MergedAt:  mr.MergedAt,
MergedBy:  Author{Username: mr.MergedBy.Username},
// Approvers intentionally left nil — the per-commit MR object has no approvers; a separate
// /approvals call per MR is not paid (ADR / spec: best-effort).
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run 'GitLab'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich_gitlab.go internal/generators/native/enrich_gitlab_internal_test.go
git commit -m "feat(generators/native): fetch MR dates/mergedBy (GitLab)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Populate PR review fields — Azure

**Files:**
- Modify: `internal/generators/native/enrich_azure.go`
- Test: `internal/generators/native/enrich_azure_internal_test.go`

**Interfaces:**
- Produces: Azure `PullRequest` with `CreatedAt`, `MergedAt`, `MergedBy`, `Approvers` (reviewers with vote ≥ 10).

- [ ] **Step 1: Write the failing test**

```go
func TestEnrichAzure_ReviewFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"results":[{"abc123":[{"pullRequestId":42,"title":"t",
			"createdBy":{"displayName":"Jane","uniqueName":"jane@x"},
			"creationDate":"2026-01-01T00:00:00Z","closedDate":"2026-01-02T00:00:00Z",
			"closedBy":{"displayName":"Maint","uniqueName":"maint@x"},
			"reviewers":[{"uniqueName":"rev1@x","vote":10},{"uniqueName":"rev2@x","vote":-10}]}]}]}`)
	}))
	defer srv.Close()

	got, err := enrichAzure(srv.Client(), azureLC(srv.URL), []string{"abc123"})
	require.NoError(t, err)
	pr := got["abc123"]
	assert.Equal(t, "2026-01-01T00:00:00Z", pr.CreatedAt.UTC().Format(time.RFC3339))
	assert.Equal(t, "2026-01-02T00:00:00Z", pr.MergedAt.UTC().Format(time.RFC3339))
	assert.Equal(t, "maint", pr.MergedBy.Username)
	require.Len(t, pr.Approvers, 1, "only vote >= 10 counts as approved")
	assert.Equal(t, "rev1", pr.Approvers[0].Username)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'Azure_ReviewFields'`
Expected: FAIL — fields zero.

- [ ] **Step 3: Extend `azurePR` and the mapping**

Add to `azurePR`:
```go
	CreationDate time.Time        `json:"creationDate"`
	ClosedDate   time.Time        `json:"closedDate"`
	ClosedBy     azureIdentityRef `json:"closedBy"`
	Reviewers    []struct {
		UniqueName  string `json:"uniqueName"`
		DisplayName string `json:"displayName"`
		Vote        int    `json:"vote"`
	} `json:"reviewers"`
```
In the `PullRequest{...}` literal add `CreatedAt: pr.CreationDate`, `MergedAt: pr.ClosedDate`, `MergedBy: Author{Username: azureAuthorLogin(pr.ClosedBy)}`, and build approvers before the literal:
```go
var approvers []Author
for _, r := range pr.Reviewers {
	if r.Vote >= 10 {
		approvers = append(approvers, Author{Username: azureAuthorLogin(azureIdentityRef{DisplayName: r.DisplayName, UniqueName: r.UniqueName})})
	}
}
```
assign `Approvers: approvers`. (Reuse the existing `azureAuthorLogin` helper for the uniqueName→handle rule.)

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run 'Azure'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich_azure.go internal/generators/native/enrich_azure_internal_test.go
git commit -m "feat(generators/native): fetch PR dates/mergedBy/approvers (Azure)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 5: The public template model structs

Define the documented contract types templates see. Field names are the public API.

**Files:**
- Create: `internal/generators/native/templatemodel.go`
- Test: `internal/generators/native/templatemodel_internal_test.go`

**Interfaces:**
- Consumes: `Author` (model.go).
- Produces:
  ```go
  type tplChangelog struct { Releases []tplRelease; Heraut tplHeraut }
  type tplRelease struct {
      Version, Tag, PreviousTag, CompareURL, HeadingPrefix string // HeadingPrefix for the contributors/stats headings
      Date time.Time
      Groups []tplGroup; Contributors []tplContributor; Stats tplStats; Heraut tplHeraut
  }
  type tplHeraut struct { Version, URL string; GeneratedAt time.Time }
  type tplGroup struct { Name, HeadingPrefix string; Commits []tplCommit }  // HeadingPrefix = "#"×types_heading_level
  type tplCommit struct {
      Type, Scope string; Breaking bool
      Description, Subject, Body, Hash, ShortHash, CommitURL string
      Date time.Time
      Author Author; PR *tplPR; Tickets []tplLink; Footers []tplFooter
  }
  type tplPR struct {
      Number int; URL, Title, Ref string; Labels []string
      Author Author; CreatedAt, MergedAt time.Time; MergedBy Author; Approvers []Author
  }
  type tplContributor struct { Author Author; PR *tplPR }
  type tplStats struct {
      CommitCount, ConventionalCount, TimespanDays, DaysSincePrev int
      HasDaysSincePrev bool; Tickets []tplStatTicket
  }
  type tplLink struct { Text, Href string }
  type tplStatTicket struct { Text, Href string; Count int }
  type tplFooter struct { Token, Value string }
  ```

- [ ] **Step 1: Write the failing test**

```go
package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateModel_FieldsPresent(t *testing.T) {
	r := tplRelease{
		Version: "1.2.3", Tag: "v1.2.3",
		Groups: []tplGroup{{Name: "Features", Commits: []tplCommit{{
			Description: "x", ShortHash: "abc1234",
			Author: Author{Username: "jane"},
			PR:     &tplPR{Number: 42, Ref: "#42", Author: Author{Username: "jane"}},
			Footers: []tplFooter{{Token: "Refs", Value: "#1"}},
		}}}},
		Contributors: []tplContributor{{Author: Author{Username: "jane"}}},
		Stats:        tplStats{CommitCount: 1},
		Heraut:       tplHeraut{Version: "0.48.0"},
	}
	assert.Equal(t, "#42", r.Groups[0].Commits[0].PR.Ref)
	assert.Equal(t, "Refs", r.Groups[0].Commits[0].Footers[0].Token)
	assert.Equal(t, 1, r.Stats.CommitCount)
	c := tplChangelog{Releases: []tplRelease{r}, Heraut: tplHeraut{Version: "0.48.0"}}
	assert.Len(t, c.Releases, 1)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'TemplateModel_FieldsPresent'`
Expected: FAIL — undefined types.

- [ ] **Step 3: Create `templatemodel.go` with the structs above**

Write the struct definitions from the Interfaces block (with a doc comment on each type noting it is the public template contract — field names are stable/experimental per ADR-0037).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run 'TemplateModel_FieldsPresent'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/templatemodel.go internal/generators/native/templatemodel_internal_test.go
git commit -m "feat(generators/native): public template model structs

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 6: The template-model builder

Map the internal data (grouped commits + enrichment + tickets + stats + prev date + heraut meta) into a `tplRelease`.

**Files:**
- Modify: `internal/generators/native/templatemodel.go` (add `buildRelease`)
- Test: `internal/generators/native/templatemodel_internal_test.go`

**Interfaces:**
- Consumes: `group` (`group.go`: `{ name string; commits []parsedCommit }`), `parsedCommit`/`rawCommit`, `PullRequest`, `tplContributor` inputs from `[]Contributor` (contributors.go), `config.Ticket`, `port.LinkContext`, `buildCommitURL`/`buildCompareURL`/`prRef`/`resolveTickets`/`buildStatTicketLinks` (render.go helpers — reuse), `Author`.
- Produces:
  ```go
  func buildRelease(
      version, previousVersion string, releaseDate, prevReleaseDate time.Time,
      groups []group, lc *port.LinkContext, tickets []config.Ticket,
      typesHeadingLevel int, enrichment map[string]PullRequest, contributors []Contributor,
      heraut tplHeraut,
  ) tplRelease
  ```
  Each `tplGroup.HeadingPrefix` is set to `headingPrefix(typesHeadingLevel)` (the existing
  render.go helper). `buildChangelog` (also in this task) wraps `[]tplRelease` + `tplHeraut`.

- [ ] **Step 1: Write the failing test**

```go
func TestBuildRelease_MapsTree(t *testing.T) {
	pc := parsedFrom("abc1234def", "feat(api): add thing")
	pc.raw.Email = "jane@x"
	pc.raw.Author = "Jane"
	pc.raw.Date = fixedDate1
	groups := []group{{name: "🚀 Features", order: 0, commits: []parsedCommit{pc}}}
	enrichment := map[string]PullRequest{
		"abc1234def": {Number: 42, URL: "https://github.com/o/r/pull/42", AuthorLogin: "jane", RefPrefix: "#", Title: "PR title"},
	}
	contribs := []Contributor{{Author: Author{Name: "Jane", Email: "jane@x", Username: "jane"}, IsFirstTime: true}}

	r := buildRelease("v1.2.3", "v1.2.2", fixedDate1, time.Time{}, groups, githubLC, nil, 3, enrichment, contribs, tplHeraut{Version: "0.48.0"})

	assert.Equal(t, "1.2.3", r.Version)
	assert.Equal(t, "v1.2.3", r.Tag)
	require.Len(t, r.Groups, 1)
	c := r.Groups[0].Commits[0]
	assert.Equal(t, "feat", c.Type)
	assert.Equal(t, "api", c.Scope)
	assert.Equal(t, "Add thing", c.Description) // upper-first, conventional-commit description
	assert.Equal(t, "abc1234", c.ShortHash)
	assert.Equal(t, "jane", c.Author.Username)
	require.NotNil(t, c.PR)
	assert.Equal(t, "#42", c.PR.Ref)
	assert.Equal(t, "PR title", c.PR.Title)
	require.Len(t, r.Contributors, 1)
	assert.Equal(t, 1, r.Stats.CommitCount)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'BuildRelease_MapsTree'`
Expected: FAIL — `undefined: buildRelease`.

- [ ] **Step 3: Implement `buildRelease`**

Reuse the existing render.go helpers (do not duplicate their logic):
- `ver := strings.TrimPrefix(version, "v")`, `Tag: version`.
- `CompareURL: buildCompareURL(lc, previousVersion, version)`.
- Per commit: `commitLineDetails(pc)` gives `scope, breaking, desc` (upper-first description); `hash7` truncation as in `buildCommitLine`; `CommitURL: buildCommitURL(lc) + pc.raw.Hash` when `buildCommitURL(lc) != ""`; `Type` from `pc.parsed.Type` (empty for non-conventional); `Body` from `pc.parsed.Body` (or `pc.raw.Body`); `Footers` from `pc.parsed.Footers` mapped to `tplFooter{Token, Value}`; `Tickets` from `resolveTickets(text, tickets)` mapped to `tplLink`.
- `PR`: when `pr, ok := enrichment[pc.raw.Hash]`, set `&tplPR{Number, URL, Title, Labels, Ref: prRef(pr), Author: Author{Username: pr.AuthorLogin}, CreatedAt: pr.CreatedAt, MergedAt: pr.MergedAt, MergedBy: pr.MergedBy, Approvers: pr.Approvers}`; else nil.
- `Contributors`: map `[]Contributor` → `[]tplContributor{Author, PR}` (PR from the Contributor's `*PullRequest` mapped to `*tplPR` the same way; nil when absent).
- `Stats`: reuse the current `buildNotesView` stat logic (`commitCount`, `conventionalCount`, `timespan`, `daysSincePrev`/`hasDaysSincePrev`, `buildStatTicketLinks` → `[]tplStatTicket`).
- `Heraut: heraut`.

Extract shared PR-mapping into a small helper `tplPRFrom(pr PullRequest) *tplPR` used for both commit PRs and contributor PRs.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run 'BuildRelease_MapsTree'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/templatemodel.go internal/generators/native/templatemodel_internal_test.go
git commit -m "feat(generators/native): buildRelease template-model builder

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 7: Template funcs

**Files:**
- Create: `internal/generators/native/funcs.go`
- Test: `internal/generators/native/funcs_internal_test.go`

**Interfaces:**
- Produces: `func templateFuncs() template.FuncMap` with keys `upperFirst`, `date`, `join`, `indent`, `trim`.

- [ ] **Step 1: Write the failing test**

```go
package native

import (
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderWith(t *testing.T, tmpl string, data any) string {
	t.Helper()
	tt, err := template.New("t").Funcs(templateFuncs()).Parse(tmpl)
	require.NoError(t, err)
	var sb strings.Builder
	require.NoError(t, tt.Execute(&sb, data))
	return sb.String()
}

func TestTemplateFuncs(t *testing.T) {
	assert.Equal(t, "Hello", renderWith(t, `{{ upperFirst "hello" }}`, nil))
	assert.Equal(t, "a,b", renderWith(t, `{{ join "," (list "a" "b") }}`, nil))
	assert.Equal(t, "  x", renderWith(t, `{{ indent 2 "x" }}`, nil))
	assert.Equal(t, "x", renderWith(t, `{{ trim "  x  " }}`, nil))
	d := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-07-04", renderWith(t, `{{ date "2006-01-02" .D }}`, map[string]any{"D": d}))
}
```

Note: `list` is a stdlib-free helper we also add (a variadic returning `[]string`) so `join` has an argument to work with; keep it in the func map.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'TemplateFuncs'`
Expected: FAIL — `undefined: templateFuncs` (and missing `strings` import in the test — add it).

- [ ] **Step 3: Implement `funcs.go`**

```go
package native

import (
	"strings"
	"text/template"
	"time"
)

// templateFuncs is the small, safe func map exposed to user + built-in templates (ADR-0037).
// No OS / file / network access.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"upperFirst": upperFirst, // existing helper in render.go
		"date":       func(layout string, t time.Time) string { return t.Format(layout) },
		"join":       func(sep string, s []string) string { return strings.Join(s, sep) },
		"list":       func(items ...string) []string { return items },
		"indent":     func(n int, s string) string { return strings.Repeat(" ", n) + s },
		"trim":       strings.TrimSpace,
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run 'TemplateFuncs'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/funcs.go internal/generators/native/funcs_internal_test.go
git commit -m "feat(generators/native): template func map

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 8: Rewrite built-in templates onto the model + wire render.go (dogfood, output-identical)

The big one. The built-in `.tmpl` files become named blocks over the `tplRelease`/`tplChangelog` model; `render.go` builds the model and executes the block set. The fat-injection view models and line-builders are removed. **Output must be byte-identical** — verified by re-baselining the goldens and confirming the diff is empty.

**Files:**
- Modify: `internal/generators/native/changelog.tmpl`, `release_notes.tmpl` (rewrite as named blocks)
- Modify: `internal/generators/native/render.go` (`renderChangelogSection`/`renderReleaseNotes` build the model + execute blocks; delete `changelogView`/`notesView`/`commitView`/`groupView`/`commitNoteView`/`contributorView` and the `buildCommitLine`/`buildCommitBlock`/`buildContributorViews`/`buildChangelogView`/`buildNotesView` helpers — their logic now lives in `buildRelease` + the block templates)
- Modify: `internal/generators/native/generator.go` (pass `tplHeraut{Version, URL, GeneratedAt: g.now()}` into the render calls; add `now func() time.Time` + heraut version to `Generator`, defaulting to `time.Now`)
- Test: existing render/golden/integration tests; re-baseline `internal/generators/native/testdata`

**Interfaces:**
- Consumes: `buildRelease` (Task 6), `templateFuncs` (Task 7).
- Produces: `renderReleaseNotes(...)` and `renderChangelogSection(...)` keep their existing external signatures **plus** a `tplHeraut` argument; internally they build the model and execute the block set. `New(...)` gains an internal `now`/`herautVersion` (defaulted).

- [ ] **Step 1: Write the block templates (define the target)**

Rewrite `release_notes.tmpl` into named blocks. The root `release-notes` renders header → groups (heading + commit loop) → contributors → stats → footer, matching current output exactly. Example structure (fill every block to reproduce today's bytes — cross-check against the current `render.go` builders and the existing goldens):

```gotmpl
{{ define "release-notes" }}{{ template "header" . }}{{ range .Groups }}{{ template "group" . }}
{{ range .Commits }}{{ template "commit" . }}
{{ end }}{{ end }}{{ if .Contributors }}{{ template "contributors" . }}{{ end }}{{ template "stats" . }}{{ end }}

{{ define "header" }}{{ end }}
{{ define "group" }}{{ .HeadingPrefix }} {{ .Name }}{{ end }}
{{ define "commit" }}- {{ if .Scope }}*({{ .Scope }})* {{ end }}{{ if .Breaking }}[**breaking**] {{ end }}{{ .Description }} - {{ if .CommitURL }}([{{ .ShortHash }}]({{ .CommitURL }})){{ else }}{{ .ShortHash }}{{ end }}{{ if .PR }}{{ if .PR.Author.Username }} by @{{ .PR.Author.Username }}{{ end }}{{ if .PR.Number }} in [{{ .PR.Ref }}]({{ .PR.URL }}){{ end }}{{ end }}{{ range .Tickets }} ([{{ .Text }}]({{ .Href }})){{ end }}{{ end }}
```

The `contributors`, `stats`, and `footer` blocks are **ports of the current `release_notes.tmpl` fragments**, changing only their data source:
- `contributors`: the current `{{ if .Contributors }}… New Contributors ❤️ … {{ range .Contributors }}{{ .Line }}{{ end }}` becomes `{{ range .Contributors }}{{ template "contributor" . }}{{ end }}` (each contributor rendered via the `contributor` block: `* @{{ .Author.Username }} made their first contribution{{ if .PR }} in [{{ .PR.Ref }}]({{ .PR.URL }}){{ end }}` — port `buildContributorViews`).
- `stats`: the current Commit-Statistics block verbatim, reading `.CommitCount`/`.TimespanDays`/`.ConventionalCount`/`.Tickets`/`.DaysSincePrev`/`.HasDaysSincePrev` off `tplStats` instead of `notesView`.
- `footer`: empty (built-in output has no footer today).

**This is the exacting part:** every block must reproduce the current bytes. Use the current `render.go` builders (`buildCommitLine`, `buildCommitBlock`, `buildContributorViews`) and the current `changelog.tmpl` / `release_notes.tmpl` layout as the byte-level reference; the committed goldens are the objective gate.

**`HeadingPrefix` threading:** headings appear at the group level (group heading) **and** the release level (the current `contributors` and `stats` headings use the same prefix). So carry the prefix wherever a heading renders — put `HeadingPrefix string` on `tplGroup` **and** `tplRelease` (both set to `headingPrefix(typesHeadingLevel)` in `buildRelease`); the `contributors`/`stats` blocks receive the `Release` (`.HeadingPrefix`), the `group` block receives its `Group` (`.HeadingPrefix`). Update Task 5's `tplRelease` to include `HeadingPrefix string` accordingly.

**One `commit` block, body/footers wrapped by the caller:** there is a single overridable `commit` block — the one-line commit shown above (`buildCommitLine`). The changelog uses it directly. The release-notes `release` block renders `{{ template "commit" . }}` and then, when the commit has a body/footers, appends them indented (the `buildCommitBlock` tail) *around* the `commit` block, not inside it. So a user overriding `commit` reformats the line in both modes, while the release-notes body/footers stay built-in. Document this in the ADR (Task 11).

- [ ] **Step 2: Wire render.go + run to verify current tests fail first (RED via signature change), then pass**

Change `renderReleaseNotes`/`renderChangelogSection` to: build `tplRelease`/`tplChangelog` via `buildRelease`/`buildChangelog`, parse the built-in `.tmpl` with `templateFuncs()`, execute the root block. Delete the old view-model structs + builders. Update `generator.go` to pass `tplHeraut`.

Run: `go test ./internal/generators/native/`
Expected: compile/golden failures first; iterate until green.

- [ ] **Step 3: Re-baseline goldens (diff must be empty in substance)**

Run the golden tests; if output differs, fix the block templates until the diff vs the committed goldens is empty (that is the success criterion — same bytes via the new path). Only regenerate a golden after confirming the change is whitespace-identical. If a diff is non-empty and you cannot explain it as identical, STOP and report.

Run: `go test ./internal/generators/native/` → Expected: PASS with goldens unchanged (or regenerated to identical bytes).

- [ ] **Step 4: Full suite**

Run: `go test ./...` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/
git commit -m "refactor(generators/native): render built-ins from the template model

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 9: Config — `rendering.templates` + per-driver `rendering` + merge

**Files:**
- Modify: `internal/config/config.go` (`Rendering.Templates map[string]string \`yaml:"templates,omitempty"\``; `ContentDriver.Rendering *Rendering \`yaml:"rendering,omitempty"\``)
- Modify: `internal/config/merge.go` (merge `Rendering` — templates map key-by-key + excludes — when merging content drivers)
- Modify: `internal/app/pipeline.go` (`withEnvDerivations` computes effective templates map: global `cfg.Rendering.Templates` overlaid by `driver.Rendering.Templates`, set onto an app-computed `ContentDriver.EffectiveTemplates map[string]string`)
- Modify: `internal/config/config.go` (add app-computed `ContentDriver.EffectiveTemplates map[string]string \`yaml:"-"\``)
- Test: `internal/config/merge_test.go`, `internal/app/*_internal_test.go`, `schema.json` + `docs/heraut.sample.yml`

**Interfaces:**
- Produces: `Rendering.Templates`, `ContentDriver.Rendering`, `ContentDriver.EffectiveTemplates` (app-computed), `ContentDriver.Template` (existing, now meaningful).

- [ ] **Step 1: Write the failing test (merge precedence)**

```go
func TestWithEnvDerivations_MergesTemplates(t *testing.T) {
	driver := &config.ContentDriver{
		Generator: "native",
		Rendering: &config.Rendering{Templates: map[string]string{"commit": "driver-commit"}},
	}
	cfg := &config.Config{
		Rendering: &config.Rendering{Templates: map[string]string{"commit": "global-commit", "group": "global-group"}},
		Changelog: driver,
	}
	got := withEnvDerivations(driver, cfg, "")
	assert.Equal(t, "driver-commit", got.EffectiveTemplates["commit"], "driver overrides global")
	assert.Equal(t, "global-group", got.EffectiveTemplates["group"], "unset key falls through")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/app/ -run 'MergesTemplates'`
Expected: FAIL — unknown fields / undefined `EffectiveTemplates`.

- [ ] **Step 3: Add the config fields + merge logic**

- `config.go`: `Rendering` gains `Templates map[string]string \`yaml:"templates,omitempty"\``; `ContentDriver` gains `Rendering *Rendering \`yaml:"rendering,omitempty"\`` and `EffectiveTemplates map[string]string \`yaml:"-"\``.
- `withEnvDerivations`: build `eff := map[string]string{}`; copy `cfg.Rendering.Templates`; overlay `driver.Rendering.Templates`; set `clone.EffectiveTemplates = eff` (nil when empty). Include it in the "nothing applies" early-return guard.
- `merge.go`: when merging per-env content drivers, deep-merge `Rendering.Templates` key-by-key and take the override's `Template` file when set (mirror the existing `Template` handling already in `merge.go`).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/app/ ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Sync schema + sample + commit**

Add `rendering.templates` (object of string→string) to `schema.json`; add `<driver>.rendering` + `<driver>.template` docs to `docs/heraut.sample.yml`. Then:

```bash
git add -A && hk fix
git commit -m "feat(config): rendering.templates + per-driver rendering overrides

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 10: Engine — layer user snippets + file on the built-in blocks, with validation

**Files:**
- Create: `internal/generators/native/templateset.go` (`buildTemplateSet` — parse built-in `.tmpl`, then inline snippets, then the `template` file, in precedence order)
- Modify: `internal/generators/native/render.go` (use `buildTemplateSet` instead of parsing only the built-in)
- Modify: `internal/generators/native/generator.go` (pass `g.cfg.EffectiveTemplates` + `g.cfg.Template` into the render path)
- Modify: `internal/config/validator.go` (`rendering.templates`/`template` require `generator: native`; each snippet parses; the `template` file exists + parses)
- Test: `internal/generators/native/templateset_internal_test.go`, `internal/config/validator_test.go`

**Interfaces:**
- Consumes: `templateFuncs`, the built-in embedded `.tmpl` strings.
- Produces: `func buildTemplateSet(builtin string, snippets map[string]string, filePath string) (*template.Template, error)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestBuildTemplateSet_InlineOverridesBlock(t *testing.T) {
	base := `{{ define "root" }}[{{ template "commit" . }}]{{ end }}{{ define "commit" }}builtin{{ end }}`
	ts, err := buildTemplateSet(base, map[string]string{"commit": "OVERRIDDEN"}, "")
	require.NoError(t, err)
	var sb strings.Builder
	require.NoError(t, ts.ExecuteTemplate(&sb, "root", nil))
	assert.Equal(t, "[OVERRIDDEN]", sb.String())
}

func TestBuildTemplateSet_BadSnippetErrors(t *testing.T) {
	base := `{{ define "root" }}{{ end }}`
	_, err := buildTemplateSet(base, map[string]string{"commit": "{{ .Bad "}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit")
}
```

Plus a validator test that `rendering.templates` with `generator: communique` is rejected, and native + a valid snippet is accepted.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/generators/native/ -run 'BuildTemplateSet'` and `go test ./internal/config/ -run 'Template'`
Expected: FAIL — `undefined: buildTemplateSet` / validator not enforcing.

- [ ] **Step 3: Implement `buildTemplateSet` + validation**

```go
func buildTemplateSet(builtin string, snippets map[string]string, filePath string) (*template.Template, error) {
	ts := template.New("native").Funcs(templateFuncs())
	if _, err := ts.Parse(builtin); err != nil {
		return nil, fmt.Errorf("parsing built-in template: %w", err)
	}
	// Inline snippets: order the keys for determinism; each defines one block.
	keys := make([]string, 0, len(snippets))
	for k := range snippets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := ts.Parse(fmt.Sprintf("{{ define %q }}%s{{ end }}", k, snippets[k])); err != nil {
			return nil, fmt.Errorf("parsing rendering.templates.%s: %w", k, err)
		}
	}
	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading template %q: %w", filePath, err)
		}
		if _, err := ts.Parse(string(b)); err != nil {
			return nil, fmt.Errorf("parsing template %q: %w", filePath, err)
		}
	}
	return ts, nil
}
```

Wire `renderReleaseNotes`/`renderChangelogSection` to call `buildTemplateSet(builtinTmpl, effTemplates, templateFile)` and execute the root block. In `validator.go`, add: `rendering.templates`/`template` require `generator: native` (like `tag_pattern`); each snippet parses (`template.New("").Funcs(templateFuncs()).Parse`); the `template` file, if set, exists and parses.

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/generators/native/ ./internal/config/`
Expected: PASS. Then `go test ./...` → PASS (built-in output unchanged; no snippets/file set in existing tests).

- [ ] **Step 5: Commit**

```bash
git add -A && hk fix
git commit -m "feat(generators/native): layer user snippets + template file over built-ins

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 11: End-to-end tests + docs (ADR-0037, spec 05, schema, sample)

**Files:**
- Test: `internal/generators/native/generator_internal_test.go` (a Generate-level test proving an inline `commit` override changes the release-notes output, and a `template` file override)
- Create: `docs/adr/0037-native-template-api.md`; Modify `docs/adr/README.md`, `docs/specs/05-generators-and-platforms.md`, `schema.json`, `docs/heraut.sample.yml`, `docs/tasks/native-generator-roadmap.md`

- [ ] **Step 1: Write the end-to-end override test**

```go
func TestGenerate_InlineCommitOverride(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)               // previousTag
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil) // tagDate
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: add x", ""), "", nil)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore
	g := New(mr, &config.ContentDriver{
		Generator:          "native",
		EffectiveTemplates: map[string]string{"commit": "CUSTOM {{ .Description }}"},
	}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "CUSTOM Add x", "inline commit override is applied")
	assert.NotContains(t, out, "- *(", "built-in commit format is replaced")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'InlineCommitOverride'`
Expected: FAIL — override not wired (should pass if Tasks 9/10 wired `EffectiveTemplates` into the render path; if it fails on wiring, fix generator.go to pass `g.cfg.EffectiveTemplates`/`g.cfg.Template` through).

- [ ] **Step 3: Make it pass (wire the override through if not already)**

Ensure `generator.go` passes `g.cfg.EffectiveTemplates` and `g.cfg.Template` into the render calls, which feed `buildTemplateSet`.

- [ ] **Step 4: Write the docs**

- `docs/adr/0037-native-template-api.md`: record the public template API — the two entry points (inline `rendering.templates.<block>` + `<driver>.template` file), the data contract (reference `templatemodel.go`), the block set + funcs, the dogfooded built-in rewrite, the new PR fields (best-effort approvers), the injected clock for `GeneratedAt`, and the **experimental-in-v1** stability stance. Reference the design spec. Add the row to `docs/adr/README.md`.
- `docs/specs/05-generators-and-platforms.md`: document `rendering.templates`, `<driver>.rendering`, `<driver>.template`, the block list, funcs, and a "template data model" summary.
- `schema.json`: `rendering.templates` (string→string object), `<driver>.rendering`, `<driver>.template`.
- `docs/heraut.sample.yml`: examples for all three.
- `docs/tasks/native-generator-roadmap.md`: a `Phase 2.8 — user-customizable templates` section with these tasks `[x]` + notes; progress-table row.

- [ ] **Step 5: Lint, full suite, commit**

```bash
go test ./... && git add -A && hk fix
git commit -m "docs(adr): 0037 native template API; e2e override tests

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Notes for the implementer

- **Task 8 is the load-bearing one.** Its success criterion is *byte-identical built-in output*. Work block-by-block against the current `render.go` builders and the committed goldens; treat any non-empty golden diff as a bug in the rewrite, not a fixture to bless — unless you can prove it's whitespace-identical.
- **Two commit variants:** the changelog renders a one-line commit; release-notes renders a commit *block* with indented body + footers (`buildCommitBlock`). Keep them as distinct blocks so overriding `commit` behaves predictably per driver — mirror the current split, and document which block each mode uses in the ADR.
- **`HeadingPrefix`** (from `commits.types_heading_level`) is on the model (group/release), not a template literal.
- **Determinism:** the built-in templates never render `Heraut.GeneratedAt`; only user templates opt into it. The injected `now` defaults to `time.Now`; tests fix it.
- **No new deps; layer rule holds** — everything is stdlib `text/template` + existing internal packages.
