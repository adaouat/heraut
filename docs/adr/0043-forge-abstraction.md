# ADR-0043: Forge abstraction + unified `forges:` config

- **Status**: Accepted
- **Date**: 2026-07-24
- **Deciders**: bchatard
- **Extends / supersedes**: [ADR-0006](0006-config-naming-generator-platform.md) (naming),
  [ADR-0020](0020-platform-base-url.md) (`base_url`),
  [ADR-0023](0023-remote-metadata-policy.md) (remote-metadata policy),
  [ADR-0025](0025-multi-instance-platforms.md) (multi-instance platforms),
  [ADR-0026](0026-azure-devops-metadata-remote.md) (`changelog.remote`),
  [ADR-0034](0034-native-remote-enrichment.md) (native enrichment via CLIs),
  [ADR-0035](0035-azure-enrichment-native-http.md) (Azure native http),
  [ADR-0039](0039-commit-author-attribution.md) (commit-author attribution),
  [ADR-0040](0040-changelog-remote-native-base-url.md) (`changelog.remote` for native + `base_url`),
  [ADR-0041](0041-remote-metadata-required-enforcement-and-force.md) (`remote_metadata: required` + `--force`),
  [ADR-0042](0042-gitlab-graphql-enrichment.md) (GitLab GraphQL enrichment)

---

## Context

Three problems, one root cause — the "remote" heraut talks to is not a first-class,
self-configuring concept:

1. **`CI_JOB_TOKEN` cannot enrich.** GitLab enrichment is 100% GraphQL (ADR-0042), and GitLab's
   GraphQL API structurally rejects job tokens ("You cannot use job tokens to authenticate GraphQL
   requests"). `glab` also only ever sends `PRIVATE-TOKEN`, never the `JOB-TOKEN` header a job
   token requires. So in GitLab CI, the free, always-present `CI_JOB_TOKEN` produces no
   attribution and no MR links — the user must manually create a Personal/Project Access Token.
2. **Config naming mismatch.** Enrichment is configured under `changelog.remote` ("remote"),
   publishing under `release.platforms` ("platform"). Both describe the same underlying thing — a
   forge heraut talks to — under two names, in two places, with overlapping fields (`base_url`,
   `token_env`, `project`/`repository`).
3. **No clean extension point for new forges.** Enrichment lives as per-platform free functions
   (`enrichGitHub` / `enrichGitLab` / `enrichAzure`) dispatched in a `switch`; link-building is
   scattered; identity/auth resolution is ad hoc. Adding Gitea/Bitbucket/Forgejo means touching
   many call sites rather than implementing one interface.

Full problem statement, goals, spike findings, and phasing are recorded in the design spec:
[`docs/superpowers/specs/2026-07-24-forge-abstraction-design.md`](../superpowers/specs/2026-07-24-forge-abstraction-design.md).

## Decision

Introduce a `port.Forge` interface — one code-hosting platform heraut talks to, resolving its own
identity, building web links, and fetching per-commit PR/MR enrichment:

```go
type Forge interface {
    Type() string
    Identity() ForgeIdentity
    CommitURL(sha string) string
    ChangeURL(number int) string
    CompareURL(from, to string) string
    Enrich(commits []Commit) (Enrichment, error)
}
```

`ForgeIdentity{Type, Host, APIURL, Project, Token, TokenKind, APIMode}` carries the resolved
connection facts; `TokenKind` (`TokenNone | TokenJob | TokenPrivate`) selects the auth header a
transport must send. `Enrichment{PRs, Authors}` and `Commit{Hash, Author, Email, Date}` lift the
native generator's per-commit remote-data shape into `port`, so a forge implementation owns
**remote data** and the generator narrows to **rendering**. One implementation per platform type
lives in a new `internal/forge/{gitlab,github,azure}` package.

**Config unifies around a top-level `forges:` list** (connection only — `name`, `platform`,
`project`/`repository`, `base_url`, `api_url`, `api_mode`, `token_env`), replacing
`changelog.remote` and the connection half of `release.platforms`. Publish behavior moves to
`release.targets:` (draft, prerelease, assets), each entry referencing a forge by name. Enrichment
source and policy become `commits.enrichment_forge` (which forge resolves PR/MR metadata) and
`commits.enrichment_policy` (renamed from `remote_metadata`), both global settings governing
changelog and release notes uniformly.

**Identity is auto-resolved, field by field, with explicit config always winning:** explicit
`forges:` config → CI environment (`GITLAB_CI`/`GITHUB_ACTIONS`/`TF_BUILD` markers pin the type
unambiguously) → `git remote get-url origin` (host + owner/repo, type inferred for known public
hosts) → offline. Auto-detection must resolve exactly one forge type or fails loud — it never
guesses:

```
ambiguous forge: detected candidates [gitlab, github] and no CI/origin to disambiguate.
Declare a `forges:` block to choose explicitly.
```

**GitLab gets a native `net/http` forge** (no `glab` for GitLab enrichment), with the auth header
selected by `TokenKind`: Job → `JOB-TOKEN`, Private → `PRIVATE-TOKEN`. `api_mode: rest` (default)
works with `CI_JOB_TOKEN` out of the box — commit-author `by @` renders the local git author name
(REST carries no linked handle), MR association comes from the job-token-allowed
`GET /projects/:id/repository/commits/:sha/merge_requests`. `api_mode: graphql` (opt-in) requires a
`read_api` token and renders the linked `@username` (today's ADR-0042 fidelity); supplying only a
job token with `graphql` is a validation error, since GraphQL structurally rejects job tokens.

GitHub and Azure keep their current transports (`gh api graphql`, native `net/http`) in this phase
and are adapted to feed the resolver; both migrate onto `port.Forge` in a later phase (see
Phasing).

## Consequences

- **Breaking config change, acceptable pre-v1.0.** `changelog.remote`, `release.platforms`, and
  `commits.remote_metadata` are removed. Rather than a silent alias, the loader emits a clear
  migration error mapping the old keys to `forges:` / `release.targets:` /
  `commits.enrichment_policy`, with a before/after snippet in the hint, so existing configs fail
  fast with actionable guidance. `schema.json`, `docs/heraut.sample.yml`, and the specs update in
  lockstep with each phase.
- **Zero-config in CI**: an empty (or absent) `forges:` block plus `CI_JOB_TOKEN` produces a fully
  enriched changelog/release notes with no token creation, no host/project config — the pain this
  ADR exists to remove.
- **One concept, one name**: `forges:` (connection) is orthogonal to `release.targets:` (publish)
  and `commits.enrichment_forge`/`enrichment_policy` (enrichment), instead of the prior
  `changelog.remote` / `release.platforms` overlap.
- **Extension point for new forges** narrows to implementing one `port.Forge` per platform, instead
  of touching a `switch` and scattered link-building call sites.
- Publishing to multiple forges, or selecting an enrichment source among several forges, always
  requires an explicit `forges:` block — zero-config is single-forge by definition.
- This ADR lands the design; each phase (P1–P4) is its own implementation plan, not implemented
  here:
  - **P1** — GitLab-first, end-to-end zero-config: `port.Forge` + `internal/forge` + `forges:` /
    `release.targets:` config + resolution + the GitLab native forge + links + migration error.
  - **P2** — GitHub and Azure migrate onto `port.Forge`; the enrichment `switch` retires.
  - **P3** — Publishing folds into `Forge`; the publish HTTP transport (stdlib vs SDK) is decided
    by P3's own ADR.
  - **P4** — `heraut init` wizard generates `forges:` / `release.targets:` / `commits.enrichment_forge`,
    deliberately last so the wizard targets a config shape already battle-tested in P1–P3.

## Alternatives considered

- **Keep `changelog.remote` and `release.platforms` separate, just add a GitLab native transport.**
  Rejected: it fixes problem 1 (job tokens) without touching the naming mismatch or the extension
  problem — the root cause survives, and a future forge still means touching a `switch` plus two
  config blocks.
- **Alias the old keys instead of a hard migration error.** Rejected pre-v1.0: an alias hides the
  semantic split between connection (`forges:`) and publish behavior (`release.targets:`) behind a
  guess, and a config that silently maps old fields onto the new shape can misplace `assets`/`draft`
  fields that used to live on `Platform` and now belong on a release target. A loud, actionable
  migration error is cheaper than a silently wrong mapping.
- **Have `glab` carry the job token.** Not possible: `glab`/`GITLAB_TOKEN` only ever sends
  `PRIVATE-TOKEN`, which GitLab rejects for job tokens — a native client that sets the correct
  header is required regardless of config shape.
- **Skip auto-detection; require explicit `forges:` always.** Rejected: it reintroduces the manual
  setup step in CI this ADR exists to remove. Auto-detection with a hard ambiguity error keeps
  zero-config safe without silently guessing.

## References

- Design spec: [`docs/superpowers/specs/2026-07-24-forge-abstraction-design.md`](../superpowers/specs/2026-07-24-forge-abstraction-design.md)
- [ADR-0023](0023-remote-metadata-policy.md) — `remote_metadata` policy, renamed `commits.enrichment_policy` here
- [ADR-0025](0025-multi-instance-platforms.md) — multi-instance same-platform releases, preserved by `release.targets:`
- [ADR-0034](0034-native-remote-enrichment.md) — native remote enrichment transport (`gh api` / `glab api` via `port.Runner`); GitLab's transport is replaced here
- [ADR-0035](0035-azure-enrichment-native-http.md) — Azure native `net/http` transport, unchanged in this phase
- [ADR-0042](0042-gitlab-graphql-enrichment.md) — GitLab GraphQL enrichment logic, reused by the `api_mode: graphql` path
