# Commit-Author Attribution (native, GitHub-first) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Credit the **commit author** (`by @<handle>`) on native changelog / release-notes commit lines, independent of pull requests, resolving the handle from GitHub at zero extra API cost.

**Architecture:** Extend native's existing per-SHA GitHub GraphQL query with `author{user{login}}` to get each commit author's handle in the same batched call. Carry a `sha→handle` map out of enrichment, overlay it onto the grouped commits (`rawCommit.AuthorHandle`), and render `by @{{ .Author.Username }}` — with a PR contributing only its `in [#N](url)` link. GitLab/Azure return an empty handle map (unchanged) and are tracked as follow-ups.

**Tech Stack:** Go, stdlib; `gh api graphql` via `port.Runner`; existing `internal/generators/native` package. No new dependencies.

## Global Constraints

- **Native-only, GitHub-only in this cut.** GitLab/Azure return an empty handle map → no `by @` there (unchanged from today). No regression.
- **No new Go dependencies.** stdlib + existing internal packages.
- **Zero extra GitHub API calls** — the handle rides the existing `object(oid:sha){…}` query.
- **Attribution rule (verbatim):** `by @<commit-author handle>` when the handle is non-empty, then ` in [#N](url)` when the commit has an associated PR. The PR's own author no longer drives `by @`.
- **Unlinked author / offline / degrade → no `by @`** (empty handle; never a bare name).
- **Per-section render output stays byte-identical for `nil` enrichment.** Existing goldens under `internal/generators/native/testdata/*.golden` that pass `nil` enrichment must not change. The deliberate exception is the contributors golden (Task 3), whose `by @` source changes by design — re-baselined with a handle in the fixture.
- Layer rules: `internal/generators/native` imports only `internal/{port,config,conventionalcommit}` + stdlib.
- TDD (failing test first); lint via `hk fix` (never gofmt directly); never bypass git hooks.
- Commit trailer: `Co-Authored-By: Claude <model> <noreply@anthropic.com>`.

---

### Task 1: GitHub — fetch the commit-author handle

**Files:**
- Modify: `internal/generators/native/enrich_github.go`
- Modify: `internal/generators/native/enrich_internal_test.go` (the `ghGraphQLResponse` test helper)
- Test: `internal/generators/native/enrich_github_internal_test.go`

**Interfaces:**
- Produces: `func enrichGitHub(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]PullRequest, map[string]string, error)` — the second map is `sha → commit-author GitHub login` (absent when the author email isn't linked to a user). `func parseGitHubResponse(stdout string, shas []string) (map[string]PullRequest, map[string]string, error)`.

- [ ] **Step 1: Write the failing test**

Add to `enrich_github_internal_test.go`:

```go
func TestEnrichGitHub_CommitAuthorHandle(t *testing.T) {
	mr := exectest.NewMockRunner()
	sha := "aa11bb22cc33dd44ee55ff6677889900aabbccdd"
	lc := makeGitHubLC("owner", "repo", "tok")
	// Commit has an author linked to a GitHub user, but NO associated PR.
	mr.QueueResponse(`{"data":{"repository":{"s0":{
		"author":{"user":{"login":"alice"}},
		"associatedPullRequests":{"nodes":[]}}}}}`, "", nil)

	prs, authors, err := enrichGitHub(mr, lc, []string{sha})
	require.NoError(t, err)
	assert.Empty(t, prs, "no associated PR")
	assert.Equal(t, "alice", authors[sha], "commit-author handle resolved")
}

func TestEnrichGitHub_CommitAuthorUnlinked(t *testing.T) {
	mr := exectest.NewMockRunner()
	sha := "bb22cc33dd44ee55ff6677889900aabbccddeeff"
	lc := makeGitHubLC("owner", "repo", "tok")
	mr.QueueResponse(`{"data":{"repository":{"s0":{
		"author":{"user":null},
		"associatedPullRequests":{"nodes":[]}}}}}`, "", nil)

	_, authors, err := enrichGitHub(mr, lc, []string{sha})
	require.NoError(t, err)
	_, ok := authors[sha]
	assert.False(t, ok, "unlinked author yields no handle entry")
}
```

Also update the `prFragment` assertion is automatic (`TestBuildGitHubQuery` builds `want` from the `prFragment` const), but you MUST update every existing `enrichGitHub(...)` / `parseGitHubResponse(...)` call in this test file to the new three-value return (e.g. `prs, _, err := enrichGitHub(...)`).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'EnrichGitHub'`
Expected: FAIL — `enrichGitHub` returns 2 values, not 3; `authors` undefined.

- [ ] **Step 3: Implement**

In `enrich_github.go`:

Extend `prFragment` (add `author{user{login}}` to the `Commit` node, before `associatedPullRequests`):

```go
	prFragment = "...on Commit{author{user{login}}associatedPullRequests(first:1){nodes{number url title author{login}labels(first:20){nodes{name}}createdAt mergedAt mergedBy{login}latestReviews(first:20){nodes{state author{login}}}}}}"
```

Extend `graphQLCommit`:

```go
type graphQLCommit struct {
	Author struct {
		User *struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"author"`
	AssociatedPullRequests struct {
		Nodes []graphQLPR `json:"nodes"`
	} `json:"associatedPullRequests"`
}
```

Change `enrichGitHub`, `fetchGitHubChunk`, and `parseGitHubResponse` to also return the authors map. `enrichGitHub` merges each chunk's authors alongside prs:

```go
func enrichGitHub(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]PullRequest, map[string]string, error) {
	prs := make(map[string]PullRequest)
	authors := make(map[string]string)
	for i := 0; i < len(shas); i += ghChunkSize {
		end := i + ghChunkSize
		if end > len(shas) {
			end = len(shas)
		}
		p, a, err := fetchGitHubChunk(runner, lc, shas[i:end])
		if err != nil {
			return nil, nil, fmt.Errorf("enriching GitHub PRs (chunk %d): %w", i/ghChunkSize, err)
		}
		for k, v := range p {
			prs[k] = v
		}
		for k, v := range a {
			authors[k] = v
		}
	}
	return prs, authors, nil
}

func fetchGitHubChunk(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]PullRequest, map[string]string, error) {
	query := buildGitHubQuery(lc.Owner, lc.Repo, shas)
	stdout, _, err := runner.RunEnv(lc.APIEnv(), "gh", "api", "graphql", "-f", "query="+query)
	if err != nil {
		return nil, nil, fmt.Errorf("gh api graphql: %w", err)
	}
	return parseGitHubResponse(stdout, shas)
}
```

In `parseGitHubResponse`, build both maps (add the authors map; populate it before the `len(...Nodes) == 0` early `continue` so a commit with an author but no PR still records its handle):

```go
func parseGitHubResponse(stdout string, shas []string) (map[string]PullRequest, map[string]string, error) {
	var resp graphQLResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		return nil, nil, fmt.Errorf("parsing gh api graphql response: %w", err)
	}
	if len(resp.Errors) > 0 {
		return nil, nil, fmt.Errorf("gh api graphql: %s", resp.Errors[0].Message)
	}
	prs := make(map[string]PullRequest)
	authors := make(map[string]string)
	for i, sha := range shas {
		alias := fmt.Sprintf("s%d", i)
		commit, ok := resp.Data.Repository[alias]
		if !ok || commit == nil {
			continue
		}
		if commit.Author.User != nil && commit.Author.User.Login != "" {
			authors[sha] = commit.Author.User.Login
		}
		if len(commit.AssociatedPullRequests.Nodes) == 0 {
			continue
		}
		pr := commit.AssociatedPullRequests.Nodes[0]
		// ... existing PR mapping unchanged ...
		prs[sha] = PullRequest{ /* unchanged fields */ }
	}
	return prs, authors, nil
}
```

Update the `ghGraphQLResponse` helper in `enrich_internal_test.go` so its generated JSON also carries the commit author (same login as the PR author, so downstream `by @<login>` assertions hold after the render switch):

```go
func ghGraphQLResponse(number int, url, login string) string {
	return fmt.Sprintf(
		`{"data":{"repository":{"s0":{"author":{"user":{"login":%q}},"associatedPullRequests":{"nodes":[{"number":%d,"url":%q,"author":{"login":%q}}]}}}}}`,
		login, number, url, login)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run 'EnrichGitHub|BuildGitHubQuery'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich_github.go internal/generators/native/enrich_github_internal_test.go internal/generators/native/enrich_internal_test.go
git commit -m "feat(generators/native): resolve commit-author handle (GitHub)

Co-Authored-By: Claude <model> <noreply@anthropic.com>"
```

---

### Task 2: Carry the handle to the commit (enrichResult + overlay)

**Files:**
- Modify: `internal/generators/native/enrich.go` (`enrichResult` type; `enrich` + `enrichForRelease` return it)
- Modify: `internal/generators/native/commits.go` (`rawCommit.AuthorHandle`)
- Modify: `internal/generators/native/group.go` (add `overlayAuthorHandles`)
- Modify: `internal/generators/native/generator.go` (`generateReleaseNotes`, `renderRelease` use `.prs` + overlay `.authors`)
- Test: `internal/generators/native/group_internal_test.go`

**Interfaces:**
- Consumes: `enrichGitHub` three-value return (Task 1).
- Produces: `type enrichResult struct { prs map[string]PullRequest; authors map[string]string }`; `func (g *Generator) enrich(...) (enrichResult, error)`; `func (g *Generator) enrichForRelease(...) (enrichResult, error)`; `func overlayAuthorHandles(groups []group, authors map[string]string)`; `rawCommit.AuthorHandle string`.

- [ ] **Step 1: Write the failing test**

In `group_internal_test.go`:

```go
func TestOverlayAuthorHandles(t *testing.T) {
	groups := []group{{name: "🚀 Features", order: 0, commits: []parsedCommit{
		parsedFrom("aaaaaaa", "feat: x"),
		parsedFrom("bbbbbbb", "feat: y"),
	}}}
	overlayAuthorHandles(groups, map[string]string{"aaaaaaa": "alice"})
	assert.Equal(t, "alice", groups[0].commits[0].raw.AuthorHandle)
	assert.Equal(t, "", groups[0].commits[1].raw.AuthorHandle, "no handle → empty")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'OverlayAuthorHandles'`
Expected: FAIL — `overlayAuthorHandles` undefined; `AuthorHandle` unknown field.

- [ ] **Step 3: Implement**

`commits.go` — add to `rawCommit` (documented as enrichment-populated, empty from git):

```go
	Body    string    // %b — body without the subject
	// AuthorHandle is the commit author's platform handle (e.g. GitHub login), overlaid by
	// enrichment after collection; empty from git alone and for platforms without resolution.
	AuthorHandle string
```

`group.go` — add:

```go
// overlayAuthorHandles stamps each commit's resolved author handle (sha → handle) onto the grouped
// commits, so the renderer can credit the commit author independently of any associated PR.
func overlayAuthorHandles(groups []group, authors map[string]string) {
	if len(authors) == 0 {
		return
	}
	for gi := range groups {
		for ci := range groups[gi].commits {
			if h, ok := authors[groups[gi].commits[ci].raw.Hash]; ok {
				groups[gi].commits[ci].raw.AuthorHandle = h
			}
		}
	}
}
```

`enrich.go` — introduce the struct and change returns:

```go
// enrichResult bundles per-commit remote data: associated PRs and commit-author handles.
type enrichResult struct {
	prs     map[string]PullRequest
	authors map[string]string
}

func (g *Generator) enrich(lc *port.LinkContext, commits []rawCommit) (enrichResult, error) {
	if lc == nil {
		return enrichResult{}, nil
	}
	shas := make([]string, 0, len(commits))
	for _, c := range commits {
		shas = append(shas, c.Hash)
	}
	switch lc.Platform {
	case "github":
		prs, authors, err := enrichGitHub(g.runner, lc, shas)
		return enrichResult{prs: prs, authors: authors}, err
	case "gitlab":
		prs, err := enrichGitLab(g.runner, lc, shas)
		return enrichResult{prs: prs}, err
	case "azure_devops":
		prs, err := enrichAzure(g.httpClient, lc, shas)
		return enrichResult{prs: prs}, err
	default:
		return enrichResult{}, nil
	}
}
```

In `enrichForRelease`, change the signature to `(enrichResult, error)` and replace the three `return nil, nil` / `return nil, fmt.Errorf(...)` with the struct form:

```go
func (g *Generator) enrichForRelease(lc *port.LinkContext, commits []rawCommit) (enrichResult, error) {
	if g.cfg.RemoteMetadata == "disabled" {
		return enrichResult{}, nil
	}
	er, err := g.enrich(lc, commits)
	if err != nil {
		if g.cfg.RemoteMetadata == "required" {
			return enrichResult{}, fmt.Errorf("remote enrichment (required): %w", err)
		}
		if !g.degraded {
			fmt.Fprintf(os.Stderr, "warning: remote enrichment unavailable; rendering without PR attribution: %v\n", err)
		}
		g.degraded = true
		return enrichResult{}, nil
	}
	return er, nil
}
```

`generator.go` — `generateReleaseNotes`: capture the struct, overlay, pass `.prs`:

```go
	er, err := g.enrichForRelease(lc, commits)
	if err != nil {
		return "", err
	}
	before, err := authorsBefore(g.runner, prev)
	if err != nil {
		return "", err
	}
	groups := groupCommits(commits, g.cfg.Types, g.cfg.Excludes)
	overlayAuthorHandles(groups, er.authors)
	contributors := collectContributors(toParsedCommits(renderedCommits(commits, groups)), before, er.prs)
	return renderReleaseNotes(tag, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, prevDate, g.cfg.TypesHeadingLevel, er.prs, contributors, g.herautMeta(), g.cfg.EffectiveTemplates, g.cfg.Template)
```

`generator.go` — `renderRelease` (groups already built + empty-checked before enrich; overlay onto the groups):

```go
	var prs map[string]PullRequest
	if enrichEnabled {
		er, err := g.enrichForRelease(lc, commits)
		if err != nil {
			return "", err
		}
		prs = er.prs
		overlayAuthorHandles(groups, er.authors)
	}
	return renderChangelogSection(version, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, g.cfg.HeadingVersionPattern, g.cfg.TypesHeadingLevel, prs, g.herautMeta(), g.cfg.EffectiveTemplates, g.cfg.Template)
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/`
Expected: PASS. Output is unchanged in this task (the handle is populated but not yet rendered) — the existing `by @octocat` Generate assertions still pass via the PR-author path in the unchanged template.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/enrich.go internal/generators/native/commits.go internal/generators/native/group.go internal/generators/native/generator.go internal/generators/native/group_internal_test.go
git commit -m "feat(generators/native): carry commit-author handle onto commits

Co-Authored-By: Claude <model> <noreply@anthropic.com>"
```

---

### Task 3: Render `by @<commit author>` (buildCommit + template + goldens)

**Files:**
- Modify: `internal/generators/native/templatemodel.go` (`buildCommit`)
- Modify: `internal/generators/native/blocks.tmpl` (`commit` block)
- Modify: `internal/generators/native/render_internal_test.go` (contributors golden test — set a handle)
- Regenerate: `internal/generators/native/testdata/release_notes_contributors.golden`
- Test: `internal/generators/native/enrich_internal_test.go` (author-attribution render cases)

**Interfaces:**
- Consumes: `rawCommit.AuthorHandle` (Task 2); `parsedFrom` test helper.

- [ ] **Step 1: Write the failing tests**

In `enrich_internal_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'CommitBlock_ByCommitAuthor|CommitBlock_NoHandle'`
Expected: FAIL — `by @` still gated on the PR; `AuthorHandle` not yet the source.

- [ ] **Step 3: Implement**

`templatemodel.go` — in `buildCommit`, drop the PR-author `username` and source `Author.Username` from the commit's handle:

```go
	var pr *tplPR
	if p, ok := enrichment[pc.raw.Hash]; ok {
		pr = tplPRFrom(p)
	}

	return tplCommit{
		Type:        commitType,
		Scope:       scope,
		Breaking:    breaking,
		Description: desc,
		Subject:     pc.raw.Subject,
		Body:        body,
		Hash:        pc.raw.Hash,
		ShortHash:   shortHash,
		CommitURL:   commitURL,
		Date:        pc.raw.Date,
		Author:      Author{Name: pc.raw.Author, Email: pc.raw.Email, Username: pc.raw.AuthorHandle},
		PR:          pr,
		Tickets:     links,
		Footers:     footers,
	}
```

(Delete the now-unused `username := ""` / `username = p.AuthorLogin` lines.)

`blocks.tmpl` — change the `commit` block's attribution segment. Old segment:
`{{ if .PR }}{{ if .PR.Author.Username }} by @{{ .PR.Author.Username }}{{ end }}{{ if .PR.Number }} in [{{ .PR.Ref }}]({{ .PR.URL }}){{ end }}{{ end }}`
New segment:
`{{ if .Author.Username }} by @{{ .Author.Username }}{{ end }}{{ if .PR }}{{ if .PR.Number }} in [{{ .PR.Ref }}]({{ .PR.URL }}){{ end }}{{ end }}`

So the full `commit` block line becomes:

```
{{define "commit"}}- {{ if .Scope }}*({{ .Scope }})* {{ end }}{{ if .Breaking }}[**breaking**] {{ end }}{{ .Description }} - {{ if .CommitURL }}([{{ .ShortHash }}]({{ .CommitURL }})){{ else }}{{ .ShortHash }}{{ end }}{{ if .Author.Username }} by @{{ .Author.Username }}{{ end }}{{ if .PR }}{{ if .PR.Number }} in [{{ .PR.Ref }}]({{ .PR.URL }}){{ end }}{{ end }}{{ range .Tickets }} ([{{ .Text }}]({{ .Href }})){{ end }}{{end}}
```

- [ ] **Step 4: Run the new tests + the existing goldens**

Run: `go test ./internal/generators/native/ -run 'CommitBlock|Golden'`
Expected: the three new tests PASS. The changelog/release-notes goldens that pass `nil` enrichment stay green (no handle, no PR → no `by @`, byte-identical). The **contributors** golden test (`TestRenderReleaseNotes_Contributors_Golden`) now FAILS: its commit line previously showed `by @alice` from the PR author, but with the new rule and no overlaid handle it shows only ` in [#7](...)`.

- [ ] **Step 5: Fix the contributors test to exercise the new path, then re-baseline**

In `render_internal_test.go`, `TestRenderReleaseNotes_Contributors_Golden`: overlay a commit-author handle so the commit line shows `by @alice in [#7]` under the new rule. After building `fixtureGroups()`, set the handle on the api commit (rc1) and pass the groups:

```go
	groups := fixtureGroups()
	overlayAuthorHandles(groups, map[string]string{rc1.Hash: "alice"})
	got, err := renderReleaseNotes(
		"v1.2.3", "v1.2.2", releaseDate,
		groups, githubLC, nil, prevDate, 3, prs, contribs, tplHeraut{}, nil, "",
	)
```

Then re-baseline the golden and diff-review that the only change is the api commit line now reads `… )) by @alice in [#7](…)` (commit-author credit + PR link), everything else identical:

```bash
UPDATE_GOLDEN=1 go test ./internal/generators/native/ -run 'Contributors_Golden'
git diff internal/generators/native/testdata/release_notes_contributors.golden
```

- [ ] **Step 6: Full suite (incl. Generate enrich tests)**

Run: `go test ./...`
Expected: PASS. The `TestGenerate_Enrich_*` tests still assert `by @octocat in [#42]` — now sourced from the commit author (the `ghGraphQLResponse` fixture sets `author.user.login = octocat`, overlaid onto the commit), so the assertion holds.

- [ ] **Step 7: Commit**

```bash
git add internal/generators/native/templatemodel.go internal/generators/native/blocks.tmpl internal/generators/native/render_internal_test.go internal/generators/native/enrich_internal_test.go internal/generators/native/testdata/release_notes_contributors.golden
git commit -m "feat(generators/native): credit the commit author on commit lines

Co-Authored-By: Claude <model> <noreply@anthropic.com>"
```

---

### Task 4: Docs — ADR-0039, spec, roadmap, follow-ups

**Files:**
- Create: `docs/adr/0039-commit-author-attribution.md`; Modify `docs/adr/README.md`
- Modify: `docs/specs/05-generators-and-platforms.md`
- Modify: `docs/tasks/native-generator-roadmap.md`

- [ ] **Step 1: Write ADR-0039**

Create `docs/adr/0039-commit-author-attribution.md` (Status Accepted, Date 2026-07-18). Record: the change of `by @` source from the PR author to the **commit author** (git-cliff-matching), why (dogfooding surfaced that native's PR-only attribution produces nothing for direct-commit workflows), the GitHub free-query mechanism (`author{user{login}}` on the existing batched query), the unlinked/offline → no `by @` behavior, and that GitLab/Azure are deferred follow-ups (GitLab pending a GraphQL schema spike). Reference the design spec `docs/superpowers/specs/2026-07-17-commit-author-attribution-design.md`. Note it as a follow-on to [ADR-0036](0036-unified-enrichment-model.md). Add the README row:

```markdown
| [0039](0039-commit-author-attribution.md) | Commit-Author Attribution (native) | Accepted |
```

- [ ] **Step 2: Update spec 05**

In `docs/specs/05-generators-and-platforms.md`, under the native section, note that commit lines credit the **commit author** (`by @<handle>`) resolved from the platform, with an associated PR contributing only `in [#N]`; GitHub is supported, GitLab/Azure render no `by @` yet (follow-ups).

- [ ] **Step 3: Roadmap — Phase 2.10 + follow-up tasks**

In `docs/tasks/native-generator-roadmap.md`: add a `Phase 2.10 — commit-author attribution (ADR-0039)` progress-table row (Status: Complete — GitHub) and a Phase 2.10 section marked `[x]` with a completion note. Log two open follow-up tasks with `[ ]` markers: **GitLab commit-author handle** (includes the GitLab GraphQL schema spike: verify `mergeRequests(commitSha:)` / a commit-by-sha field / `Commit.author { username }` via `glab api graphql` introspection, and whether it can batch), and **Azure commit-author handle** (identity API). Note the New-Contributors-block handle reuse as a possible extension.

- [ ] **Step 4: Verify + commit**

Run: `go test ./... && hk fix`
Expected: all green; lint clean.

```bash
git add docs/
git commit -m "docs(adr): 0039 commit-author attribution; spec, roadmap, follow-ups

Co-Authored-By: Claude <model> <noreply@anthropic.com>"
```

---

## Notes for the implementer

- **The handle rides the EXISTING query.** Do not add a second GitHub API call — `author{user{login}}` goes into `prFragment` alongside `associatedPullRequests`.
- **`by @` is now commit-author, not PR-author.** The commit block sources it from `.Author.Username` (fed by `rawCommit.AuthorHandle`); the PR block contributes only the `in [#N]` link. When committer ≠ PR author, the committer wins — that is the intended change.
- **Byte-identity gate (Task 3):** every golden that passes `nil` enrichment must stay identical (no handle, no PR → no `by @`). The one intended golden change is `release_notes_contributors.golden`, and only because the test now overlays a handle — diff-review that the sole change is the api commit line gaining ` by @alice`.
- **`ghGraphQLResponse` sets commit-author = PR-author = `login`** so the `TestGenerate_Enrich_*` `by @octocat` assertions survive the source switch. Keep that helper's two logins equal.
