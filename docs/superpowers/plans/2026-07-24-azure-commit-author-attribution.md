# Azure DevOps commit-author `by @` from local git — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render `by @<author>` on Azure DevOps commit lines, sourced from the local git author email's local-part (no new API call), giving Azure attribution parity with GitHub/GitLab.

**Architecture:** A pure `azureCommitAuthors(commits)` helper builds a `sha → handle` map from each commit's git author (email local-part via the existing `azureAuthorLogin`); `enrich()`'s azure branch returns it as the `authors` map, which the existing `overlayAuthorHandles` path stamps into `by @`. No new HTTP request; the map rides `enrichForRelease`, so it's gated by `remote_metadata` exactly like GitHub/GitLab.

**Tech Stack:** Go, `testify`, `exectest.MockRunner` + `httptest` (existing Azure test harness).

## Global Constraints

- TDD: failing test first, RED, then implement (`.claude/rules/testing.md`).
- No new Go dependencies; `internal/generators/native` imports only `internal/{port,config,conventionalcommit}` + stdlib.
- **No real data** in tests/docs — the T151 spike used confidential Azure data (host, emails, names); NONE of it may appear anywhere. Synthetic placeholders only (`alice@example.com`, `alice`, `a@example.com`, `a`).
- `by @` credits the commit **author** (`%ae`), per ADR-0039 — not the committer or PR author.
- Behavior is gated by the existing `enrichForRelease` policy (no special-casing): `disabled`/`--offline` or a degraded `pullrequestquery` → no Azure `by @`.
- Never bypass git hooks. Fix lint via `hk fix` (never `gofmt`/`yamlfmt` directly).
- Commit trailer for subagent commits: `Co-Authored-By: Claude <claude-sonnet-5> <noreply@anthropic.com>` (no `Claude-Session:` line).

---

### Task 1: `azureCommitAuthors` helper + `enrich()` dispatch

**Files:**
- Modify: `internal/generators/native/enrich_azure.go` (add `azureCommitAuthors`)
- Modify: `internal/generators/native/enrich.go` (azure dispatch returns the `authors` map)
- Test: `internal/generators/native/enrich_azure_internal_test.go` (unit for the helper + flip the e2e assertion)

**Interfaces:**
- Consumes: existing `azureAuthorLogin(azureIdentityRef) string`, `azureIdentityRef{DisplayName, UniqueName}`, `rawCommit{Hash, Author, Email}`.
- Produces: `func azureCommitAuthors(commits []rawCommit) map[string]string` — `sha → author handle`.

- [ ] **Step 1: Write the failing tests**

Add a unit test to `enrich_azure_internal_test.go`:

```go
func TestAzureCommitAuthors(t *testing.T) {
	got := azureCommitAuthors([]rawCommit{
		{Hash: "aaa", Author: "Alice", Email: "alice@example.com"}, // email → local-part
		{Hash: "bbb", Author: "Bob", Email: ""},                    // no email → git name fallback
		{Hash: "ccc", Author: "", Email: ""},                       // nothing → omitted
	})
	assert.Equal(t, map[string]string{"aaa": "alice", "bbb": "Bob"}, got)
}
```

In the existing `TestGenerate_Enrich_Azure` (same file), **replace** the two assertion lines:

```go
	assert.Contains(t, out, "in [!42]("+srv.URL+"/myorg/myproj/_git/myrepo/pullrequest/42)")
	assert.NotContains(t, out, "by @", "Azure commit-author handle resolution is not yet implemented")
```

with:

```go
	assert.Contains(t, out, "by @a in [!42]("+srv.URL+"/myorg/myproj/_git/myrepo/pullrequest/42)",
		"Azure renders the commit-author from the local git email local-part (a@example.com → a)")
```

(The fixture commit is `record("abc1234567", "A", "a@example.com", …)`, so the author email `a@example.com` → handle `a`.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'TestAzureCommitAuthors|TestGenerate_Enrich_Azure'`
Expected: `TestAzureCommitAuthors` fails to compile (`azureCommitAuthors` undefined); once that's addressed, `TestGenerate_Enrich_Azure` fails on the new `by @a` assertion (no `by @` rendered yet).

- [ ] **Step 3: Add `azureCommitAuthors`**

In `enrich_azure.go`, add (next to `azureAuthorLogin`):

```go
// azureCommitAuthors maps each commit SHA to its author handle — the git author email's
// local-part, via the existing azureAuthorLogin. Azure exposes no identity resolvable from a git
// email (T151 spike), so this local render is the only source and it makes no API call. Commits
// whose author yields no handle are omitted.
func azureCommitAuthors(commits []rawCommit) map[string]string {
	authors := make(map[string]string, len(commits))
	for _, c := range commits {
		if h := azureAuthorLogin(azureIdentityRef{DisplayName: c.Author, UniqueName: c.Email}); h != "" {
			authors[c.Hash] = h
		}
	}
	return authors
}
```

- [ ] **Step 4: Wire the `enrich()` dispatch**

In `enrich.go`, change the azure branch:

```go
	case "azure_devops":
		prs, err := enrichAzure(g.httpClient, lc, shas)
		return enrichResult{prs: prs, authors: azureCommitAuthors(commits)}, err
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/generators/native/`
Expected: PASS (the new unit test + the flipped e2e).

- [ ] **Step 6: Full suite + commit**

Run: `go test ./... && hk fix`
Expected: all PASS; lint clean.

```bash
git add internal/generators/native/enrich_azure.go internal/generators/native/enrich.go internal/generators/native/enrich_azure_internal_test.go
git commit -m "feat(generators/native): Azure commit-author by @ from local git email"
```

---

### Task 2: Docs — roadmap T151 done + spec 05

**Files:**
- Modify: `docs/tasks/native-generator-roadmap.md` (T151 → `[x]`, Phase 2.10 glance row)
- Modify: `docs/specs/05-generators-and-platforms.md` (Azure enrichment caveat)

- [ ] **Step 1: Flip T151 to done + note**

In `docs/tasks/native-generator-roadmap.md`, change `#### \`[ ]\` T151` → `#### \`[x]\` T151` and append a completion note:

```markdown
**Completion note (2026-07-24):** A live spike proved Azure exposes no identity resolvable from a
git commit email — the Commits API carries only git `name`/`email`, and both `_apis/identities`
and Graph `subjectquery` returned no match for the author email. So the Azure commit-author handle
is rendered from the **local git author email local-part** (via the existing `azureAuthorLogin`,
the same rendering Azure PR authors use) — no new API call. It rides `enrich()` → `enrichForRelease`,
so it is gated by `remote_metadata` like GitHub/GitLab (absent under `disabled`/offline or a
degraded `pullrequestquery`). It is a text attribution, not a clickable Azure @mention (inherent to
Azure). **Scope:** S.
```

Update the "Progress at a glance" Phase 2.10 row to reflect Azure is now covered (e.g. drop the "GitHub, GitLab" qualifier / note all three platforms have `by @`; keep the row's format).

- [ ] **Step 2: Update spec 05 Azure caveat**

In `docs/specs/05-generators-and-platforms.md`, locate the "**Azure DevOps only, this cut**: it does not yet [resolve the commit-author handle]" caveat (near line 40) and update it: Azure now renders `by @<author>` from the local git author email's local-part (no Azure identity is resolvable from a git email — T151 spike); note it is a text attribution, not a clickable Azure mention. Remove the "does not yet" wording. Edit faithfully to the current text.

- [ ] **Step 3: Lint + commit**

Run: `hk fix`
Expected: clean.

```bash
git add docs/tasks/native-generator-roadmap.md docs/specs/05-generators-and-platforms.md
git commit -m "docs: mark T151 done — Azure commit-author from local git email"
```

---

## Final verification (after all tasks)

- [ ] `go test ./...` → all PASS.
- [ ] `hk check` (or `mise run lint:check`) → clean.
- [ ] Confirm no confidential/real Azure data (host, emails, names) appears in any changed file — synthetic placeholders only.
