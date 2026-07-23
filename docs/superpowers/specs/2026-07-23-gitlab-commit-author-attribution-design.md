# GitLab enrichment via batched GraphQL (commit authors + MR refs) — Design

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-07-23
- **Author**: bchatard (with Claude)
- **Related**: ADR-0034 (native remote enrichment), ADR-0036 (unified enrichment model), ADR-0039 (commit-author attribution, GitHub), roadmap T150 (this), Phase 3 (raw-HTTP clients)

---

## Problem

Native renders `by @<handle> in [#N]` for GitHub but for GitLab renders neither cheaply. GitLab's
enrichment (`enrich_gitlab.go`) makes a **per-commit** REST call to
`/repository/commits/:sha/merge_requests` — **O(commit)**, slow and rate-limit-prone on large
repos (observed live: 404s, connection resets, GitLab rate limits) — and it returns the MR but not
the commit author's GitLab handle, so there is no `by @` for GitLab at all.

A GraphQL spike (bchatard) established that GitLab's API can supply **both** signals, batched:

1. **Commit-author handles:** `project.repository.commits(ref:).nodes.author.username` — the commit
   author's GitLab handle for every commit in one paginated connection. `commits(ref:)` accepts
   `ref` (required), `committedAfter`/`committedBefore` (ISO8601 bounds), `first`/`after` (cursor
   pagination), `author`/`path`/`query` filters.
2. **MR references (inverted):** there is no commit→MR field, but `project.mergeRequests.nodes`
   exposes `iid`, `webUrl`, `author{username}`, `mergedAt`, `mergedBy{username}`, `labels`,
   `title`, **`mergeCommitSha`**, **`squashCommitSha`**, and **`commits.nodes.sha`**. Inverting the
   MR→commit relationship (index each MR by the SHA(s) that land on the target branch) yields a
   batched `commitSha → MR` map. `mergeRequests` accepts `state`, `mergedAfter`/`mergedBefore`,
   `targetBranches`, `first`/`after`.

So GitLab can reach full parity with GitHub (`by @author in [!N]` plus MR review-metadata) at
**O(pages), never O(commit)**.

## Goal

Replace GitLab's O(commit) per-commit MR REST enrichment with **two batched GraphQL connections** —
`repository.commits` for commit-author handles and `mergeRequests` for MR references — so GitLab
changelogs and release notes render `by @<commit author> in [!N]` with MR review-metadata, all
batched.

## Non-goals

- **GitLab "New Contributors" first-timer detection** — remains deferred (unchanged). The MR data
  now available could feed it later, but that is a separate task.
- **Azure commit-author handle** (roadmap T151) — separate; Azure is already native HTTP.
- **Phase 3 raw-HTTP clients** — this stays on the `glab` CLI transport (`glab api graphql`);
  Phase 3 will port it with all enrichment if/when it happens.
- **Combining both connections into a single GraphQL request** — allowed as an optimization but not
  required; two focused paginated functions are clearer (see Design §2).

## Design

### 1. `enrich_gitlab.go` returns both maps (like GitHub)

Replace `enrichGitLab(runner, lc, shas) (map[string]PullRequest, error)` (per-commit REST) with a
GraphQL implementation returning **both** a PR map and an author map:

```go
// enrichGitLab returns, for the given SHAs: sha → MR (references + review metadata) and
// sha → commit-author GitLab handle, via two batched glab api graphql connection queries.
// since bounds the fetch to the release window; the zero value means unbounded.
func enrichGitLab(runner port.Runner, lc *port.LinkContext, shas []string, since time.Time) (map[string]PullRequest, map[string]string, error)
```

`enrich()`'s gitlab branch mirrors GitHub, deriving `since` from the range commits it already has
(no extra git call): the oldest committed date in the range — an MR merged before that cannot have
a target-landing commit in the range.

```go
case "gitlab":
    prs, authors, err := enrichGitLab(g.runner, lc, shas, oldestCommitDate(commits))
    return enrichResult{prs: prs, authors: authors}, err
```

`oldestCommitDate` returns the minimum `rawCommit.Date` (committed date, `%cI`) over the range, or
the zero time when empty. The renderer already consumes both maps
(`by @{{.Author.Username}} in [{{.PR.Ref}}]({{.PR.URL}})`), so no template change.

### 2. Two batched connection fetches

**a. Commit-author handles** — `fetchGitLabAuthors`:

```graphql
{ project(fullPath:"<owner>/<repo>"){ repository {
    commits(ref:"<tag>", committedAfter:"<since>", first:100, after:"<cursor>"){
      nodes { sha author { username } }
      pageInfo { endCursor hasNextPage }
} } } }
```

Paginate via `after`/`endCursor`; collect `sha → author.username` for nodes whose `sha` is in the
passed set; `author == null` (email unlinked) → omit (no `by @`), mirroring GitHub. Stop when the
SHA set is covered or `hasNextPage:false`.

**b. MR references (inverted)** — `fetchGitLabMRs`:

```graphql
{ project(fullPath:"<owner>/<repo>"){
    mergeRequests(state: merged, mergedAfter:"<since>", first:100, after:"<cursor>"){
      nodes {
        iid webUrl title author{username} mergedAt mergedBy{username}
        labels{nodes{title}}
        mergeCommitSha squashCommitSha
        commits{nodes{sha}}
      }
      pageInfo { endCursor hasNextPage }
} } }
```

Invert to a `commitSha → PullRequest` map, indexing **each MR by every SHA that can land on the
target branch**: `mergeCommitSha` (merge-commit merges), `squashCommitSha` (squash merges), and
each `commits.nodes.sha` (fast-forward merges, where source commits land directly). Build
`PullRequest{Number: iid, URL: webUrl, Title, AuthorLogin: author.username, Labels, RefPrefix: "!",
MergedAt, MergedBy}`. Keep only entries whose SHA is in the passed set. Paginate until the SHA set
is covered or `hasNextPage:false`.

**SHA match is authoritative; `mergedAfter`/`committedAfter` only bound pagination.** A commit that
matches no MR (direct push, or an MR merged outside the date bound) renders **no** `in [!N]` — a
graceful omission, exactly like a GitHub commit with no associated PR. A commit whose author email
is unlinked renders no `by @`. Neither is an error.

Both fetches call `glab api graphql -f query=<query>` through `runner.RunEnv(lc.APIEnv(), …)`, so
`GITLAB_HOST`/`GITLAB_TOKEN` (incl. self-hosted) work as today. `project(fullPath:)` uses
`lc.Owner + "/" + lc.Repo`. GraphQL transport errors and a non-empty top-level `errors[]` (the
response is 200 with an `errors` array on query failure) are wrapped and returned.

Because `enrich()` runs per-section with that section's SHAs and `since`, `--regenerate` stays
**O(pages)** across sections, never O(commit).

### 3. Spike (plan Task 1, ≤30 min)

A `glab api graphql` introspection/run against the live instance confirms the field/arg names this
design assumes, before implementation: on `Repository.commits` — `committedAfter`, `after`/`first`,
`author.username`; on `Project.mergeRequests` — `state`, `mergedAfter`, `iid`, `webUrl`,
`mergeCommitSha`, `squashCommitSha`, `commits.nodes.sha`, `mergedBy`, `labels`. If a name differs,
the spike records the correction; the design shape is unchanged.

### 4. Remove the GitLab regeneration rate-limit warning

`gitlabRegenWarning` (`warn.go`) existed only because GitLab enrichment was O(commit). GitLab is now
batched, so it is removed (it was gated on `lc.Platform == "gitlab"`, the sole caller condition).
`changelogGenResult` keeps its degrade branch; its success branch returns no GitLab heads-up.

### 5. Data flow

```
git log (range SHAs + committed dates)  →  enrich(lc, commits)  ─ gitlab ─→  enrichGitLab(since=oldest date)
    ├─ fetchGitLabAuthors  →  sha → commit-author username   (batched, paginated)
    └─ fetchGitLabMRs      →  sha → MR (iid/url/author/review) via mergeCommitSha ∪ squashCommitSha ∪ commits.sha
  →  enrichResult{prs, authors}  →  overlayAuthorHandles + PR overlay  →  buildCommit
  →  "… - ([sha](commit-url)) by @<commit author> in [!<iid>](<webUrl>)"
```

## Error handling

- Query error (transport non-2xx, or 200 + `errors[]`): wrapped error from the fetch, handled by
  `enrichForRelease` per `remote_metadata` (`required` fails, `optional` degrades with the reason
  surfaced as the step sub-result, `disabled` skips, `--force` downgrades).
- `author == null` / commit not in any MR / SHA absent from a page: omitted, not an error.
- Malformed JSON: wrapped parse error.

## Testing

Contract tests (`MockRunner`, no network):

- `fetchGitLabAuthors`: query args (`committedAfter`, `first`), parse → `sha → username`, null
  author omitted, pagination followed via `after`/`endCursor`.
- `fetchGitLabMRs`: query args (`state: merged`, `mergedAfter`), inversion — a fixture MR maps its
  `mergeCommitSha`, `squashCommitSha`, and each `commits.nodes.sha` to the same MR; `iid`→`!N`,
  `webUrl`, MR author, review fields; pagination.
- `enrichGitLab`: both maps returned; only in-range SHAs kept.
- End-to-end `Generate` (changelog + release-notes): renders `by @<commit author> in [!<iid>]` for
  a GitLab commit; degrade path renders bare and sets `Degraded()`.
- The existing per-commit-MR REST contract tests are **replaced** by the GraphQL ones;
  `TestGenerate_Enrich_GitLab` is rewritten to assert `by @… in [!N]` from GraphQL.
- Optional: extend the skippable real-CLI smoke test for query acceptance, if feasible.

## Documentation & records to sync

- **ADR-0042 (new):** "GitLab enrichment via batched GraphQL." Records: commit-author handles from
  `commits.author.username`; MR references recovered by **inverting** `mergeRequests` →
  `mergeCommitSha`/`squashCommitSha`/`commits.sha` (no direct commit→MR field exists); the
  per-commit REST enrichment dropped; O(pages) not O(commit).
- **Spec 05 (`docs/specs/05-generators-and-platforms.md`):** GitLab enrichment now via GraphQL
  (authors + MR refs, batched); note the graceful omission (a commit not attributable to any MR
  renders no `in [!N]`).
- **Roadmap (`docs/tasks/native-generator-roadmap.md`):** mark **T150 `[x]` done** (GitLab
  commit-author + MR refs, batched GraphQL). No MR-refs follow-up is needed — they are included.

## Migration (behavior change, pre-v1.0)

GitLab enrichment changes transport (REST per-commit → batched GraphQL) but the **rendered output
is a superset** of before: `by @<commit author>` is new, `in [!N]` and MR review-fields are
preserved (now via GraphQL). Direct-push commits (no MR) render no `in [!N]`, as before. No config
change. GitHub and Azure are unchanged.
