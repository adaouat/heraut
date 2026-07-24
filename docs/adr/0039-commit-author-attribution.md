# ADR-0039: Commit-Author Attribution (native)

- **Status**: Accepted
- **Date**: 2026-07-18
- **Deciders**: bchatard
- **Builds on**: [ADR-0036](0036-unified-enrichment-model.md) (unified enrichment model — this ADR
  changes what drives one field of that model's rendering, not the model itself)

---

> **Update (2026-07-24):** The "GitHub only" scope of this cut has since been extended to all three
> platforms. GitLab commit-author `by @` shipped in
> [ADR-0042](0042-gitlab-graphql-enrichment.md) (roadmap T150), and Azure DevOps in roadmap T151 —
> rendered from the local git author email's local-part, since no Azure identity is resolvable from
> a git email (no new ADR; see the spec and `docs/tasks/native-generator-roadmap.md` Phase 2.10).
> The "Scope of this cut: GitHub only" limitation in the Decision and the GitLab/Azure regression in
> the Consequences below are therefore historical, not current behavior.

## Context

Native's commit line rendered `by @<login>` only when a commit had an associated pull request,
and the login came from the **PR author** (`blocks.tmpl`'s `commit` block gated the credit on
`{{ if .PR }}`; `buildCommit` set `Author.Username` from the PR's `AuthorLogin`). git-cliff, by
contrast, resolves the **commit author's** platform handle directly and shows it regardless of
whether a PR exists.

Dogfooding the `native` migration ([ADR-0038](0038-incremental-changelog.md)) surfaced the gap:
heraut's own commits land directly on `main` (no PRs — see `.claude/rules/workflow.md`'s
pre-v1.0 trunk-based workflow), so native's PR-only attribution produced **zero** `by @…` credit
in heraut's own `CHANGELOG.md`, where the previous git-cliff output carried `by @bchatard`
throughout. Direct-commit workflows are exactly the case native's attribution model missed.

Full design: [`docs/superpowers/specs/2026-07-17-commit-author-attribution-design.md`](../superpowers/specs/2026-07-17-commit-author-attribution-design.md).

## Decision

Credit the **commit author** on every commit line, independent of any associated pull request.
A PR, when present, now contributes only its reference link.

- `by @<commit-author handle>` renders whenever the handle resolves; ` in [#N](url)` is appended
  when the commit also has an associated PR. The PR's own author no longer drives the credit.
- When the committer differs from the PR author (e.g. someone else merges a squashed PR, or
  rebases and reauthors a commit), **the committer is credited** — matching git-cliff's behavior.

**Mechanism (GitHub only, this cut).** Native's GitHub enrichment already issues one batched
GraphQL query keyed per commit SHA (`object(oid:"<sha>"){ ...on Commit{ associatedPullRequests(...) } }`,
aliased, up to 50 SHAs/query — [ADR-0034](0034-native-remote-enrichment.md)). The `Commit` node
also exposes `author { user { login } }` — the GitHub user linked to the commit's author email.
`prFragment` (`internal/generators/native/enrich_github.go`) now selects that field alongside
`associatedPullRequests`, so `enrichGitHub` resolves a `sha → authorHandle` map for **every** SHA
in the **same query, at zero extra API calls**. That map is threaded through `enrichForRelease` →
`renderRelease`/`renderChangelogSection`/`renderReleaseNotes` → `overlayAuthorHandles`, which
stamps each grouped commit's `AuthorHandle` before `buildCommit` reads it into
`Author.Username`.

**Unlinked author / offline → no `by @`.** When GitHub's `author.user` is `null` (the commit's
author email isn't linked to any GitHub account), no entry lands in the handle map and the line
renders with no credit — same as the fully offline / `remote_metadata: disabled` path. No name
is guessed and no placeholder is substituted; the git author name/email still feed the local
contributors tier unchanged (ADR-0036).

**Scope of this cut: GitHub only.** `enrich_gitlab.go` and `enrich_azure.go` are unchanged and
continue to return no author-handle data, so GitLab and Azure commit lines render **no `by @`**
even when a PR/MR link is present — a visible change from the previous PR/MR-author attribution
on those two platforms. This is deliberate, not an oversight:

- **GitLab** — whether GitLab's GraphQL API can resolve a commit's author as a platform user (and
  whether it can be batched the way the GitHub query is) is unconfirmed against the live schema.
  Deferred to a follow-up that starts with a schema spike.
- **Azure DevOps** — resolving a commit author to an Azure identity needs a separate identity
  lookup this cut does not add.

Both are tracked as open follow-ups in `docs/tasks/native-generator-roadmap.md` (Phase 2.10).

## Consequences

- Closes heraut's own dogfooding gap on GitHub at no added API cost — the handle rides the
  existing batched query.
- The built-in `commit` block's attribution source moves from the PR author to the commit
  author, matching git-cliff. Any project relying on native's previous PR-author-driven credit
  (visible only when committer == PR author, the common squash-merge case) sees no change in the
  common case, and correct behavior in the squash-merge-by-someone-else case.
- **GitLab and Azure regress relative to git-cliff parity**: those two platforms rendered
  PR/MR-author credit before this ADR and render none now, until their respective follow-ups
  land. No regression relative to *native's own* prior output on those platforms beyond the loss
  of PR-author credit — the PR/MR reference link (`in [#N]` / `in !N`) is unaffected.
- Existing changelog history is unaffected: per [ADR-0038](0038-incremental-changelog.md)'s
  incremental model, handles appear on the next release's section (or on `--regenerate`), never
  retroactively rewritten.

## Alternatives considered

- **Keep PR-author attribution, add commit-author as a fallback only when no PR exists.**
  Rejected: this preserves the committer-≠-PR-author mismatch (crediting the wrong person when a
  maintainer merges someone else's squashed PR under their own account) and diverges from
  git-cliff's simpler, single-source rule.
- **Defer until GitLab/Azure parity is ready, ship all three platforms together.** Rejected: the
  GitHub mechanism is free and already dogfooding-blocking; gating it on an unscoped GitLab
  schema spike would delay a zero-cost fix for an unrelated reason.

## References

- Design spec: [`docs/superpowers/specs/2026-07-17-commit-author-attribution-design.md`](../superpowers/specs/2026-07-17-commit-author-attribution-design.md)
- [ADR-0036](0036-unified-enrichment-model.md) — unified enrichment model this ADR builds on
- [ADR-0034](0034-native-remote-enrichment.md) — native remote enrichment mechanism (batched GraphQL query, unchanged transport)
- [ADR-0038](0038-incremental-changelog.md) — incremental changelog; the dogfooding migration that surfaced this gap
- `docs/tasks/native-generator-roadmap.md` — Phase 2.10 task breakdown and follow-up tasks (GitLab, Azure)
