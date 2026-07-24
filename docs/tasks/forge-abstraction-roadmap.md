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
| P1 — GitLab-first: `port.Forge` + config + resolution + native REST/GraphQL + links | T154–T160 | Not started |
| P2 — migrate GitHub + Azure onto `port.Forge`            | T161, T162  | Not started |
| P3 — fold publishing into `port.Forge`                   | T163        | Not started |
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

#### `[ ]` T158: GitLab REST forge (native `net/http`)

`internal/forge/gitlab` implementing `port.Forge` over native `net/http` (no `glab`). REST default:
per-commit `GET /projects/:id/repository/commits/:sha/merge_requests` for MR refs
(number/title/labels/author/merger `username`); commit-author `by @` = the **local git author name**
(no API call — as Azure). Auth header follows `TokenKind`: **Job → `JOB-TOKEN`**, **Private →
`PRIVATE-TOKEN`**. Links (`CommitURL` / `ChangeURL` `!N` / `CompareURL`). Tests: `httptest.Server`
request-shape + header-selection + mapping + `by @git-name` render.

#### `[ ]` T159: GitLab GraphQL forge mode (opt-in)

`api_mode: graphql` path: reuse the batched query logic (ADR-0042 — `commits(ref:){author{username}}`
+ `mergeRequests`) POSTed via native `net/http` with `PRIVATE-TOKEN`, rendering the **linked
`@username`** commit-author handle. Guard: graphql + only a job token → the validation error from
T156 (no network call). Tests: `httptest.Server` GraphQL path; `PRIVATE-TOKEN` header; linked-handle
render; job-token → error.

#### `[ ]` T160: pipeline wiring + old-path removal

Wire the forge end-to-end: changelog + release-notes enrichment resolves the `port.Forge` from
`commits.enrichment_forge` and applies `commits.enrichment_policy` (unchanged degrade/required
semantics; `--offline` forces disabled); the native generator consumes `port.Forge`. Publishing
reads `release.targets` (default: the single resolved forge with default options). GitHub and Azure
are adapted to *feed* the resolver (transports unchanged). Perform the breaking config migration:
rename `commits.remote_metadata` → `commits.enrichment_policy`, remove the now-dead
`changelog.remote` / `release.platforms` config paths and the `enrich()` GitLab-graphql-via-glab
path, and emit a **clear migration error** when any removed key is present (map old → new with a
before/after hint; no silent alias). Tests:
integration — zero-config GitLab CI changelog (enriched via `CI_JOB_TOKEN`); happy-path + dry-run
release to a resolved forge.

---

## Phase 2 — migrate GitHub + Azure onto `port.Forge`

Bring the other two platforms under the interface so enrichment is uniform and the `enrich()` switch
retires.

#### `[ ]` T161: GitHub forge onto `port.Forge`

Migrate GitHub enrichment (`gh api graphql`, `GITHUB_TOKEN`) to a `internal/forge/github`
implementing `port.Forge`. Transport may stay `gh api` (its token works everywhere) or move to
native http — decided at plan time. Tests: contract parity with today's GitHub enrichment.

#### `[ ]` T162: Azure forge onto `port.Forge` + retire the switch

Wrap the existing Azure `net/http` enrichment (ADR-0035) as `internal/forge/azure` implementing
`port.Forge`; remove the per-platform `enrich()` dispatch switch in favor of the interface. Tests:
Azure contract parity; switch removal doesn't regress GitHub/GitLab.

---

## Phase 3 — fold publishing into `port.Forge`

#### `[ ]` T163: publishing via `port.Forge`

Give `port.Forge` a publish capability (or a sibling constructed from the same resolved identity),
retire the `release.platforms`-era `port.Platform` split, and have `release.targets` drive publish
through the forge. A forge becomes the single object for enrich + links + publish. Tests: contract
tests for `gh`/`glab` (or native) release create, fed the resolved identity.

**Publishing HTTP client — own ADR (decided here).** Whether release-create + asset-upload use
stdlib `net/http` or official SDKs (`go-github` / `gitlab-org/api/client-go`) is decided in this
phase, informed by the P1/P2 build. A generic client like `resty` is rejected (an SDK gives typed
endpoints for the same dependency cost). Enrichment (P1/P2) is stdlib; the `port.Forge` abstraction
lets the GitLab/GitHub impls swap to the SDK too, if chosen, without consumer churn.

---

## Phase 4 (last) — `heraut init` wizard

Deliberately last: the wizard codifies the config shape, so it lands only after the new config is
**battle-tested in real pipelines** (P1–P3), to avoid churning it against a moving schema.

#### `[ ]` T164: `heraut init` generates the forge config

Update the scaffold wizard (`internal/scaffold`) to generate `forges:` / `release.targets:` /
`commits.enrichment_forge` / `commits.enrichment_policy`, with auto-detection defaults and an
`api_mode` prompt. Tests: wizard-output fixtures validate against `schema.json`.
