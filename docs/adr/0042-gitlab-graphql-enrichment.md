# ADR-0042: GitLab enrichment via batched GraphQL

- **Status**: Accepted
- **Date**: 2026-07-23
- **Deciders**: bchatard

---

## Context

Native rendered `by @<handle> in [#N]` for GitHub but not GitLab. GitLab's enrichment made a
per-commit REST call to `/repository/commits/:sha/merge_requests` — O(commit), slow and
rate-limit-prone on large repos — and never resolved the commit author's GitLab handle, so GitLab
carried no `by @`. GitLab GraphQL exposes no commit→MR field, but a spike found two batched paths:
`repository.commits(ref:).nodes.author.username` (commit-author handles) and `mergeRequests.nodes`
with `mergeCommitSha` / `commits.nodes.sha` (invertible to a `commitSha → MR` map; no
squashed-commit SHA field exists — only a `squashOnMerge` bool).

## Decision

GitLab enrichment uses two batched `glab api graphql` connection queries:

1. **Commit authors** — `commits(ref: <range tip>, committedAfter: <oldest range date>)`, paginated,
   filtered to the range SHAs → `by @<commit author>`.
2. **MR references** — `mergeRequests(state: merged, mergedAfter: <oldest range date>)`, paginated,
   inverted by indexing each MR under its `mergeCommitSha` and each source `commits.nodes.sha`
   (covering merge-commit, squash-with-merge-commit, and fast-forward merges) → `in [!N]` plus MR
   review-metadata (`createdAt`, `mergeUser`, `mergedAt`, labels, title). GitLab GraphQL exposes no squashed-commit
   SHA (only a `squashOnMerge` bool), so a squash+fast-forward MR matches no target commit and that
   commit renders no ref — a graceful omission.

SHA match is authoritative; the date bounds only cap pagination. A commit with no MR renders no
`in [!N]`; an unlinked author renders no `by @`. The per-commit MR REST enrichment is removed, so
GitLab is now O(pages), and the `--regenerate` GitLab rate-limit warning is dropped.

## Consequences

- GitLab reaches parity with GitHub — `by @<commit author> in [!N]` with MR review-metadata — at a
  fraction of the API cost.
- Enrichment stays on the `glab` CLI transport; Phase 3 (raw-HTTP clients) would port it later.
- `by @` credits the *commit* author (ADR-0039), not the MR author.
