# Forge P1 Follow-ups — T167, T166, T165 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the three follow-ups left by the shipped forge epic (P1): restore the GraphQL enricher's release-window bounding and pagination (a correctness gap), make the `changelog.remote` migration error honest for git-cliff users, and specify the `forges:` surface in the behavioural spec.

**Architecture:** Three independent changes. T167 rewrites `internal/forge/gitlab/graphql.go`'s single unbounded query into two paginated, time-bounded fetches mirroring the proven legacy design (ADR-0042). T166 is a wording fix in `internal/config/loader.go`'s removed-key table. T165 adds a `forges:` section to `docs/specs/02-configuration.md`. No dependency between them; ordered by severity.

**Tech Stack:** Go, stdlib `net/http` + `encoding/json`, `testify`, `httptest`. No new dependencies.

## Context: what already shipped

The forge epic (ADR-0043) landed a `port.Forge` abstraction and a native `net/http` GitLab forge so GitLab CI enriches changelogs with the built-in `CI_JOB_TOKEN`. Two transports exist:
- **REST (default)** — per-commit `…/repository/commits/{sha}/merge_requests`; unaffected by T167.
- **GraphQL (opt-in, `api_mode: graphql`)** — batched; renders the *linked* `@username`. This is what T167 fixes.

## Global Constraints

- **TDD:** failing test first (RED), then implement (`.claude/rules/testing.md`). Table-driven tests preferred.
- **No new Go dependencies.** `internal/forge/gitlab` imports only `internal/port` + stdlib.
- **No network in tests** — `httptest.Server` only. **No real data** — synthetic placeholders only (`gitlab.example.com`, `group/subgroup/project`, `alice`, `alice-gl`).
- **No environment leakage** — any test touching resolution must inject `getenv` or neutralize CI vars (a real CI break was caused by exactly this).
- Errors wrapped with `%w`; sentinels `errors.Is`-able.
- Do not remove or alter `release.platforms` (its removal is P3). Do not touch `internal/scaffold/` (P4).
- Never bypass git hooks. Fix lint via `hk fix` (never `gofmt`/`yamlfmt` directly). Full `go test ./...` before committing.
- Commit trailer for subagent commits: `Co-Authored-By: Claude <claude-sonnet-5> <noreply@anthropic.com>` — with angle brackets, and **never** a `Claude-Session:` line.
- Each task flips its roadmap entry to `[x]` in `docs/tasks/forge-abstraction-roadmap.md` with a one-paragraph completion note.

---

### Task 1 (T167): restore GraphQL release-window bounding + pagination

**Files:**
- Modify: `internal/forge/gitlab/graphql.go`
- Test: `internal/forge/gitlab/graphql_test.go`

**Why this matters (read before coding):** the code this replaced (`internal/generators/native/enrich_gitlab.go`, still present) does two things the current GraphQL forge does not: it **pages** via `pageInfo{endCursor hasNextPage}`, and it **bounds** both connections to the release window with `committedAfter` / `mergedAfter`. The current `mergeRequests(state:merged, first:100)` specifies **no sort order**, so on a long-lived project the 100 MRs returned are whatever GitLab orders first — potentially none of the release's MRs. That is a correctness bug (wrong attribution), not just a coverage limit.

**Interfaces:**
- Consumes: `(*Forge).apiBase()`, `(*Forge).setAuth(req)`, `gitAuthors(commits)`, `ErrJobTokenGraphQL`, `gqlMRToPullRequest(n)`, `landingSHAs(n)`, `newestHash(commits)` (all already in the package).
- Produces (internal): `func oldestCommitDate(commits []port.Commit) time.Time`; two query builders and two paginated fetch helpers (names below).

- [ ] **Step 1: Write the failing tests** — append to `internal/forge/gitlab/graphql_test.go`

```go
// The release window must bound both connections: an unbounded mergeRequests(first:100) returns
// whatever GitLab orders first, which on a long-lived project can exclude the release's own MRs.
func TestEnrichGraphQL_SendsReleaseWindowBounds(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"project":{
			"repository":{"commits":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":false}}},
			"mergeRequests":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":false}}
		}}}`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/subgroup/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())

	oldest := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	_, err := f.Enrich([]port.Commit{
		{Hash: "aaa", Author: "Alice", Date: oldest},
		{Hash: "bbb", Author: "Bob", Date: oldest.Add(48 * time.Hour)},
	})
	require.NoError(t, err)

	all := strings.Join(bodies, "\n")
	assert.Contains(t, all, "committedAfter", "commits must be bounded to the release window")
	assert.Contains(t, all, "mergedAfter", "merged MRs must be bounded to the release window")
	// The bound is the OLDEST commit (minus a small buffer), not the newest.
	assert.Contains(t, all, "2026-07-01T11:59:00Z")
}

// Both connections must follow cursors until exhausted, so a release with >100 commits/MRs
// resolves fully instead of silently truncating at the first page.
func TestEnrichGraphQL_PaginatesBothConnections(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		w.Header().Set("Content-Type", "application/json")
		switch {
		// commits, page 1 → one more page
		case strings.Contains(body, "commits(") && !strings.Contains(body, "CURSOR-C"):
			_, _ = w.Write([]byte(`{"data":{"project":{"repository":{"commits":{
				"nodes":[{"sha":"aaa","author":{"username":"alice-gl"}}],
				"pageInfo":{"endCursor":"CURSOR-C","hasNextPage":true}}}}}}`))
		// commits, page 2 → done
		case strings.Contains(body, "commits("):
			_, _ = w.Write([]byte(`{"data":{"project":{"repository":{"commits":{
				"nodes":[{"sha":"bbb","author":{"username":"bob-gl"}}],
				"pageInfo":{"endCursor":"","hasNextPage":false}}}}}}`))
		// MRs, page 1 → one more page
		case !strings.Contains(body, "CURSOR-M"):
			_, _ = w.Write([]byte(`{"data":{"project":{"mergeRequests":{
				"nodes":[{"iid":"1","title":"first","mergeCommitSha":"aaa","commits":{"nodes":[]},
				          "labels":{"nodes":[]},"author":{"username":"alice-gl"}}],
				"pageInfo":{"endCursor":"CURSOR-M","hasNextPage":true}}}}}`))
		// MRs, page 2 → done
		default:
			_, _ = w.Write([]byte(`{"data":{"project":{"mergeRequests":{
				"nodes":[{"iid":"2","title":"second","mergeCommitSha":"bbb","commits":{"nodes":[]},
				          "labels":{"nodes":[]},"author":{"username":"bob-gl"}}],
				"pageInfo":{"endCursor":"","hasNextPage":false}}}}}`))
		}
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/subgroup/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{
		{Hash: "aaa", Author: "Alice", Date: time.Now().Add(-48 * time.Hour)},
		{Hash: "bbb", Author: "Bob", Date: time.Now()},
	})
	require.NoError(t, err)

	assert.Equal(t, "alice-gl", en.Authors["aaa"], "page 1 authors")
	assert.Equal(t, "bob-gl", en.Authors["bbb"], "page 2 authors must not be lost to truncation")
	assert.Equal(t, 1, en.PRs["aaa"].Number)
	assert.Equal(t, 2, en.PRs["bbb"].Number, "page 2 MRs must not be lost to truncation")
	assert.GreaterOrEqual(t, calls, 4, "both connections paginate")
}

// A malformed hasNextPage:true with no cursor must stop, not refetch page 1 forever.
func TestEnrichGraphQL_StopsOnEmptyCursor(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"project":{
			"repository":{"commits":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":true}}},
			"mergeRequests":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":true}}
		}}}`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/subgroup/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())

	done := make(chan struct{})
	go func() {
		_, _ = f.Enrich([]port.Commit{{Hash: "aaa", Author: "Alice", Date: time.Now()}})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Enrich did not terminate on an empty cursor — infinite pagination loop")
	}
	assert.Less(t, calls, 10, "must stop promptly, not spin")
}
```

Add `io`, `strings`, and `time` to the test file's imports if absent.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/forge/gitlab/ -run 'GraphQL_(SendsReleaseWindowBounds|PaginatesBothConnections|StopsOnEmptyCursor)'`
Expected: FAIL — the current single query sends no `committedAfter`/`mergedAfter` and never paginates.

- [ ] **Step 3: Restructure `graphql.go` into two paginated, bounded fetches**

Replace the single `gqlQuery` const and `postGraphQL`/`enrichGraphQL` bodies. Mirror the proven legacy shape in `internal/generators/native/enrich_gitlab.go` (read it first — it is the reference implementation for cursor handling and early-stop):

**Critical implementation note — inline these arguments, do not introduce typed variables.** The
tests use `httptest`, which returns canned JSON and performs **no GraphQL validation**: a wrong
scalar type or argument name would pass every test here and fail only against a real GitLab
instance. So mirror the legacy implementation **exactly** — it was validated against a live instance
during ADR-0042's spike. `internal/generators/native/enrich_gitlab.go` builds these arguments by
inlining a quoted RFC3339 string (`fmt.Fprintf(&extra, ",committedAfter:%s", gqlString(...))`),
with no variable declaration. Copy that shape, including its `gqlString` JSON-quoting helper (there
is an equivalent in `internal/generators/native/enrich.go`; add a small local one to
`internal/forge/gitlab` rather than importing across packages — `internal/forge/gitlab` may import
only `internal/port` + stdlib).

```go
// gqlCommitsQuery fetches commit-author handles for the release window, one page at a time.
// committedAfter bounds the scan to the window; after advances the cursor. Arguments are inlined
// (quoted via gqlString) exactly as the live-validated legacy query does — see the note above.
func gqlCommitsQuery(project, ref, committedAfter, after string) string {
	var extra strings.Builder
	if committedAfter != "" {
		fmt.Fprintf(&extra, `,committedAfter:%s`, gqlString(committedAfter))
	}
	if after != "" {
		fmt.Fprintf(&extra, `,after:%s`, gqlString(after))
	}
	return fmt.Sprintf(`{project(fullPath:%s){repository{commits(ref:%s%s,first:100){`+
		`nodes{sha author{username}}pageInfo{endCursor hasNextPage}}}}}`,
		gqlString(project), gqlString(ref), extra.String())
}

// gqlMRsQuery fetches merged MRs for the release window, one page at a time. mergedAfter is what
// keeps an unsorted connection from returning MRs unrelated to this release.
func gqlMRsQuery(project, mergedAfter, after string) string {
	var extra strings.Builder
	if mergedAfter != "" {
		fmt.Fprintf(&extra, `,mergedAfter:%s`, gqlString(mergedAfter))
	}
	if after != "" {
		fmt.Fprintf(&extra, `,after:%s`, gqlString(after))
	}
	return fmt.Sprintf(`{project(fullPath:%s){mergeRequests(state:merged%s,first:100){`+
		`nodes{iid webUrl title author{username}createdAt mergedAt mergeUser{username}`+
		`labels{nodes{title}}mergeCommitSha commits{nodes{sha}}}`+
		`pageInfo{endCursor hasNextPage}}}}`,
		gqlString(project), extra.String())
}

// gqlString renders s as a GraphQL string literal so an interpolated value cannot break out of it.
func gqlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
```

Because the arguments are inlined, `postGraphQL` sends `{"query": "..."}` with no `variables` object
— adjust it accordingly (it currently sends `path`/`ref` as variables).

`enrichGraphQL` keeps its job-token guard first, then:
1. builds `want` from the commits,
2. computes `since := oldestCommitDate(commits).Add(-time.Minute).UTC().Format(time.RFC3339)` (the minute of slack matches the legacy code: an exclusive `committedAfter`/`mergedAfter` must not drop a boundary commit; SHA matching remains authoritative),
3. calls a commits loop and an MRs loop,
4. returns `port.Enrichment{PRs: prs, Authors: authors}`.

Each loop: build its query with the current cursor (empty on the first page), POST it, decode, surface `errors[0].Message` as an error, collect, then:
```go
if !page.HasNextPage || page.EndCursor == "" || allWantedSeen {
    break
}
after = page.EndCursor
```
The `EndCursor == ""` guard is what prevents the infinite loop the third test asserts against. Early-stop when every wanted SHA has been seen (commits) or every wanted SHA has a PR (MRs) — same optimization the legacy code makes.

Add the helper:
```go
// oldestCommitDate returns the minimum commit date — the lower bound of the release window — or
// the zero time when commits is empty.
func oldestCommitDate(commits []port.Commit) time.Time {
	var oldest time.Time
	for _, c := range commits {
		if oldest.IsZero() || c.Date.Before(oldest) {
			oldest = c.Date
		}
	}
	return oldest
}
```
When `oldest` is zero (no dates), pass an **empty** `committedAfter`/`mergedAfter` so the argument is omitted from the query entirely — matching the legacy code's `if committedAfter != ""` guard and preserving today's unbounded behaviour for date-less inputs.

Update the response structs so both connections carry `pageInfo{endCursor hasNextPage}`.

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/forge/gitlab/`
Expected: PASS — the three new tests plus every pre-existing GraphQL/REST test (the `TestEnrichGraphQL_LinkedUsernameAndHeader` fixture may need `pageInfo` added to its canned JSON; update the fixture, not the assertions).

- [ ] **Step 5: Full suite + lint + roadmap + commit**

Flip **T167** to `[x]` in `docs/tasks/forge-abstraction-roadmap.md` with a completion note.

```bash
go test ./... && hk fix
git add internal/forge/gitlab/ docs/tasks/forge-abstraction-roadmap.md
git commit -m "fix(forge/gitlab): bound and paginate GraphQL enrichment (T167)"
```

---

### Task 2 (T166): make the `changelog.remote` migration hint honest for git-cliff

**Files:**
- Modify: `internal/config/loader.go` (the `removedKeys` table + the per-env message)
- Test: `internal/config/migration_test.go`

**Decision already taken (do not re-litigate):** `cfg.Forges` is consumed only by `resolveEnrichForgeIfNeeded`, which is gated on a **native** generator. `changelog.remote` was a **git-cliff** feature (ADR-0026), so today's hint — "replace with a top-level `forges:` entry and point `commits.enrichment_forge` at it" — yields a config that does nothing for a git-cliff user. Wiring `forges:` into git-cliff was rejected because the git-cliff package is itself slated for removal (native-generator roadmap Phase 2.5). So the fix is **honest wording**, not new plumbing.

- [ ] **Step 1: Write the failing test** — add to `internal/config/migration_test.go`

```go
// A git-cliff user's changelog.remote has no working replacement yet, so the migration error must
// say so rather than prescribe a forges: entry that is inert for them.
func TestLoad_RemovedKey_ChangelogRemoteHintMentionsNativeOnly(t *testing.T) {
	_, err := config.Load(writeCfg(t, `version: "1"
versioning: {strategy: semver}
changelog:
  generator: git-cliff
  output: CHANGELOG.md
  remote:
    type: gitlab
    project: group/subgroup/project
`))
	require.Error(t, err)
	require.True(t, errors.Is(err, config.ErrRemovedConfigKey))
	assert.Contains(t, err.Error(), "forges:", "the replacement is still named")
	assert.Contains(t, err.Error(), "native", "the hint must state the forges: path applies to the native generator")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/config/ -run TestLoad_RemovedKey_ChangelogRemoteHintMentionsNativeOnly`
Expected: FAIL — the current hint never mentions `native`.

- [ ] **Step 3: Reword the hint**

In `internal/config/loader.go`, update the `changelog.remote` entry in `removedKeys` and the per-env message (~line 58) so both carry the same corrected guidance. Keep them consistent — a reader hitting either must learn the same thing:

```go
{"changelog.remote", "replace with a top-level `forges:` entry and point `commits.enrichment_forge` at it (this drives enrichment for `generator: native`; explicit remote pinning for `generator: git-cliff` is not carried over)"},
```

Apply the same parenthetical to the per-env `environments.%s.changelog.remote` message.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS (the new test plus the existing migration tests, which assert on `forges:` and are unaffected).

- [ ] **Step 5: Full suite + lint + roadmap + commit**

Flip **T166** to `[x]` with a completion note recording that option (b) was chosen because git-cliff is slated for removal (Phase 2.5).

```bash
go test ./... && hk fix
git add internal/config/ docs/tasks/forge-abstraction-roadmap.md
git commit -m "fix(config): state that changelog.remote's forges: replacement is native-only (T166)"
```

---

### Task 3 (T165): specify `forges:` in `docs/specs/02-configuration.md`

**Files:**
- Modify: `docs/specs/02-configuration.md`
- Modify: `docs/tasks/forge-abstraction-roadmap.md`

**Why:** `docs/specs/` is this repo's behavioural authority (ranked above ADRs in `CLAUDE.md`'s source-of-truth hierarchy). The epic shipped a **breaking** config change whose replacement surface is currently documented only in `docs/heraut.sample.yml` and ADR-0043. A user consulting the spec finds the removed keys gone but no specification of what replaced them.

**Scope guard:** document only what is **shipped and stable**. `release.targets[]` is parsed and validated but does **not** drive publishing until P3 — if you mention it, state exactly that. Do not document it as functional.

- [ ] **Step 1: Read the ground truth**

Read, in this order: `docs/heraut.sample.yml` (the `forges:` block), `internal/config/config.go` (`Forge` + `Target` structs and their yaml tags), `internal/config/commits.go` (`EnrichmentForge`, `EnrichmentPolicy`), `internal/forge/resolve.go` + `internal/forge/detect.go` (resolution precedence and the ambiguity error), `schema.json` (enums and required fields), and `docs/adr/0043-forge-abstraction.md`. The spec must match the code, not the design doc's aspirations.

- [ ] **Step 2: Write the section**

Add a `## Forges` section to `docs/specs/02-configuration.md`, placed to match the file's existing ordering conventions (follow how sibling top-level blocks are ordered/introduced there). Cover:

1. **What a forge is** — one code-hosting platform heraut talks to; connection/identity only.
2. **Fields** — a table of `name`, `platform` (`github` | `gitlab` | `azure_devops`), `project`, `repository`, `base_url`, `api_url`, `api_mode` (`rest` default | `graphql`), `token_env`; which are required (`name`, `platform`) and which are inferred when omitted.
3. **Identity resolution** — the precedence **explicit config → CI env → git `origin` → offline**, the per-platform CI variables actually read (from `detect.go`), and the **fail-on-ambiguity** rule with the error a user sees.
4. **`commits.enrichment_forge`** — names which forge supplies PR/MR metadata; optional with exactly one forge, required with more than one.
5. **`commits.enrichment_policy`** — `disabled` | `optional` (default) | `required`; `--offline` forces `disabled`; `--force` downgrades `required` to `optional`. Cross-reference the existing prose if the file already documents this key.
6. **`api_mode` trade-off** — `rest` (default) works with a GitLab CI job token and renders `by @` from the local git author name; `graphql` needs a `read_api` token (a job token is rejected) and renders the linked `@username`.
7. **A zero-config example** — an empty/absent `forges:` block in GitLab CI, and a minimal explicit one. Synthetic placeholders only.
8. **`release.targets[]`** — one sentence: it is accepted and validated today but does not yet drive publishing (P3); `release.platforms` remains the publishing surface.

Match the document's existing tone, heading depth, and table formatting exactly.

- [ ] **Step 3: Verify accuracy against the code**

Run: `go test ./internal/config/` (fixtures + schema tests must still pass — you changed no code, so this is a sanity check that nothing else drifted).
Then re-read your section against `schema.json`'s enums and required list; any mismatch is a spec bug.

- [ ] **Step 4: Lint + roadmap + commit**

Flip **T165** to `[x]` with a completion note.

```bash
hk fix
git add docs/specs/02-configuration.md docs/tasks/forge-abstraction-roadmap.md
git commit -m "docs(specs): specify the forges: configuration surface (T165)"
```

---

## Final verification (after all tasks)

- [ ] `go test ./...` → all PASS.
- [ ] Full suite green **under a simulated CI environment** (a real CI break came from ambient env leaking into forge resolution):
      `GITHUB_ACTIONS=true GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=acme/widget go test ./...`
- [ ] `hk check` → clean.
- [ ] `git grep -n "release.platforms"` → still present and working (its removal is P3).
- [ ] No real data in any changed file — synthetic placeholders only.
