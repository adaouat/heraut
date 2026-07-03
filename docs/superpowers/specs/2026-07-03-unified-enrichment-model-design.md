# Unified Cross-Platform Enrichment Model — Design

- **Date**: 2026-07-03
- **Status**: Approved (brainstorm) — pending implementation plan
- **Scope**: the native generator's enrichment **data model** (transport internalization is
  explicitly out of scope — see below)

---

## Problem

Native enrichment grew platform-by-platform (T127 GitHub, T128 GitLab, T137 GitLab
first-timers, T129 Azure), and it shows:

1. **Three divergent first-timer mechanisms.** GitHub reads the API's `authorAssociation`;
   GitLab issues a per-author "earliest merged MR" query (T137, N extra API calls); Azure has
   none (T141 stalled — git-cliff has no Azure first-timer logic to mirror). Same concept, three
   implementations, one of them missing.
2. **No local/offline enrichment.** With `remote_metadata: disabled`, native renders bare —
   even though git already carries author name/email, which is enough for contributor and
   first-timer data.
3. **No common data shape.** Each platform returns its own struct (`prInfo` is a lowest common
   denominator that also conflated PR data with a `FirstTimer` bool). There is no normalized
   model to hand a future **user-customizable template**.

## Decision

Adopt a **two-tier enrichment model** with a **flat, platform-agnostic schema**.

- **Local tier** — always runs, git-only, independent of `remote_metadata`. Produces authors and
  first-timer status from git history.
- **Remote tier** — runs under `remote_metadata: optional|required`, per the existing policy and
  scope (ADR-0034 §5: new/unreleased section only). Fetches PR/MR metadata and **normalizes it
  into the common schema**, overlaying the local tier.

`first_time` is a **local git computation keyed on git author email** — identical online and
offline, and platform-agnostic. This lets us delete all three per-platform first-timer paths.

### Out of scope (deliberately)

- **Transport internalization** (raw HTTP / Go SDKs replacing `gh`/`glab`): a separate, later
  ADR. The CLIs stay required for *publish* regardless, so internalizing enrichment alone removes
  no binary; the payoff needs publish internalization too (ADR-0032 Phase 3). This design is
  **transport-agnostic** — it works with today's `gh api` / `glab api` / Azure `net/http`.
- **User-customizable templates**: a later task. This design defines the *schema* that task will
  expose, and documents the bridge (see "Fat injection → exposed model").
- **Local tier on every changelog section** (historical enrichment): deferred behind a future
  flag. This design scopes the local tier to the release being generated / the new changelog
  section, matching the remote scope, so a full regeneration leaves historical sections
  byte-identical.

---

## Data model

Flat common core; platform-unique fields under a `Platforms` escape hatch.

```go
// Author — a contributor identity: git-first, with an optional remote handle.
type Author struct {
    Name     string // git author name — always (local)
    Email    string // git author email — always; the first_time identity key
    Username string // platform handle, e.g. "octocat" — remote only, "" offline
}

// PullRequest — normalized PR/MR: common fields + per-platform escape hatch.
type PullRequest struct {
    Number    int
    URL       string
    Title     string
    Labels    []string
    Platforms map[string]any // platform-unique bits, e.g. Platforms["github"]
    // RefPrefix "#"/"!" is derived from the platform at render time, not stored.
}

// Contributor — a per-release contributor, for the "New Contributors" block.
type Contributor struct {
    Author      Author
    IsFirstTime bool
    PR          *PullRequest // their first PR in this release — nil offline
}
```

- **`title` + `labels` are common fields** (universal across GitHub/GitLab/Azure, already in the
  fetched payloads). `labels` unlocks label-driven template sections/filtering.
- Platform-unique data (GitHub `review_decision`/`draft`, GitLab `squash`, Azure `merge_status`,
  …) lives under `Platforms.<name>`.
- Internally this **replaces `prInfo`**. Per commit we keep `rawCommit`/`parsedCommit` and attach
  an optional `*PullRequest`; the merged `Author` is `{git name+email} ⊕ {PR username}`,
  correlated by commit SHA.

---

## Flow

```
1. LOCAL  (always, git-only, any remote_metadata):
   collectCommits → rawCommits (name, email, subject, body, date)
   authorsBefore(prev) → set of author emails that authored before this release   [1 git call]
   Contributors = distinct release authors; IsFirstTime = email ∉ authorsBefore

2. REMOTE (optional/required only, per policy, new section only):
   enrich<Platform> → sha → PullRequest{Number, URL, Title, Labels, Platforms}
   overlay: attach the PR to each commit; set Contributor.Username + .PR from the
   PR of that author's first release commit (correlate by SHA)

3. RENDER: commit lines + New Contributors from the same model
```

`first_time`: `authorsBefore(runner, prev)` runs one `git log <prev> --format=%ae` (empty set for
a first release → everyone is new); a release author is a first-timer when their email is not in
that set. De-duplicated by email.

---

## Code impact

**Fetchers → return `PullRequest`:**

- `enrich_github.go`: GraphQL query gains `title` + `labels`; **drop `authorAssociation`** parsing.
- `enrich_gitlab.go`: **delete `markGitLabFirstTimers` + `gitLabEarliestMergedMR`** (T137) and
  their per-author calls; fetch MR `title` + `labels`.
- `enrich_azure.go`: add `title` (present on the PR) and `labels` (Azure PR tags — best-effort;
  may need an expand, can start empty).
- `enrich.go`: dispatch unchanged; the returned map is now `sha → PullRequest`.

**New local tier** (`contributors.go`): `authorsBefore(runner, prev)` and
`buildContributors(commits, before, prMap) → []Contributor` (git-derived `is_first_time`, PR
overlaid by SHA). `prev` threads in from the generator (it already resolves it).

**Render:** `prInfo → PullRequest` through `buildCommitLine` / `buildCommitBlock`; the render-side
contributors helper consumes `[]Contributor` instead of scanning a `FirstTimer` flag.

**Removals:** `prInfo.FirstTimer`; GitHub `authorAssociation` logic; GitLab T137 functions + tests;
the Azure first-timer idea (T141).

---

## Rendering behavior (contained by design)

- **Commit line:** `by @login in [#N](url)` stays **remote-only** → offline lines are unchanged
  (no per-line author noise; the author lives in the *model* for user templates).
- **New Contributors block:** the built-in template renders it only when contributors carry
  remote data (online), so **offline built-in output does not change**. `Contributor{Author,
  IsFirstTime}` is nonetheless *always* populated, ready for offline user templates.
- **Net visible change, online:** block membership now derives from git first-appearance instead
  of `authorAssociation` / earliest-MR (more consistent, arguably more accurate). **Offline
  built-in output: unchanged.**

### Fat injection → exposed model (bridge to the templates task)

Native renders with the **fat-injection / thin-template** pattern (ADR-0022): the document
skeleton is a Go `text/template` (`changelog.tmpl` / `release_notes.tmpl`), but each line's
*content* is assembled in Go (`buildCommitLine` etc.) and injected as a finished string
(`commitView{Line}`), so the template is branch-free and never sees `.PR.Number` or
`.Author.Username`.

This epic keeps that pattern — the Go line-builders simply read the new model instead of `prInfo`.
But it is exactly the wall the **future user-customizable templates** task hits: a user cannot
reformat a line they only receive pre-baked. That task will need to **expose the normalized model
to the template** (`.Author.Username`, `.PR.Labels`, …) and move branching from Go into the
template. Defining the schema now is what makes that future shift tractable — the model in this
spec is the template-facing contract, chosen with that consumer in mind.

---

## Testing

- **Local tier:** unit tests for `authorsBefore` and first-time (first release, dedup, email
  membership).
- **Fetchers:** contract tests updated — argv unchanged; the parsed struct gains `title`/`labels`;
  `FirstTimer` removed. **Remove the T137 tests.**
- **Golden snapshots (T126):** online output may shift where first-timer membership differs
  (git-based vs `authorAssociation`) — re-baseline deliberately, with the diff reviewed. Offline
  goldens stay put (built-in offline output is unchanged).
- **Determinism:** no network (MockRunner / httptest as today); `authorsBefore` is a git call via
  the runner.

---

## Rollout — Phase 2.7 "Unified enrichment model"

Its own ADR (superseding the first-timer portions of ADR-0034 / ADR-0035). Task breakdown:

1. Model + local tier (`Author`/`PullRequest`/`Contributor`, `authorsBefore`,
   `buildContributors`); remove `FirstTimer`.
2. `enrich_github` → `PullRequest` (+title/labels, −authorAssociation).
3. `enrich_gitlab` → `PullRequest` (+title/labels, −T137).
4. `enrich_azure` → `PullRequest` (+title, labels best-effort).
5. Wire render / view-model + built-in templates; re-baseline goldens.
6. ADR + docs (spec 05; record the model as the future template schema and the fat-injection
   bridge).
