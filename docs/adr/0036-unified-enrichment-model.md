# ADR-0036: Unified cross-platform enrichment model

- **Status**: Accepted
- **Date**: 2026-07-03
- **Deciders**: bchatard
- **Supersedes**: [ADR-0034](0034-native-remote-enrichment.md)'s first-timer mechanism
  (GitHub `authorAssociation`) and the GitLab T137 earliest-MR-per-author approach. ADR-0034's
  transport decisions (`gh api` / `glab api` via `port.Runner`) and ADR-0035 (Azure native
  `net/http`) stand.

---

## Context

Phase 2 (T127 GitHub, T128 GitLab, T129 Azure) and the T137 GitLab follow-up landed native
enrichment platform-by-platform, and the result showed three cracks — recorded in full in the
design spec
[`docs/superpowers/specs/2026-07-03-unified-enrichment-model-design.md`](../superpowers/specs/2026-07-03-unified-enrichment-model-design.md):

1. **Three divergent first-timer mechanisms.** GitHub read the GraphQL `authorAssociation`
   field; GitLab issued a per-author "earliest merged MR" query (T137, one extra `glab api` call
   per distinct release author); Azure had none — T141 was left open because Azure PRs carry no
   `authorAssociation` and mirroring GitLab's per-author query would be net-new cost for the
   lowest-value platform. Same concept, three implementations, one of them missing.
2. **No local/offline enrichment.** With `remote_metadata: disabled` native rendered bare, even
   though git already carries everything needed for contributor / first-timer data (author name
   + email).
3. **No common data shape.** Each platform's driver returned its own ad hoc struct (`prInfo`),
   which also conflated PR data with a boolean `FirstTimer` field. There was no schema to hand a
   future user-customizable template.

## Decision

Adopt a **two-tier enrichment model** with a **flat, platform-agnostic schema**.

### Local tier — always runs, git-only

`internal/generators/native/contributors.go`:

- `authorsBefore(runner, prev)` — one `git log <prev> --format=%ae`, returning the set of author
  emails reachable from the previous release tag. An empty `prev` (first release) short-circuits
  to an empty set with no git call, so every release author counts as new.
- `collectContributors(commits, before, prs)` — walks the release's commits, dedupes by author
  email (first-seen order), and marks `IsFirstTime = email ∉ before`. When a PR is known for that
  author's first contributing commit (correlated by SHA against the remote tier's map), its
  handle/number/URL are overlaid onto the `Contributor`.

This tier is independent of `remote_metadata` and runs offline. `first_time` is now **one
git-email computation**, identical online and offline, replacing three per-platform mechanisms.

### Remote tier — normalizes into the common schema

`enrich_github.go` / `enrich_gitlab.go` / `enrich_azure.go` (dispatched by `enrich.go`) each
return `map[string]PullRequest`, the common model defined in `internal/generators/native/model.go`:

```go
type Author struct {
    Name     string // git author name — always (local)
    Email    string // git author email — always; the first_time identity key
    Username string // platform handle, e.g. "octocat" — remote only, "" offline
}

type PullRequest struct {
    Number      int
    URL         string
    AuthorLogin string
    RefPrefix   string // "#" GitHub/Azure, "!" GitLab — set per-platform at fetch time
    Title       string
    Labels      []string
    Platforms   map[string]any // platform-unique escape hatch
}

type Contributor struct {
    Author      Author
    IsFirstTime bool
    PR          *PullRequest // their first PR in this release; nil offline
}
```

- **`Title` and `Labels` are common fields**, fetched for all three platforms — GitHub via the
  GraphQL `associatedPullRequests` fragment, GitLab from the per-commit merge-request response,
  Azure best-effort from the `pullrequestquery` payload. They unlock label-driven consumers
  (future user templates) without a platform-specific branch.
- `Platforms` is defined as part of the schema but **left empty by this epic** — there is no
  consumer until user-customizable templates land (see below). Populating
  `Platforms["github"] = {review_decision, draft, …}` etc. is deliberately deferred, not a gap.
- This replaces the old `prInfo` struct and its `FirstTimer bool` field, removed entirely.

### Rendering behavior (contained by design)

- **Commit line** (`by @login in [#N](url)`) stays **remote-only** — offline lines are unchanged
  from Phase-1 output.
- **New Contributors block** — the built-in template (`buildContributorViews` in `render.go`)
  renders a contributor only when `Author.Username != ""`, i.e. only when remote enrichment
  resolved a handle. `Contributor{Author, IsFirstTime}` is nonetheless *always* populated by the
  local tier, ready for offline user templates. Net effect: **offline built-in output is
  unchanged**; the only visible change is online, where New-Contributors membership now derives
  from git first-appearance instead of `authorAssociation` / earliest-MR.

### Fat injection → exposed model (bridge to the future user-templates task)

Native renders with the fat-injection / thin-template pattern
([ADR-0022](0022-fat-injection-thin-templates.md)): the document skeleton is a Go `text/template`,
but each line's content is assembled in Go (`buildCommitLine`, `buildCommitBlock`,
`buildContributorViews`) and injected as a finished string, so the template never branches on
`.PR.Number` or `.Author.Username`. This epic keeps that pattern — the line-builders simply read
`Author`/`PullRequest`/`Contributor` instead of `prInfo`.

This is exactly the wall a future **user-customizable templates** task will hit: a user cannot
reformat a line they only receive pre-baked. That task will need to expose this normalized model
directly to the template (`.Author.Username`, `.PR.Labels`, …) and move branching from Go into the
template. Defining the schema now — flat core fields plus the `Platforms` escape hatch — is what
makes that future shift tractable: this model *is* the template-facing contract that task will
expose, chosen with that consumer in mind.

## Known limitation

Contributors are currently derived from **all** of the release's raw commits, before
`rendering.excludes` type-filtering is applied (`collectContributors` in `contributors.go` is
called with `toParsedCommits(commits)` — the unfiltered `collectCommits` output — not the grouped,
excludes-filtered commits `groupCommits` produces). Consequence: a bot's first-ever commit that
happens to be an excluded type (e.g. `chore(deps)` from Renovate/Dependabot, excluded by default —
[ADR-0033](0033-native-config-model.md)) can still surface that bot in the "New Contributors"
block, even though none of its commits render in the changelog body. Bot / excluded-commit
filtering for the contributor computation is a **deferred follow-up**, not addressed by this ADR.

## Consequences

- **Removed**: `prInfo.FirstTimer`; GitHub's `authorAssociation` GraphQL field and its parsing;
  GitLab's `markGitLabFirstTimers` + `gitLabEarliestMergedMR` (T137) and their per-author `glab
  api` calls and tests. One local git computation replaces three divergent, platform-specific
  mechanisms.
- **T141 (Azure "New Contributors") is resolved**, not implemented: Azure gets the block for free
  through the local tier's git-based `first_time` — no per-author Azure API query is needed. The
  "reconsider" decision pending in the roadmap is closed by this design, not by new Azure code.
- GitLab and Azure each save an API round trip per distinct release author (GitLab's earliest-MR
  query is gone entirely; Azure never needed one).
- Golden snapshots: online output can shift where New-Contributors membership differs
  (git-based vs the old `authorAssociation` / earliest-MR signal) — this diff was reviewed and
  re-baselined deliberately, not blanket-accepted. Offline golden output is unchanged.
- The model is now the schema a future user-customizable-templates task will expose to
  `text/template` — this ADR is that task's dependency, not its implementation.

## Alternatives considered

- **Keep the three per-platform first-timer mechanisms, just add one for Azure.** Rejected: it
  perpetuates the divergence (GitHub API-derived, GitLab query-derived, Azure copy-of-GitLab) this
  ADR exists to remove, and costs GitLab/Azure an extra API call per distinct author.
- **Populate `Platforms.<name>` now** (GitHub `review_decision`/`draft`, GitLab `squash`, Azure
  `merge_status`, …). Deferred: there is no consumer until user-customizable templates land: the
  built-in template only needs the common fields. Adding it now would be speculative surface with
  no test coverage driving it.
- **Filter bot/excluded commits out of the contributor computation now.** Deferred (see "Known
  limitation" above) to keep this epic scoped to the model unification; it is a narrower,
  separable follow-up once the model has shipped and been observed on a real repo.

## References

- Design spec: [`docs/superpowers/specs/2026-07-03-unified-enrichment-model-design.md`](../superpowers/specs/2026-07-03-unified-enrichment-model-design.md)
- [ADR-0022](0022-fat-injection-thin-templates.md) — fat-injection / thin-template pattern
- [ADR-0023](0023-remote-metadata-policy.md) — `remote_metadata` policy this model's remote tier honors unchanged
- [ADR-0033](0033-native-config-model.md) — `commits`/`rendering` config, incl. default `rendering.excludes`
- [ADR-0034](0034-native-remote-enrichment.md) — native remote enrichment mechanism (transport unchanged; first-timer mechanism superseded here)
- [ADR-0035](0035-azure-enrichment-native-http.md) — Azure native `net/http` transport (unchanged; its deferred first-timer note is resolved here)
- `docs/tasks/native-generator-roadmap.md` — Phase 2.7 task breakdown and completion notes
