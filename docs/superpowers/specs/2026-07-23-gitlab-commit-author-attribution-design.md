# GitLab commit-author attribution via batched GraphQL — Design

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-07-23
- **Author**: bchatard (with Claude)
- **Related**: ADR-0034 (native remote enrichment), ADR-0036 (unified enrichment model), ADR-0039 (commit-author attribution, GitHub), roadmap T150 (this), Phase 3 (raw-HTTP clients)

---

## Problem

Native renders `by @<handle>` commit-author attribution for GitHub (ADR-0039) but **not** for
GitLab. GitHub gets the handle for free by adding `author{user{login}}` to its existing batched
`gh api graphql` query. GitLab's enrichment (`enrich_gitlab.go`) instead makes a **per-commit**
REST call to `/repository/commits/:sha/merge_requests` — which returns the associated MR (for
`in [!N]` refs and MR review-metadata) but **not** the commit author's GitLab handle. So GitLab
changelogs carry no `by @`, and the per-commit REST fetch is **O(commit)** — slow and
rate-limit-prone on large repos (observed live: 404s, connection resets, GitLab rate limits).

A spike (bchatard) established two facts about GitLab's GraphQL API:

1. **There is no commit→MR mapping in GraphQL.** MR references therefore remain O(commit) REST —
   there is no way to batch them.
2. **`project.repository.commits(ref:).nodes.author.username` returns the commit author's GitLab
   handle for every commit in one connection**, batched (paginated). The `commits(ref:)`
   connection accepts `ref` (required), `committedAfter`/`committedBefore` (ISO8601 date bounds),
   `first`/`after` (cursor pagination), and `author`/`path`/`query` filters.

## Goal

Give GitLab the same `by @<handle>` commit-author attribution GitHub has, via a **batched**
GraphQL fetch (O(pages), not O(commit)), by resolving a `sha → author.username` map that flows
into the existing `overlayAuthorHandles` path.

## Non-goals (explicitly out of scope)

- **GitLab MR-ref attribution (`in [!N]`) and MR review-metadata** (merged_by, dates, labels).
  GraphQL cannot map commit→MR, and the REST path is O(commit) — the exact cost this change
  avoids. The per-commit MR REST enrichment is **dropped**. This is a deliberate behavior change
  (GitLab loses `in [!N]` and MR-derived review fields). Tracked as a deferred follow-up (an
  opt-in O(commit) MR-refs mode) and documented as a user-facing limitation.
- **Azure commit-author handle** (roadmap T151) — separate, Azure is already native HTTP.
- **GitLab "New Contributors" first-timer detection** — remains deferred (unchanged).
- **Phase 3 raw-HTTP clients** — this stays on the `glab` CLI transport; Phase 3 will port it
  along with all enrichment when/if it happens.

## Design

### 1. Replace GitLab enrichment: MR REST → GraphQL commit-author

`enrich_gitlab.go`'s `enrichGitLab(runner, lc, shas) (map[string]PullRequest, error)` is replaced
by a function returning a **`sha → handle` author map** (no PRs):

```go
// returns sha → GitLab author username for the given SHAs (batched); commits whose author email
// is not linked to a GitLab account are absent.
func enrichGitLab(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]string, error)
```

`enrich()`'s gitlab branch changes from `enrichResult{prs: prs}` to `enrichResult{authors: authors}`:

```go
case "gitlab":
    authors, err := enrichGitLab(g.runner, lc, shas)
    return enrichResult{authors: authors}, err
```

`overlayAuthorHandles(groups, er.authors)` already consumes the `authors` map and stamps
`by @<handle>` — identical to GitHub. No renderer/template change.

### 2. Fetch shape — spike picks A or B (wiring identical either way)

The plan's **first task is a ≤15-minute `glab api graphql` introspection spike** to decide:

**A — aliased by-SHA (preferred, if `Repository.commit(ref:)` singular exists).** Mirror
`enrichGitHub`: chunk SHAs (≤50/query) and alias one commit per SHA:

```graphql
{ project(fullPath:"<owner>/<repo>"){ repository {
    c0: commit(ref:"<sha0>"){ author{ username } }
    c1: commit(ref:"<sha1>"){ author{ username } }
} } }
```

Exact SHAs, no pagination, symmetric with GitHub.

**B — `commits(ref:)` connection, date-scoped (confirmed fallback).** If the singular field does
not exist (the spike likely finds this), query the connection bounded to the release window and
paginate, matching against the passed SHA set (SHA match is authoritative; the date bound just
keeps the fetch small):

```graphql
{ project(fullPath:"<owner>/<repo>"){ repository {
    commits(ref:"<tag>", committedAfter:"<prev-tag-committedDate>", first:100, after:"<cursor>"){
      nodes { sha author { username } }
      pageInfo { endCursor hasNextPage }
} } } }
```

- `ref` = the release's tag (commits reachable from it, newest-first).
- `committedAfter` = the previous tag's committed date (`tagDate(prev)`); omitted for the first
  release. Bounds the fetch to the release window → typically one page.
- Paginate via `after`/`endCursor` while `hasNextPage` and the SHA set is not fully matched.
- Collect `sha → author.username` for nodes whose `sha` is in the passed set; `author == null`
  (email unlinked) → omit (no `by @`), mirroring GitHub.

Both variants call `glab api graphql -f query=<query>` through `runner.RunEnv(lc.APIEnv(), …)`,
so `GITLAB_HOST`/`GITLAB_TOKEN` (incl. self-hosted) work exactly as today. GraphQL transport-level
errors and a non-empty top-level `errors` array are wrapped and returned (the response is 200 with
an `errors` array on query failure, like GitHub).

Because `enrich()` is already called per-section with that section's SHAs, both variants are
naturally range-scoped, so `--regenerate` stays **O(pages)** across sections rather than O(commit).

### 3. Error handling / policy (unchanged surface)

Enrichment stays behind `enrichForRelease`, so `remote_metadata` is honored as-is: `required`
errors on failure (or when unenrichable), `optional` degrades (the reason surfaces as the step
sub-result), `disabled` skips, and `--force` downgrades `required`→`optional`. No new config.

### 4. Drop the GitLab regeneration rate-limit warning

`gitlabRegenWarning` (`warn.go`) exists solely because GitLab enrichment was O(commit). GitLab is
now batched, so the warning is obsolete and is removed (it was gated on `lc.Platform == "gitlab"`,
the only caller condition). `changelogGenResult` keeps its degrade branch; its success branch
returns no GitLab heads-up.

### 5. Data flow

```
git log (range SHAs)  →  enrich(lc, commits)  ─ gitlab ─→  enrichGitLab (glab api graphql)
                                                              → sha → author.username (batched)
     → enrichResult{authors}  →  overlayAuthorHandles(groups, authors)  →  buildCommit
     →  "… - ([sha](commit-url)) by @<username>"   (no "in [!N]" for GitLab)
```

## Error handling

- **GraphQL query error** (transport non-2xx, or 200 + `errors[]`): wrapped error from
  `enrichGitLab`, handled by `enrichForRelease` per `remote_metadata`.
- **`author == null`**: omitted (no `by @`) — normal, not an error.
- **SHA not returned** (e.g. not reachable from `ref`): absent from the map → no `by @` for that
  commit; not an error.
- **Malformed JSON**: wrapped parse error.

## Testing

Contract tests (`MockRunner`, no network):

- Query contract: `glab api graphql` invoked with the expected `-f query=…` (aliased-by-SHA for A,
  or connection-with-`committedAfter`/`first` for B) and `GITLAB_HOST`/`GITLAB_TOKEN` env.
- Parse: response → `sha → username` map; `author: null` omitted; unknown/absent SHA omitted.
- Pagination (B): a two-page response is followed via `after`/`endCursor` until `hasNextPage:false`
  or the SHA set is covered.
- End-to-end: `Generate` (release-notes + changelog modes) renders `by @<username>` and **no**
  `in [!N]` for GitLab; degrade path renders bare + sets `Degraded()`.
- The 9 existing GitLab MR contract tests are **removed** with the REST code they cover; the
  `TestGenerate_Enrich_GitLab` end-to-end is rewritten to assert `by @` (not `in [!7]`).
- Optional: extend the skippable real-CLI smoke test to accept the GraphQL query against a fixture
  repo (config-acceptance only), if feasible.

## Documentation & records to sync

- **ADR-0042 (new):** "GitLab commit-author attribution via batched GraphQL." Records: the
  GraphQL `commits.author.username` batched approach; dropping the O(commit) MR REST enrichment
  (no GraphQL commit→MR mapping); GitLab loses `in [!N]` + MR review-fields (breaking, pre-v1.0);
  the A/B fetch decided by spike.
- **Spec 05 (`docs/specs/05-generators-and-platforms.md`):** GitLab enrichment now yields
  commit-author handles only. Add a **user-facing limitation note**: "GitLab renders `by @author`
  but not MR references (`in [!N]`) or MR review-metadata — GitLab's GraphQL API exposes no
  commit→MR mapping, so MR-ref enrichment is deferred (see roadmap)." (Also surface this note
  wherever GitLab enrichment behavior is user-visible, e.g. the sample/README if applicable.)
- **Roadmap (`docs/tasks/native-generator-roadmap.md`):** mark **T150 `[x]` done**; add a new
  deferred follow-up task "GitLab MR-ref attribution (opt-in, O(commit) REST)" so the dropped
  capability is tracked, not lost.

## Migration (breaking, pre-v1.0)

GitLab users lose `in [!N]` and MR review-fields on the next `--regenerate` (or new release
section). No config change is required or offered. The change is loud in the ADR + spec + the
user-facing limitation note. GitHub and Azure are unchanged.
