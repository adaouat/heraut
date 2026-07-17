# Commit-Author Attribution (native) — Design

- **Date**: 2026-07-17
- **Status**: Approved (brainstorm) — pending implementation plan
- **Scope**: give the `native` generator a `by @<commit author>` credit on changelog / release-notes
  commit lines, independent of pull requests. Closes the git-cliff→native regression surfaced when
  heraut dogfooded native (ADR-0038 migration): git-cliff attributed the **commit author**, native
  only attributes via an associated **PR**, so direct-commit workflows (heraut's own pre-v1.0 trunk)
  lost all `by @…`. Builds on the unified enrichment model ([ADR-0036](../../adr/0036-unified-enrichment-model.md)).

---

## Problem

Native's commit line renders `by @<login>` only when a commit has an associated pull request, and
the login comes from the **PR author** (`blocks.tmpl` gates it on `{{ if .PR }}`; `buildCommit`
sets `Author.Username = pr.AuthorLogin`). heraut commits land directly on `main` (no PRs), so the
native-generated `CHANGELOG.md` shows **zero attribution**, where git-cliff showed `by @bchatard`
throughout — git-cliff resolves the **commit author's** platform handle, a path native does not have.

## Decision

Credit the **commit author** on every commit line, and let a PR contribute only its reference link.

- `by @<commit-author handle>` whenever the handle resolves; then ` in [#N](url)` appended when the
  commit has an associated PR. (Rule chosen: *commit author always*, matching git-cliff — when
  committer ≠ PR author, the committer is credited.)
- The PR's own author no longer drives display; the PR supplies only `Number` / `URL` / `Ref`.

**Scope of this first cut: GitHub only.** GitLab and Azure are tracked follow-ups (see below).

## Data flow

Native's GitHub enrichment already issues one batched GraphQL query keyed per commit SHA
(`object(oid:"<sha>"){ ...on Commit{ associatedPullRequests(...) } }`, aliased, 50 SHAs/query). The
`Commit` node also exposes `author { user { login } }` — the GitHub user linked to the commit's
author email. Adding that selection resolves the commit-author handle for **every** SHA in the
**same query, at zero extra API calls**.

Enrichment therefore yields two things per release: the existing `sha → PullRequest` map, and a new
`sha → authorHandle` (`map[string]string`, empty entry when the email isn't linked to a user). To
avoid threading a second parallel map through the render call chain, both are bundled into one small
value carried from `enrichForRelease` through `renderRelease` / `renderChangelogSection` /
`renderReleaseNotes` / `buildRelease` to `buildCommit` (exact shape — a struct vs. an added
parameter — is the plan's call; a struct is recommended since these signatures already carry the PR
map plus several other args).

## Components

- **`enrich_github.go`** — extend the `prFragment` `Commit` selection with `author{user{login}}`;
  parse it into the `sha → authorHandle` map. The GitHub `enrichGitHub` (and the `enrichForRelease`
  orchestrator) return the bundled `{prs, authorHandles}`.
- **`enrich_gitlab.go` / `enrich_azure.go`** — return an **empty** author-handle map (no behavior
  change). Documented as best-effort/deferred.
- **`templatemodel.go` (`buildCommit`)** — set `tplCommit.Author.Username` from the author-handle
  map for the commit's SHA (was: the PR author's login). The PR still populates `.PR` for the link.
- **`blocks.tmpl` (`commit` block)** — change the attribution to:
  `{{ if .Author.Username }} by @{{ .Author.Username }}{{ end }}{{ if .PR }}{{ if .PR.Number }} in [{{ .PR.Ref }}]({{ .PR.URL }}){{ end }}{{ end }}`
  — commit-author credit is independent of the PR; the PR block contributes only the reference.

## Error handling & edge cases

- **Unlinked author** (GitHub returns `author.user == null`, e.g. an email with no GitHub account)
  → empty handle → **no `by @`** on that line (same as offline; no fake handle, no bare name).
- **Offline / `remote_metadata: disabled` / nil `lc`** → no enrichment → no handles → no `by @`
  (git author name/email still feed the contributors local tier, unchanged).
- **Enrichment degrade** (optional policy, fetch fails) → handles empty → no `by @`, as today.

## Out of scope (explicit)

- **GitLab & Azure commit-author handles.** GitLab REST email→user is privacy-restricted; whether
  GitLab GraphQL can resolve it (and batch it) is an open **spike** to run against the live schema
  (`glab api graphql` introspection / `/-/graphql-explorer`) — deferred to the follow-up. Azure needs
  a separate identity lookup. Until then both render no `by @` (unchanged).
- **The "New Contributors" block.** It also needs a handle (currently PR-overlay only); the same
  `sha → authorHandle` map could feed it so direct-commit repos get first-timer credit, but that is a
  separate follow-up (and moot for heraut's single-author history).
- **Re-attributing existing changelog history.** Handles appear on the next `--regenerate` (or on new
  releases going forward), per ADR-0038's incremental model — not retroactively rewritten here.

## Testing

- **Contract:** the `gh api graphql` query string includes `author{user{login}}` (assert the fragment).
- **Parse:** a response with `author.user.login` populates the handle map; `author.user == null` yields
  no entry.
- **Render:** commit with handle + no PR → `by @h`; handle + PR → `by @h in [#N](url)`; committer ≠ PR
  author → the **committer** handle is shown; unlinked author → no `by @`.
- **Platform:** GitLab/Azure enrichment returns an empty handle map → those commits render no `by @`.
- **Golden:** existing changelog/release-notes goldens pass `nil` enrichment → **byte-identical**; add
  a golden covering the commit-author line (handle + PR link) so the new format is locked.
- **Determinism:** MockRunner / httptest only; no network; offline path asserted.

## Consequences

- Closes heraut's own dogfooding gap on GitHub with no added API cost.
- The built-in `commit` block's attribution source moves from the PR author to the commit author —
  a deliberate, git-cliff-matching change; documented as an additive note to
  [ADR-0036](../../adr/0036-unified-enrichment-model.md) (or a short ADR-0039 if it warrants one at
  plan time).
- GitLab/Azure users see no attribution until their follow-ups land — no regression from today.
