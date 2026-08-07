# Héraut — Forge Abstraction Roadmap

> Status: Active
> Design: [`docs/superpowers/specs/2026-07-24-forge-abstraction-design.md`](../superpowers/specs/2026-07-24-forge-abstraction-design.md)
> ADRs: ADR-0043 (forge abstraction + config unification — written in T154) · extends/supersedes
> 0006, 0020, 0023, 0025, 0026, 0034, 0035, 0039, 0040, 0041, 0042
> Main roadmap: tracked as Phase 24 in [`roadmap.md`](roadmap.md)

This roadmap breaks the **forge abstraction** epic into incrementally shippable tasks. It lives in
its own file because the work is heavy and multi-phase; the main roadmap keeps only a Phase 24
pointer here.

A single top-level **`forges:`** list (a forge = one code-hosting platform heraut talks to) replaces
`changelog.remote` + `release.platforms`. A new **`port.Forge`** owns three responsibilities —
resolve its identity from the environment, build links, and fetch enrichment metadata. Identity
**auto-configures from CI env or git `origin`** (fail loud on ambiguity), so a changelog / release
notes render fully enriched **with zero config in CI**. GitLab gains a native `net/http` enricher
(REST default, `JOB-TOKEN`-aware) so `CI_JOB_TOKEN` enriches without a manually-created PAT — with
an opt-in GraphQL path (`api_mode: graphql`) for linked commit-author handles. Consumers reference a
forge by name: `commits.enrichment_forge` (enrichment source) and `release.targets[].forge` (publish
targets); `commits.remote_metadata` is renamed `commits.enrichment_policy`.

## Conventions

- Task IDs **continue the global sequence** (`T154+`) so they never collide with the main roadmap or
  the native-generator roadmap.
- This file is the **single source of truth** for task status. Same checkbox markers: `[ ]` not
  started, `[x]` done. Follow the two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)):
  implement (TDD: failing test first), then flip `[ ]` → `[x]` and add a one-paragraph completion
  note.
- **No real data** anywhere (samples, docs, tests): synthetic placeholders only
  (`gitlab.example.com`, `group/subgroup/project`, `alice`).
- The main `roadmap.md` Phase 24 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Phase                                                    | Tasks       | Status      |
|----------------------------------------------------------|-------------|-------------|
| P1 — GitLab-first: `port.Forge` + config + resolution + native REST/GraphQL + links | T154–T160 | Complete |
| P2 — migrate GitHub + Azure onto `port.Forge`            | T161, T162  | Complete    |
| P3 — publishing via `release.targets` (config unification, not transport) | T163 | Complete |
| P4 (last) — `heraut init` wizard                         | T164        | Not started |

Phases run in order. P2–P4 tasks are stubs to be fleshed out when reached; **P1 is the phase to
plan first** (via `superpowers:writing-plans` → subagent-driven execution).

---

## Phase 1 — GitLab-first, end-to-end zero-config

Goal: ship the pain relief and the extensible seam together. `port.Forge` + the `forges:` /
`release.targets:` / `commits.enrichment_*` config + environment resolution + a native `net/http`
GitLab forge (REST default, GraphQL opt-in) + links + the breaking-config migration error. GitHub
and Azure are temporarily adapted to *feed* the resolver; their transports are unchanged (migrated
in P2).

**Execution split (Plan A / Plan B).** P1 is executed as two implementation plans. *Plan A —
foundation (T154–T157)* is **additive and green**: new keys are added **alongside** the old ones
(no removals), so `commits.enrichment_policy` is added next to `commits.remote_metadata` and nothing
breaks. *Plan B — GitLab forge + cutover (T158–T160)* adds the native GitLab forge, wires the
pipeline, and performs the **breaking rename/removal** (`remote_metadata` → `enrichment_policy`, drop
`changelog.remote` / `release.platforms`) **plus the migration error** — all in **T160**. So the
rename and migration error land in T160, not in T155/T156.

#### `[x]` T154: ADR-0043 + `port.Forge` contract

Write **ADR-0043** (forge abstraction + config unification), distilled from the design spec — the
permanent decision record that anchors the epic. Add the `port.Forge` interface (`Type`,
`Identity`, `CommitURL` / `ChangeURL` / `CompareURL`, `Enrich`), `ForgeIdentity`
(`{Type, Host, APIURL, Project, Token, TokenKind, APIMode}`), and `TokenKind` (`Job` | `Private`).
Move the enrichment result types (`Enrichment` = `{prs, authors}`, `PullRequest`, `Author`) into
`port` (or a shared leaf) so the native generator consumes them through the port. No behavior yet —
contract + record. Tests: interface compiles; type round-trips.

Landed as planned: `internal/port/forge.go` adds `TokenKind` (`TokenNone|TokenJob|TokenPrivate`),
`ForgeIdentity`, `Author`, `PullRequest`, `Enrichment`, `Commit`, and the `Forge` interface, with
`TestForge_InterfaceComposes` proving a fake implementation satisfies it and the value types
compose end to end. `docs/adr/0043-forge-abstraction.md` records the decision, mirroring
ADR-0042's section layout plus `Alternatives considered` and `References`. No consumer wired yet —
by design, this task is contract-only; T155–T160 build config, validation, resolution, and the
GitLab forge on top of it. Commit `0512b43`.

#### `[x]` T155: config — `forges:` + `release.targets:` + `commits.enrichment_*`

Add the config structs: `Forge` (`name`, `platform`, `project`/`repository`, `base_url`, `api_url`,
`api_mode`, `token_env`), `release.targets[]` (`forge`, `draft`, `prerelease`, `assets`); add
`commits.enrichment_forge` and **add** `commits.enrichment_policy` **alongside**
`commits.remote_metadata` (Plan A is additive; the rename/removal is deferred to T160). Strict
loader (unknown keys → error with line numbers). Sync **`schema.json`** and
**`docs/heraut.sample.yml`** in lockstep (coding rules). Tests: strict-parse fixtures per `platform`
and `api_mode`; schema validates each valid fixture.

**Completion note (2026-07-24):** Landed additively (Plan A). Added `Forge`/`Target` structs,
`Config.Forges`, `Release.Targets`, and `Commits.EnrichmentForge`/`EnrichmentPolicy` in
`internal/config/{config.go,commits.go}`, left the old `changelog.remote` / `release.platforms` /
`commits.remote_metadata` intact, and synced `schema.json` + `docs/heraut.sample.yml`. The new
fixture is validated against the real JSON-Schema validator (`testdata/config/valid/forge-minimal.yml`
via `TestSchema_ValidFixtures`). Review clean — two cosmetic minors: a co-author-trailer bracket
typo (fixed by amend) and `Config.Forges` field ordering. Commit `8e992a1`.

#### `[x]` T156: config validation (new keys)

Static semantic validation (`internal/config/validator.go`) of the new keys: forge `name`
non-empty + unique, `platform` ∈ `{github, gitlab, azure_devops}`, `api_mode` ∈ `{"", rest,
graphql}`, `commits.enrichment_policy` ∈ `{"", disabled, optional, required}`; and
`commits.enrichment_forge` / `release.targets[].forge` must name a known forge (required with >1
forge, optional with one). Two related checks live elsewhere per the Plan A/B split: the
`api_mode: graphql` requires-a-token check is **resolution-time** (T157), and the **migration
error** for the removed old keys is the **T160 cutover** (Plan A is additive — the old keys still
exist). Tests: table-driven validation.

**Completion note (2026-07-24):** Landed the static, additive half of this task's scope:
`validateForges` in `internal/config/validator.go` enforces forge `name` non-empty + unique,
`platform` ∈ `{github, gitlab, azure_devops}`, `api_mode` ∈ `{"", rest, graphql}`,
`commits.enrichment_policy` ∈ `{"", disabled, optional, required}`, and that
`commits.enrichment_forge` / `release.targets[].forge` name a known forge and are required only
when more than one forge is configured. Deliberately deferred, per the Plan A/B split this doc's
own phase intro describes (line ~61: "the rename and migration error land in T160, not in
T155/T156"): the `api_mode: graphql` + job-token check (resolution-time, not static config — goes
to T157) and the migration error for removed `changelog.remote` / `release.platforms` /
`commits.remote_metadata` (T160 cutover, since those keys are not yet removed). The heading and
description above have since been reconciled to match this split. Tests:
`internal/config/validator_forge_test.go`
(`TestValidate_Forges`, table-driven, 6 cases). Commit `6f4be94`.

#### `[x]` T157: forge identity resolution (config / CI / git / ambiguity)

Resolve `{host, apiURL, project, token, tokenKind}` per forge with precedence **explicit config →
CI env → git `origin` → none**. CI detection: GitLab (`GITLAB_CI` → `CI_SERVER_URL` / `CI_API_V4_URL`
/ `CI_PROJECT_PATH` / `CI_JOB_TOKEN`→Job), GitHub (`GITHUB_ACTIONS` → `GITHUB_SERVER_URL` /
`GITHUB_API_URL` / `GITHUB_REPOSITORY` / `GITHUB_TOKEN`→Private), Azure (`TF_BUILD` →
`SYSTEM_COLLECTIONURI` / `SYSTEM_TEAMPROJECT` / `SYSTEM_ACCESSTOKEN`). Zero-config resolves exactly
one forge; **fail loud** on ambiguity (nothing pins a single type + multiple forge tokens). git
`origin` parse yields host + project for known public hosts. Tests: table-driven with `t.Setenv`;
CI-env, origin-parse, precedence, and ambiguity-error cases.

**Completion note (2026-07-24):** New package `internal/forge` (`detect.go` + `resolve.go`),
importing only `internal/{port,config}` + stdlib — all env access via an injected
`getenv func(string) string`, no direct `os.Getenv`. `detectCIForge` reads the GitLab/GitHub/Azure
markers and vars per the design spec §3 table; `parseGitOrigin` handles both `git@host:path.git`
and `https://host/path.git`, mapping `github.com`/`gitlab.com`/`dev.azure.com` to their types.
`Resolve` dispatches to `resolveExplicit` (one identity per `cfg.Forges` entry, filling
host/apiURL/project/token per-field from CI — when its type matches the entry's `platform` — then
git origin, then a type default; `token_env` always wins and is stamped `TokenPrivate`) or
`resolveAuto` (CI → git origin → a single unambiguous token-implied candidate → offline;
`ErrAmbiguousForge` when more than one candidate token is present and nothing pins a type).
Tests: `internal/forge/resolve_test.go`, the five brief-specified cases verbatim (GitLab-CI
zero-config, git-origin local GitLab, ambiguous zero-config, explicit-forge-fills-from-CI,
no-forge-offline) — all pass; `go test ./...` (1455 tests) and `hk check` clean throughout.
**Deferred, not implemented here:** the `api_mode: graphql` + job-token-only validation error
that T156's note and the design spec's §"Error handling" assign to this task/resolution-time —
the task-4 brief this session executed against defines T157's scope as exactly the five tests
(CI/origin/config resolution + ambiguity) and does not include that check or a test for it; adding
it would have been undocumented scope expansion. The roadmap still needs reconciling on exactly
where that check lands (T157 resolution-time vs. T159's "the validation error from T156" wording,
which itself conflicts with T156's completion note deferring it away). Flagged for the next
task/roadmap-reconciliation pass rather than guessed at.

#### `[x]` T158: GitLab REST forge (native `net/http`)

`internal/forge/gitlab` implementing `port.Forge` over native `net/http` (no `glab`). REST default:
per-commit `GET /projects/:id/repository/commits/:sha/merge_requests` for MR refs
(number/title/labels/author/merger `username`); commit-author `by @` = the **local git author name**
(no API call — as Azure). Auth header follows `TokenKind`: **Job → `JOB-TOKEN`**, **Private →
`PRIVATE-TOKEN`**. Links (`CommitURL` / `ChangeURL` `!N` / `CompareURL`). Tests: `httptest.Server`
request-shape + header-selection + mapping + `by @git-name` render.

Implemented as planned: `internal/forge/gitlab/rest.go` + `gitlab.go` cover the REST default path,
with `httptest.Server` contract tests asserting the request shape, `JOB-TOKEN`/`PRIVATE-TOKEN`
header selection by `TokenKind`, MR-to-render-model mapping, and the local-git-name `by @` fallback
when no linked handle is available.

#### `[x]` T159: GitLab GraphQL forge mode (opt-in)

`api_mode: graphql` path: reuse the batched query logic (ADR-0042 — `commits(ref:){author{username}}`
+ `mergeRequests`) POSTed via native `net/http` with `PRIVATE-TOKEN`, rendering the **linked
`@username`** commit-author handle. **Guard (owned by this task):** graphql + only a job token → a
validation error with a hint (GraphQL rejects job tokens; use `api_mode: rest` or supply a `read_api`
token), no network call. This check lives here, not in T156/T157: it is only reachable once the
GraphQL path exists and needs the resolved `TokenKind` (T157's `port.ForgeIdentity`) — T156 (static
config validation) and T157 (identity resolution) both deliberately left it out. Tests:
`httptest.Server` GraphQL path; `PRIVATE-TOKEN` header; linked-handle render; job-token → error.

Implemented as planned: `internal/forge/gitlab/graphql.go` reuses the ADR-0042 batched query,
POSTed via `net/http` with `PRIVATE-TOKEN`; the job-token + graphql guard rejects before any network
call, with a hint pointing at `api_mode: rest` or a `read_api` token. Contract tests cover the
GraphQL request shape, the header, the linked-`@username` render, and the guard's error path.

#### `[x]` T160: pipeline wiring + old-path removal

Wire the forge end-to-end: changelog + release-notes enrichment resolves the `port.Forge` from
`commits.enrichment_forge` and applies `commits.enrichment_policy` (unchanged degrade/required
semantics; `--offline` forces disabled); the native generator consumes `port.Forge`. Publishing
reads `release.targets` (default: the single resolved forge with default options). GitHub and Azure
are adapted to *feed* the resolver (transports unchanged). Perform the breaking config migration:
rename `commits.remote_metadata` → `commits.enrichment_policy`, remove the now-dead
`changelog.remote` config path, and emit a **clear migration error** when any removed key is
present (map old → new with a before/after hint; no silent alias).

**Scope decision (2026-07-24) — enrichment-first.** This task originally also removed
`release.platforms`. That key is consumed by **17 non-test files including `internal/scaffold/`**
(the `heraut init` wizard), whose rewrite is deliberately **P4, last**. Removing it here would drag
P4 forward, so: T160 wires the forge for **enrichment** (changelog + release notes) and derives the
render `LinkContext` from the resolved forge, and **`release.platforms` stays** for publishing until
**P3**, which already owns "fold publishing into `port.Forge`". `release.targets` remains
parsed-and-validated but unused until then — expected, not dead code. Tests:
integration — zero-config GitLab CI changelog (enriched via `CI_JOB_TOKEN`); happy-path + dry-run
release to a resolved forge.

Landed in two slices: **T160a** (pipeline wiring, prior session) resolved the enrichment forge in
`internal/app/pipeline.go` and injected it into the native generator, deriving `LinkContext` from
the resolved identity ahead of the ambient/single-platform fallback. **T160b** (this session) did the
breaking config cutover: deleted `config.Remote` and `ContentDriver.Remote` (`changelog.remote` is
gone entirely — no per-driver replacement; `forges:` is the top-level equivalent), deleted
`Commits.RemoteMetadata` in favor of the already-existing `Commits.EnrichmentPolicy`, renamed the
`*Config` accessor `RemoteMetadata()` → `EnrichmentPolicy()`, and removed `ChangelogRemote` /
`remoteLinkContext` / `remoteBaseURL` / `tokenEnvOrDefault` from `internal/pipeline`. Added
`config.ErrRemovedConfigKey` plus `checkRemovedKeys`, run against the raw YAML bytes in both
`Load` and `LoadFromReader` ahead of the strict decode, so a removed key gets a mapped,
actionable error instead of a generic "unknown key". `internal/scaffold/` needed a mechanical
two-line rename (`Commits.RemoteMetadata` → `Commits.EnrichmentPolicy`, `cfg.RemoteMetadata()` →
`cfg.EnrichmentPolicy()`) to keep compiling — the wizard's `Answers.RemoteMetadata` field and its
`forges:`/`release.targets:` UI are untouched, deferred to **P4/T164** as planned.
`release.platforms` publishing and its removal remain deferred to **P3** ("fold publishing into
`port.Forge`"), per the 2026-07-24 scope decision above — this task only removed the enrichment-side
`changelog.remote` / `commits.remote_metadata` keys, not the publish-side `release.platforms`.
Migrated (not deleted) the fixtures/tests covering a still-supported feature: `commits.remote_metadata`
→ `commits.enrichment_policy` in `testdata/config/valid/enrichment-policy.yml` (renamed from
`remote-metadata.yml`) and in `validator_test.go`'s enum tests. Deleted the fixtures/tests whose
feature is gone with no replacement shape: `changelog-remote.yml`, `changelog-remote-native.yml`,
`invalid_remote_type.yml`, `remote_api_url_removed.yml`, the `validateContentDriverRemote` test
block, and `TestRemoteLinkContext` — `forges:` is a different (top-level, not per-driver) shape, so
there is no like-for-like migration for those cases. `go test ./...` (1469 tests) and `hk check` are
clean.

### Plan B / P2 handoff notes (from Plan A's final whole-branch review)

Carried forward so they are not rediscovered once a consumer wires `port.Forge` / `forge.Resolve`
(all three are **inert in Plan A** — no consumer yet):

- **`port` vs `native` PR/Author field delta.** `port.PullRequest`/`port.Author` are near-mirrors of
  `internal/generators/native`'s types, **not** identical: native's `PullRequest` also carries
  `Platforms map[string]any` and native's `Author` carries `Name`/`Email` (not just `Username`). When
  P2 unifies native onto the `port` types, the boundary converter **must preserve** `Platforms` and
  `Name`/`Email`, or that data is silently lost.
- **Azure zero-config `APIURL`.** `detectCIForge` returns `APIURL: ""` for Azure (there is no
  `CI_API_V4_URL`-equivalent); the Azure API URL must be derived (from `SYSTEM_COLLECTIONURI` / host)
  when the Azure forge driver needs it.
- **git-origin gate is type-match, not host-match.** `resolveExplicit` fills git-origin fields when
  `originType == forge.Type` (behaviorally equivalent to host-match today, since `parseGitOrigin`
  returns `ok=false` for non-public hosts). Revisit if a self-hosted explicit forge must draw its
  project/host from `origin`.

---

## Phase 2 — migrate GitHub + Azure onto `port.Forge`

Bring the other two platforms under the interface so enrichment is uniform and the `enrich()` switch
retires.

#### `[x]` T161: GitHub forge onto `port.Forge`

Migrate GitHub enrichment (`gh api graphql`, `GITHUB_TOKEN`) to a `internal/forge/github`
implementing `port.Forge`. Transport may stay `gh api` (its token works everywhere) or move to
native http — decided at plan time. Tests: contract parity with today's GitHub enrichment.

Implemented `internal/forge/github` over stdlib `net/http` (not `gh api`), matching the transport
decision already made for GitLab in P1 — `heraut changelog` now needs no `gh` CLI on PATH for
GitHub enrichment. The GraphQL query (`prFragment`, aliased `s0…sN` batching, 50-SHA chunking) and
response-mapping logic were ported verbatim from `internal/generators/native/enrich_github.go`,
changing only the transport (POST to `{apiBase}/graphql` with a bearer token) and the result types
(`port.PullRequest`/`port.Author` instead of the native package's own structs). Added `Repository`
to `port.ForgeIdentity` for forges (Azure DevOps) that separate the repo name from the project
path; GitHub/GitLab continue to carry the full path in `Project` and leave it empty.
`internal/forge/resolve.go` populates it for `azure_devops` only, with a dedicated resolve_test.go
row. Wired into `internal/app/pipeline.go`'s `resolveEnrichForgeIfNeeded` type switch alongside
`gitlab`, guarding the typed-nil hazard by assigning to the `port.Forge` interface variable only
inside each case. Updated the stale `forge_internal_test.go` subtest that previously asserted "no
forge for github" to assert one is constructed with `Type() == "github"`. `gh` is still used for
publishing (`internal/platforms/github`) — untouched, out of scope for this task.

#### `[x]` T162: Azure forge onto `port.Forge` + retire the switch

Wrap the existing Azure `net/http` enrichment (ADR-0035) as `internal/forge/azure` implementing
`port.Forge`; remove the per-platform `enrich()` dispatch switch in favor of the interface. Tests:
Azure contract parity; switch removal doesn't regress GitHub/GitLab.

Implemented `internal/forge/azure` (`azure.go` + `prquery.go`) over stdlib `net/http`, porting the
`pullrequestquery` POST, result types, `authorLogin`, and `commitAuthors` verbatim from
`internal/generators/native/enrich_azure.go` — only the org/project/repo source changed (now
`id.Project` split + `id.Repository`, instead of `LinkContext.Owner`/`Repo`) and the result types
(native's own structs → `port.PullRequest`/`port.Author`). Preserved api-version `7.1`, HTTP Basic
auth with an empty username, `RefPrefix: "!"`, `vote >= 10` approvers, and the local
email-local-part commit-author render (Azure exposes no linked handle). Completed Azure CI
identity resolution in `internal/forge/detect.go`: `SYSTEM_COLLECTIONURI` now supplies the
organization (parsed from its first path segment) so `Project` becomes `"{org}/{teamProject}"`,
and `BUILD_REPOSITORY_NAME` supplies `Repository`, threaded through `resolveAuto`'s CI branch.
Wired `azureforge.New` into `internal/app/pipeline.go`'s `resolveEnrichForgeIfNeeded` switch
alongside gitlab/github, and restored Azure in `internal/pipeline/linkctx.go`'s
`linkContextFromIdentity`: `Owner` maps to the full unsplit `Project` (organization/project) and
`Repo` to `Repository`, with an empty-`Repository` guard falling through to nil (mirroring the
existing empty-`Project` guard) — this replaces the outright `azure_devops` exclusion that existed
only because `ForgeIdentity` had no `Repository` field before T161. Retired the legacy `enrich()`
per-platform switch in `internal/generators/native/enrich.go`, deleting `enrich_github.go`,
`enrich_gitlab.go`, `enrich_azure.go` and their test files along with now-dead helpers
(`gqlString`, `oldestCommitDate`, `newestSHA`, `enrichable`) and the now-unused `httpClient` field
on `Generator` (Azure enrichment no longer lives in `native`). `enrichForRelease`'s required
condition simplified from `required && g.forge == nil && !enrichable(lc)` to `required && g.forge
== nil`, since a forge is now the only enrichment source. The policy-behaviour tests in
`enrich_internal_test.go` (disabled/optional-degrade/required-fatal/`--force`-downgrade,
changelog-scoped-to-new-release) were rewritten against a local `countingForge` stub instead of
`gh`-via-MockRunner responses — same assertions, transport swapped, per the brief's "preserve
behaviour, not code" instruction — and two `generator_internal_test.go` cases
(`TestGenerateChangelog_ForeignFileErrors_BeforeEnrichment`,
`TestGenerateChangelog_RegenerateEnrichesAllSections`) were adjusted the same way. `TestGqlString_QuotesAndEscapes`
was deleted outright (it tested only the deleted `gqlString` helper, not policy). `go test ./...`
and the simulated `GITHUB_ACTIONS=true` run both pass at 1470 tests; `internal/generators/native`
now imports no forge package.

**Post-hoc correction (final review, C1/C2/I1/I2/I3/I5/M2):** the org-in-`Project` design
described above for Azure CI detection was wrong for two reasons a final review caught with
`httptest`-pinned request-path assertions (the original migration had dropped the old exact-shape
assertions, which is exactly how both shipped). (1) GitHub Enterprise Server's GraphQL endpoint is
`{host}/api/graphql`, not `{host}/api/v3/graphql` — fixed with a `graphqlEndpoint()` method
mirroring GitLab's `apiBase()`-suffix-trim pattern. (2) Azure `detectCIForge` concatenated the org
onto `SYSTEM_TEAMPROJECT` AND left the org inside `SYSTEM_COLLECTIONURI`-derived `Host`, so
`Host+"/"+Project` composed the organization twice in both the API endpoint and every rendered
link; it also silently mishandled the legacy `{org}.visualstudio.com` subdomain form (org lives in
the host, not a path segment). Fixed by making `Host` the trimmed `SYSTEM_COLLECTIONURI` verbatim
(org included only when it is genuinely a path segment) and `Project` the team project alone for
CI-detected identities; `internal/forge/azure/prquery.go` no longer splits `Project` into
org/project (dropped `splitProject`), composing the endpoint the same way `webBase()` always did.
`azureOrgFromCollectionURI` became dead code under the new design and was deleted along with its
test. Also: `Repository` never fell back to the CI-detected value the way `Host`/`APIURL`/`Project`
did (I1) — fixed in `resolve.go`; `port.LinkContext.APIEnv()` was orphaned production code with no
remaining caller (I5) — deleted with its ~90 lines of tests. See
`.superpowers/sdd/p2-final-fix-report.md` for the full RED/GREEN evidence per finding.

---

## Phase 3 — publishing via `release.targets` (config unification, not transport)

#### `[x]` T163: `release.targets` replaces `release.platforms`

**Scope decision (2026-07-27, recorded in ADR-0044):** P3 shipped as a **config unification**,
not the transport fold this section originally described. `release.targets[]` (added in T155)
becomes the publishing surface; `release.platforms` is removed with a migration error
(`ErrRemovedConfigKey`) naming the `forges:` + `release.targets[].forge` replacement.
`internal/platforms/{github,gitlab}` and `port.Platform` are **unchanged** — publishing still
shells out to `gh`/`glab`; only how those drivers are configured and constructed changed (from a
standalone `config.Platform` block to `platformConfigFromTarget`, fed by the same resolved
`port.ForgeIdentity` enrichment already uses). See ADR-0044 for the three reasons the transport
stayed put (no config-goal dependency on it, the original `CI_JOB_TOKEN` pain was already solved
in P1, and P2's two shipped defects demonstrate the risk of hand-rolled request shapes on the
artifact path). Native publishing (which would actually drop the `gh`/`glab` dependency) remains
a future, separately-motivated task — nothing here forecloses it, since the drivers already sit
behind `port.Platform`.

**What changed:** `config.Release.Platforms` / `config.EnvRelease.Platforms` / `config.EffectivePlatforms`
deleted; `hasEffectivePlatforms` in `internal/cmd/release.go` replaced by
`app.HasResolvablePublishTarget` (has ≥1 effective `release.targets` entry, or a forge that
auto-resolves); `internal/app/check.go`'s `RuntimeCheck` Platforms section now resolves
`EffectiveTargets` + `forges:` (via the same `forge.Resolve` enrichment uses) instead of reading
`release.platforms` directly, so `heraut check` shows the auto-detected identity per target.
`buildTargetPlatforms`'s `hasEffectivePlatforms` guard parameter — which existed only to stop a
`release.platforms` config from also gaining a silent zero-config-synthesized target — was
removed outright along with the field it guarded; publishing's forge resolution is now
unconditional (previously conditional on nothing else needing it). `internal/scaffold` changed
mechanically only, per the brief's boundary: `wizard.go`'s `PlatformAnswer` type, prompts, and
flow are untouched; `generate.go` now emits a `forges:` entry + `release.targets` entry per
platform answer (defaulting `commits.enrichment_forge` to the first forge when more than one is
configured, since the wizard has no forge-selection question yet); `ConfigToAnswers`/`dropped.go`
read back through `cfg.Forges` joined with `cfg.Release.Targets` by name. The wizard's redesign
(forge-aware prompts, an `api_mode` question, auto-detection defaults) stays T164/P4, untouched
here.

**Deferred, out of this task's file list:** `docs/specs/03-commands.md`,
`docs/specs/05-generators-and-platforms.md`, `docs/specs/01-overview.md`, and this repo's
`CLAUDE.md` still reference `release.platforms` in prose (e.g. the "requires ≥1 entry in
release.platforms" constraint). Only `docs/specs/02-configuration.md` was in this task's file
list and is fully migrated; the remaining spec-doc reconciliation is left as a follow-up (not
silently expanded here).

**Tests:** `internal/config/migration_test.go` gained `TestLoad_RemovedKey_ReleasePlatforms`
(top-level + per-env). Validator/loader/schema tests carrying `release.platforms` as filler were
migrated to `forges:`/`release.targets:`; tests asserting deleted validation logic
(`validatePlatformEntries`, name/base_url checks scoped to `release.platforms`) were deleted
outright — that behavior is superseded by `validator_forge_test.go`'s existing `forges:`
coverage, not silently dropped. `internal/app/check_test.go` and `internal/app/pipeline_test.go`
needed a `clearCIEnv` helper (mirroring the one already `internal/app`-internal) because
`RuntimeCheck`'s Platforms section and `BuildPipeline` now call `forge.Resolve` — and therefore
`os.Getenv` — for every non-nil config, so tests that don't explicitly isolate CI env can flip
outcomes under real CI. Two `internal/cmd` integration tests
(`TestCheckAll_PassesAll`, `TestRelease_NoPlatforms_Error`) failed only under the simulated-CI
run (`GITHUB_ACTIONS=true …`) during this task's own verification, for the same reason, and
needed the same isolation — confirming the project's existing CI-leakage guard against this
class of bug is warranted. `go test ./...` and the simulated-CI run both pass; `git diff --stat`
against `internal/platforms/` and `internal/port/platform.go` is empty, confirmed before
committing.

---

## Phase 4 (last) — `heraut init` wizard

Deliberately last: the wizard codifies the config shape, so it lands only after the new config is
**battle-tested in real pipelines** (P1–P3), to avoid churning it against a moving schema.

#### `[ ]` T164: `heraut init` generates the forge config

Update the scaffold wizard (`internal/scaffold`) to generate `forges:` / `release.targets:` /
`commits.enrichment_forge` / `commits.enrichment_policy`, with auto-detection defaults and an
`api_mode` prompt. Tests: wizard-output fixtures validate against `schema.json`.

---

## Follow-ups

#### `[x]` T175: `heraut check` and `heraut changelog` disagree about the same config

T172 made `heraut check` warn (rather than fail) when forge resolution fails and **publishing** is
unconfigured. But resolution is also consumed by **enrichment**: `resolveEnrichForgeIfNeeded`
(`internal/app/pipeline.go`) calls `resolveForge` whenever a driver is `generator: native` and the
policy is not `disabled`, and propagates the error. So for `changelog: {generator: native}` with no
`forges:` and no `release.targets`, on an ambiguous machine, `heraut check` prints an advisory and
exits 0 while `heraut changelog` hard-fails on the identical error — and the check no longer predicts
that failure, for exactly the changelog-only user T172 set out to protect. Two fixes, in opposite
directions: widen `wantsForge` to include enrichment consumers, or fix the deeper asymmetry —
`optional` promises "on failure, degrade" (`internal/generators/native/enrich.go`), yet a
*resolution* error under `optional` is fatal today. The second is the better behaviour and would make
T172's warning correct by construction. Found by the hardening phase's final review (2026-08-02).
**Scope:** S–M.

Implemented the second (recommended) fix, scoped to the changelog-only pipeline where the
divergence actually manifests: `resolveEnrichForgeIfNeeded` (`internal/app/pipeline.go`) gained a
`force bool` parameter and a third `string` return value (a degraded reason), computed the same way
`enrichForRelease`'s `required := ... && !g.cfg.Force` already does. A `resolveForge` failure is
fatal only when `enrichment_policy: required` and not downgraded by `--force`; under the
default/optional policy (or `required` + `--force`) it now returns a nil forge/identity plus a
non-empty degraded reason instead of an error — the same "on failure, degrade" contract
`enrichForRelease` already promises for post-resolution fetch failures, just applied one step
earlier, at resolution. `buildChangelogPipelineConfig` threads that reason into a new
`native.WithDegraded(reason)` constructor option (`internal/generators/native/generator.go`),
seeding `g.degraded`/`g.degradedReason` before generation runs, so the existing
`internal/pipeline/warn.go` sub-result rendering picks it up unchanged — no new UI plumbing needed.
`buildReleasePipelineConfig` was deliberately **not** touched: it resolves the forge unconditionally
for publishing (not just enrichment) via its own inlined `needsForge`/`resolveForge` call, and
`heraut release`'s pre-flight (`HasResolvablePublishTarget`) already gates that command before this
code is reached when there is no resolvable publish target — the divergence T175 describes is
specific to `heraut changelog`, which has no equivalent pre-flight and no publish-side reason to
ever fail on enrichment alone. `buildGenerator` gained a matching `degradedReason string` parameter;
its two `buildReleasePipelineConfig` call sites pass `""` (unaffected by this fix). Tests (TDD,
`internal/app/forge_internal_test.go`): replaced the "ambiguous forge propagates as an error" case
with three — default policy degrades (asserts a non-empty reason, nil forge/identity, no error),
`required` policy still errors, and `required` + `--force` degrades like a fetch failure would; all
other `resolveEnrichForgeIfNeeded` call sites updated for the new signature. Added
`TestWithDegraded_SeedsDegradedState` in `internal/generators/native` for the new option, and
`TestBuildChangelogPipelineConfig_AmbiguousForgeDegradesUnderOptionalPolicy` as an end-to-end proof
that `buildChangelogPipelineConfig` no longer errors for this exact config and the built generator
reports `Degraded() == true`. `go test ./...` (both plain and simulated `GITHUB_ACTIONS=true`) and
`hk check` are clean.

#### `[ ]` T176: T171 rejects duplicate forge *names*, not duplicate *destinations*

`resolvedForgeName` (`internal/config/validator.go`) compares forge **names**, so two distinctly-named
`forges:` entries that resolve to the same place still pass — e.g. two `platform: github` entries with
no explicit `repository`, both filled from the same CI env or git origin, targeted separately. The
second `release create` still fails after the tag is pushed, which is the hazard T171 exists to
prevent. Exact coordinate comparison is impossible in `internal/config` (no runner, no env), but a
cheap config-level approximation catches precisely the both-empty case: reject two `forges:` entries
sharing an identical `(platform, base_url, project/repository)` tuple. Either implement it or record
the residual gap in T171's note so it doesn't read as fully closed. Found by the hardening phase's
final review. **Scope:** S.

#### `[ ]` T174: `enrichment_policy: required` is not enforced for the git-cliff generator

`required` is a guarantee — fail rather than ship unenriched notes. It holds for `native`
(`internal/generators/native/enrich.go` errors when the policy is `required` and no forge is
resolvable), but **not** for git-cliff: `runCliff`'s `case remoteRequired: return g.exec(args, lc)`
only suppresses the `--offline` retry. With no forge resolved, `injectRemote` injects no
`[remote.*]` section, git-cliff runs and **exits 0 with unenriched output and no error**. Its own
doc comment says "a remote-fetch failure is fatal", which is true only when a fetch is actually
attempted — with nothing configured, none is. So a user who sets `required` specifically to prevent
shipping unenriched release notes gets exactly that, silently. Two generators diverging on a
user-facing policy guarantee is the defect; either make git-cliff assert a resolvable forge before
running, or document the divergence as intended. Found while fact-checking T169's docs (2026-08-02);
pre-existing, not introduced by the forge epic. Note git-cliff is itself slated for removal
(native-generator roadmap Phase 2.5), which may make documenting-and-deferring the right call.
**Scope:** S.

#### `[x]` T171: duplicate publish targets can resolve to the same destination

With no `forges:` block, `resolveTargetForge` returns the same auto-detected identity for every
target, so `release: targets: [{}, {draft: true}]` builds two drivers pointing at one repository.
The second `release create` then fails **mid-pipeline, after the tag has already been pushed** — the
worst point to fail. Cheap to reject in `validateForges`: more than one target with no `forge:` and
no `forges:` block is unsatisfiable. Found by P3's final review. **Scope:** S.

Implemented in `validateTargetForges` (`internal/config/validator.go`), not `validateForges` as
originally scoped — `validateTargetForges` is the helper that already walks each target list (top-level
and per-env) and was the natural place to add the check without duplicating logic at both call sites.
The rule is implemented as stated — no two targets may resolve to the same forge — rather than as an
enumeration of the shapes it takes: each target is **normalized to the forge it will actually resolve
to** (new `resolvedForgeName`, mirroring `internal/app`'s `resolveTargetForge` — explicit `forge: X`
→ `X`; bare with exactly one configured forge → `forges[0].Name`; bare with none → a shared
`autoDetectedForge` sentinel), and any duplicate in that normalized list is the error. A first pass
enumerated two cases — a duplicate explicit name, or more than one bare target when `forgeCount <= 1`
— and called them exhaustive; **they are not.** With exactly one configured forge, a bare target and
an explicit `forge: A` both resolve to A, and neither case sees it (`heraut check config` reported
`✓ config: ok` for that config). Normalizing subsumes all three shapes without special-casing and
cannot drift out of sync with `resolveTargetForge` the way an enumeration does. Targets already
rejected per-entry (unknown forge, or bare with more than one forge configured) are skipped in the
duplicate scan, so one mistake never draws two errors. One error is emitted at the list path (e.g.
`release.targets`, not `release.targets[1].forge`) so a duplicate set produces a single diagnostic.
`internal/app/pipeline.go` itself was not touched — the fix is a validation-time rejection, not a
change to `resolveTargetForge`'s runtime resolution.

#### `[x]` T172: `heraut check` hard-fails for changelog-only users in an ambiguous environment

`internal/app/check.go` resolves a forge unconditionally, so a user with no `forges:` block, both
`GITHUB_TOKEN` and `GITLAB_TOKEN` exported, and an origin `parseGitOrigin` doesn't recognise (any
self-hosted host) gets a failing `forge` row from `heraut check` — even if they never publish.
Previously that config produced binary-probe warnings only. `heraut check` is commonly a CI gate, so
a false failure is costly. Narrow the trigger to users who actually need a publish destination.
Found by P3's final review. **Scope:** S.

Implemented in the `resolveErr != nil` branch of `RuntimeCheck` (`internal/app/check.go`): the row's
`IsWarn` is now `!wantsForge`, where `wantsForge := len(cfg.Forges) > 0 ||
len(config.EffectiveTargets(cfg, env)) > 0` — the same "did the user ask for a forge" test the task
brief specified. `cfg` is guaranteed non-nil in this branch (`effectiveTargetPlatforms` short-circuits
to `(nil, nil)` before calling `resolveForge` when `cfg == nil`), so no extra nil-guard was needed. The
row's message is unchanged — it already carries the underlying resolution error (e.g. "detected
candidates [gitlab github] and no CI/origin to disambiguate") — so both the warn and error cases stay
equally informative; only the severity flag changes. The branch comment was rewritten to state the
warn/error distinction instead of unconditionally justifying a hard failure. One test gap surfaced
while writing the RED test: the task brief's suggested "explicit forges -> hard failure" fixture
(two `forges:` entries, `release.targets: [{forge: A}]`) does not actually exercise
`resolveErr != nil` — `forge.Resolve` takes the `resolveExplicit` path whenever `cfg.Forges` is
non-empty, and that path never returns an error (per-forge gaps are filled independently; there is no
ambiguity to detect). The test was adapted to a config that does reach the error path with explicit
intent: no `forges:` block, but a non-empty `release.targets`, which still triggers `resolveAuto`'s
ambiguity check while satisfying the "explicitly asked for a forge" half of the rule via `Targets`
rather than `Forges`.

**Both disjuncts of `wantsForge` are reachable and covered** (corrects an inaccurate claim in the
first version of this note, caught in review). `resolveExplicit` indeed never errors — but it is not
the only error source funneled into `resolveErr`: `effectiveTargetPlatforms` also calls
`resolveTargetForge` once per target, and *that* errors with a non-empty `forges:` block, either
`unknown forge %q` (a target naming a forge that isn't declared) or "forge is required when more than
one forge is configured" (a bare target with several forges). In production `config.Load` →
`validateTargetForges` normally rejects those configs first, but `check_test.go` builds `*config.Config`
from struct literals and so bypasses validation — which is how the tests reach it. Two subtests were
added: a target naming an unknown forge, and — the only shape that *isolates* the first disjunct —
two forges with an empty `release.targets` (so `EffectiveTargets` is empty and cannot carry the
result). Verified by mutation: deleting `len(cfg.Forges) > 0` from `wantsForge` flips that second row
to a warning and fails the test, while the unknown-forge fixture keeps passing (its target list is
non-empty, so the second disjunct still covers it).

#### `[ ]` T173: P3 cleanups — dead `needsForge` guard, double resolution, migration hint, test helpers

A cluster of small items from P3's final review, none behaviour-affecting on their own:
`needsForge` (`internal/app/pipeline.go`) is now a tautology — `(A && B) || len(t) > 0 || len(t) == 0`
is always true — so it reads as a safety guard while doing nothing; replace it with an unconditional
call plus a comment, or the next reader will assume `--offline` still skips resolution.
`HasResolvablePublishTarget` dereferences `cfg.Forges` without the nil-guard its sibling
`effectiveTargetPlatforms` has. Zero-config resolves the forge twice per release (once in
`internal/cmd`, once in `internal/app`), spawning two `git remote get-url origin` subprocesses where
the sharing was meant to be end-to-end. The migration hint for the removed `release.platforms` names
`base_url`/`token_env`/`repository`-or-`project` but omits the **required** `name` and `platform`, so
a user following it literally hits a second round of errors; the per-env variant should also say
`forges:` is top-level only. Finally, `clearCIEnv` is now triplicated across three test files
(`internal/testutil` is its natural home) and `config.Platform` still carries YAML tags despite its
doc comment saying it has no YAML surface. **Scope:** S–M.

**Migration-hint half done** (as part of T171's session): `releasePlatformsHint`
(`internal/config/loader.go`) now names `name` / `platform` as required alongside the optional
`base_url` / `token_env` / `repository`-or-`project` coordinates, and the per-env probe uses a new
`releasePlatformsHintPerEnv` (hint + "`forges:` is top-level only, there is no
`environments.<env>.forges`"). The remaining items in this cluster — `needsForge`,
`HasResolvablePublishTarget`'s nil-guard, double forge resolution, `clearCIEnv` triplication, and
`config.Platform`'s stray YAML tags — are still open.

#### `[ ]` T168: decide the fate of `port.Forge`'s link methods (and the dead `lc` parameter)

`port.Forge` declares `CommitURL` / `ChangeURL` / `CompareURL`, implemented and tested in all three
forges — but they have **no production caller**. Link rendering still runs through
`port.LinkContext` → `internal/generators/native/links.go`. (`webBase()` *is* used internally by the
Azure driver for PR URLs; it is the three interface methods that are dead.) That produced a real
coverage illusion during P2: the azure link tests exercised a path nothing calls, while the live
path went untested — which is part of why the org-duplication bug shipped. Decide: wire the
interface methods into rendering (collapsing two link-building implementations into one), or mark
them explicitly as P3 publishing surface. Fold in the related cleanup: `enrich`/`enrichForRelease`
still take an `lc *port.LinkContext` parameter neither reads — two production call sites, no lint
rule enforcing it, but it implies a coupling P2 deliberately deleted. Found by P2's final review.
**Scope:** S–M.

#### `[x]` T169: document the self-hosted / GHES enrichment requirement, and fix ADR drift

Two related doc gaps found by P2's final review. (1) **Capability regression, undocumented:** before
P2, a `generator: native` user on a self-hosted GitHub Enterprise / GitLab host enriched via the
legacy dispatch using the platform-derived `LinkContext`. Now `parseGitOrigin` only recognises the
public hosts (`github.com`, `gitlab.com`, `dev.azure.com`), so a self-hosted origin resolves no
forge — enrichment degrades under `optional` and hard-errors under `required`. The fix for those
users is to declare an explicit `forges:` entry, but nothing tells them so. Document it in
`docs/specs/05-generators-and-platforms.md` and consider naming it in the required-policy error.
(2) **ADR drift:** ADR-0034 is still `Accepted` while describing `gh api` as GitHub's enrichment
transport (deleted in T162), ADR-0043 says "GitHub and Azure keep their current transports", and
spec 05 still describes GitLab enrichment as "two batched `glab api graphql` connection queries".
Since ADRs outrank the roadmap in this repo's source-of-truth hierarchy, leaving them stale inverts
that hierarchy. **Scope:** S.

Added a new "Auto-detection and self-hosted hosts" subsection to
`docs/specs/05-generators-and-platforms.md`, placed directly after the `forges` fallback-chain
paragraph in the `##### forges — explicit metadata forge` section: which two sources fill gaps
(CI markers, then `git remote get-url origin`), that origin detection recognises only
`github.com`/`gitlab.com`/`dev.azure.com`, and the `commits.enrichment_policy` behaviour when no
forge resolves — stated **per generator**, since the two enforce the policy independently and do
**not** agree. `native` (`enrichForRelease`, `internal/generators/native/enrich.go`): `optional`
proceeds silently and does **not** set `Degraded()` (that flag only fires when a *configured*
forge's fetch fails — `enrich` returns a nil error via its `g.forge == nil` early return, so the
`err != nil` branch that sets the flag is never reached); `required` fails outright naming the
three remedies, with `--force` downgrading it. `git-cliff` (`runCliff` / `injectRemote` /
`linkEnv`, `internal/generators/gitcliff/generator.go`): with no forge resolved there is no
owner/repo, so no `[remote.*]` is injected and no `GITHUB_REPO`/`GITLAB_REPO` is set — git-cliff
fetches nothing and exits 0, meaning **both** `optional` and `required` yield unenriched output
with no error and no degraded flag (`required` only suppresses the `--offline` retry; it asserts
nothing about a forge existing). Confirmed no upstream enforcement compensates: the only
`required` handling in the tree is inside those two generators. Also added the explicit `forges:`
remedy with synthetic placeholders (`gitlab.example.com`, `group/subgroup/project`). Did **not**
name the gap in the required-policy error string itself — that's a code change, out of scope for a
docs-only task. Added dated `> **Update (2026-08-02):**` blockquotes (matching ADR-0039's style)
to both ADR-0034 and ADR-0043, recording that GitHub's enrichment migrated onto native `net/http`
(`internal/forge/github`) in ADR-0043's P2 phase, alongside GitLab (P1) and Azure (ADR-0035) — no
enrichment path shells out to `gh api`/`glab api` anymore — while publishing still does
(`gh`/`glab`, ADR-0044, unchanged). Annotated both ADRs' `docs/adr/README.md` rows to match.
Also corrected the full-regeneration paragraph's stale "two batched `glab api graphql` connection
queries" (GitLab enrichment is native `net/http`, `internal/forge/gitlab`), and — found while
verifying it — its "no platform pays a per-commit cost" claim, which is false for the **default**
`api_mode: rest`: `enrichREST` (`internal/forge/gitlab/rest.go`) issues one
`GET /projects/:id/repository/commits/:sha/merge_requests` per commit, so a full regeneration
there is O(commits), not O(releases). The batched two-connection-query description remains
accurate but only for the opt-in `api_mode: graphql` (`enrichGraphQL`,
`internal/forge/gitlab/graphql.go`), and is now scoped that way. GitHub's "50 SHAs/query"
(`ghChunkSize`) and Azure's single `pullrequestquery` POST were re-verified and left as-is.
Did not touch any Go code or test; `git diff --stat` is docs-only.

#### `[x]` T170: cross-forge consistency (author fallback, Azure `api_url` / `api_mode`)

Two divergences between the three forges, found by P2's final review. (1) **Author-handle fallback
differs:** when no linked handle is available, GitLab renders the git author **name** (`by @Alice
Smith` — a space inside an `@handle`) while Azure renders the email **local-part** (`by @alice`).
Both are the same "no linked identity" fallback and should render identically; the local-part reads
better as a handle. (2) **Azure ignores `api_url` and `api_mode`:** `config.Forge` accepts both for
any platform and the validator only enum-checks `api_mode`, but Azure honours neither (it derives
the API root from `Host`). At minimum document the per-platform applicability on the struct fields
and in spec 02; ideally have Azure prefer `APIURL` when set. Consider a cross-forge conformance
table test (empty commits → non-nil empty maps; non-2xx → wrapped error naming the status; partial
identity → clear error) to keep three parallel implementations honest as P3 adds publishing.
**Scope:** S–M.

Closed the author-fallback half only (1); the Azure `api_url`/`api_mode` half (2) remains open.
`gitAuthors` in `internal/forge/gitlab/gitlab.go` now prefers the git author email's local-part,
falling back to the git author name, matching `authorLogin` in `internal/forge/azure/azure.go`
(ADR-0043 / T151) — an `@handle` should not contain spaces. This is a deliberate, user-visible
changelog output change for GitLab in `api_mode: rest` (the default): `by @Alice Smith` now
renders `by @alice` when an email is present. `api_mode: graphql` is unaffected — it resolves a
real linked `@username` and never reaches this fallback. Updated
`docs/specs/05-generators-and-platforms.md` (the native generator's REST-vs-GraphQL author
description) and `docs/specs/02-configuration.md` (the `api_mode` trade-off table) to match.
Re-pointed `TestEnrichREST_JobToken` in `internal/forge/gitlab/gitlab_test.go` from `"Alice"` to
`"alice"` — the commit fixture carries both a name and an email, so the expected handle changes
under the new precedence; the assertion still proves a handle renders. Added
`internal/forge/gitlab/gitlab_internal_test.go` (`package gitlab`, testing the unexported
`gitAuthors` directly) since the existing `gitlab_test.go` is an external `gitlab_test` package.

#### `[x]` T165: dedicated `forges:` section in `docs/specs/02-configuration.md`

T160's docs pass migrated every stale `changelog.remote` / `commits.remote_metadata` reference in
`docs/specs/` to the new model, but pointed the prose at `docs/heraut.sample.yml` and ADR-0043
rather than a dedicated spec section — `docs/specs/02-configuration.md` has **no `## forges`
section**, because none existed to update. Since `docs/specs/` is this project's behavioural
authority (ranked above ADRs in `CLAUDE.md`'s source-of-truth hierarchy), the epic's headline config
surface should be specified there, not only in a sample file and a decision record. Write the
section: every `forges[]` field (`name`, `platform`, `project`/`repository`, `base_url`, `api_url`,
`api_mode`, `token_env`), the identity-resolution precedence (explicit config → CI env → git
`origin` → offline) with the fail-on-ambiguity rule, `commits.enrichment_forge` /
`enrichment_policy`, and `release.targets[]`. **Timing:** before the release that ships the breaking
config change, not open-ended — `docs/specs/` outranks ADRs in this repo's source-of-truth
hierarchy. **Scope:** S.

Added a `## Forges` section to `docs/specs/02-configuration.md`, placed between `## release`
(with a new `### release.targets[] (not yet functional)` subsection stating plainly that
targets are parsed/validated but do not drive publishing until P3) and `## commits` (whose
existing `enrichment_forge`/`enrichment_policy` subsection already referenced the not-yet-written
forges block). Covered: the field table (`name`, `platform` required; `project`, `repository`,
`base_url`, `api_url`, `api_mode`, `token_env` all inferred when omitted); identity resolution
precedence (explicit config → CI env → git origin → offline) with the exact per-platform CI
variables read (verified against `internal/forge/detect.go`) and the verbatim ambiguity error
(verified against `internal/forge/resolve.go`'s `ErrAmbiguousForge` wrapping); a cross-reference
to the existing `commits.enrichment_forge`/`enrichment_policy` prose instead of duplicating it;
the `rest` (default, job-token-friendly, `by @<name>`) vs `graphql` (opt-in, `read_api` token,
linked `@username`) trade-off, verified against `internal/forge/gitlab/{gitlab,graphql,rest}.go`
and ADR-0043 (the job-token-on-graphql failure is a resolution/enrichment-time error, not a
config-schema validation error — `internal/config/validator.go`'s `validateForges` comment says
resolution-time concerns are explicitly out of its scope); and a zero-config GitLab-CI example
plus a minimal explicit one, both with synthetic placeholders only. No Go code, `schema.json`,
or `docs/heraut.sample.yml` changed — verified `git diff --stat` is docs-only and
`go test ./internal/config/` stays green.

#### `[x]` T166: decide whether `forges:` resolves for non-native generators

`cfg.Forges` has exactly one non-validator consumer — `resolveEnrichForgeIfNeeded`
(`internal/app/pipeline.go`), gated on `usesNative(...)`. But the removed `changelog.remote` was a
**git-cliff** feature (ADR-0026). So a git-cliff user hitting the T160 migration error is told to
"replace with a top-level `forges:` entry and point `commits.enrichment_forge` at it" — which today
changes nothing for them: `ForgeIdentity` stays nil and git-cliff loses its explicit remote pin.
Worse, the behaviour is conditional in a user-invisible way: if the *release-notes* driver happens
to be native, `usesNative` is true and the git-cliff changelog *does* pick up the forge host.
Decide and implement one of: (a) resolve whenever `cfg.Forges` is non-empty regardless of generator
and feed git-cliff's `[remote.*]` injection from it, or (b) state plainly in the migration hint that
explicit enrichment pinning for git-cliff lands later. Found by Plan B's final review. **Scope:** M.

Chose option (b): reworded both `changelog.remote` migration-error sites in
`internal/config/loader.go` (the `removedKeys` table entry and the per-env
`environments.%s.changelog.remote` message) to state plainly that the `forges:` replacement drives
enrichment for `generator: native` only, and that explicit remote pinning for `generator: git-cliff`
is not carried over. Option (a) was rejected — wiring `forges:` into git-cliff would add plumbing to
a package slated for removal in the native-generator roadmap (Phase 2.5), so the honest-wording fix
is the correct stopping point rather than new integration work. No changes to `internal/app` or to
what `forges:` resolves for. Added
`TestLoad_RemovedKey_ChangelogRemoteHintMentionsNativeOnly` in `internal/config/migration_test.go`
asserting the message contains both `forges:` and `native`; existing migration tests asserting on
`forges:` alone are unaffected.

#### `[x]` T167: restore GraphQL enrichment's time-bounding (and pagination)

`internal/forge/gitlab/graphql.go` issues a single `commits(ref:,first:100)` +
`mergeRequests(state:merged,first:100)`, while the legacy path it replaces
(`internal/generators/native/enrich_gitlab.go`) both **paged** (`pageInfo{endCursor hasNextPage}`)
and **time-bounded** (`committedAfter`/`mergedAfter`, oldest commit − 1 min). The missing
`mergedAfter` is the more serious half: `mergeRequests(state:merged, first:100)` specifies no sort,
so *which* 100 MRs return depends on GitLab's default ordering rather than the release window — a
long-lived project could get MRs unrelated to the release and miss the relevant ones. Restore
`committedAfter`/`mergedAfter` at minimum, then add cursor pagination. Only affects the opt-in
`api_mode: graphql` path (REST is per-commit and unaffected). Found by Plan B's final review.
**Scope:** S–M.

Restored by mirroring `internal/generators/native/enrich_gitlab.go` exactly: split the single query
into `gqlCommitsQuery`/`gqlMRsQuery`, each paginated via `pageInfo{endCursor hasNextPage}` with an
empty-cursor guard (a malformed `hasNextPage:true` with no cursor stops instead of looping), and
each bounded to the release window via inlined `committedAfter`/`mergedAfter` (oldest commit date −
1 minute buffer, RFC3339, computed by the new `oldestCommitDate` helper). Arguments are inlined as
quoted strings via a local `gqlString` helper — deliberately **not** typed GraphQL variables —
because `httptest`-based tests perform no GraphQL validation and would pass either way; the inlined
shape is what was validated against a live instance during ADR-0042's spike. `postGraphQL` no
longer sends a `variables` object since all arguments are now embedded in the query string. Both
loops early-stop once every wanted SHA is resolved, matching the legacy optimization. No fixture
changes were needed for the pre-existing `TestEnrichGraphQL_LinkedUsernameAndHeader` test: a JSON
fixture omitting `pageInfo` decodes to the zero value (`hasNextPage: false`), which correctly
terminates pagination after one page.
