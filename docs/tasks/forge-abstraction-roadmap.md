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

#### `[ ]` T154: ADR-0043 + `port.Forge` contract

Write **ADR-0043** (forge abstraction + config unification), distilled from the design spec — the
permanent decision record that anchors the epic. Add the `port.Forge` interface (`Type`,
`Identity`, `CommitURL` / `ChangeURL` / `CompareURL`, `Enrich`), `ForgeIdentity`
(`{Type, Host, APIURL, Project, Token, TokenKind, APIMode}`), and `TokenKind` (`Job` | `Private`).
Move the enrichment result types (`Enrichment` = `{prs, authors}`, `PullRequest`, `Author`) into
`port` (or a shared leaf) so the native generator consumes them through the port. No behavior yet —
contract + record. Tests: interface compiles; type round-trips.

#### `[ ]` T155: config — `forges:` + `release.targets:` + `commits.enrichment_*`

Add the config structs: `Forge` (`name`, `platform`, `project`/`repository`, `base_url`, `api_url`,
`api_mode`, `token_env`), `release.targets[]` (`forge`, `draft`, `prerelease`, `assets`); add
`commits.enrichment_forge` and rename `commits.remote_metadata` → `commits.enrichment_policy`.
Strict loader (unknown keys → error with line numbers). Sync **`schema.json`** and
**`docs/heraut.sample.yml`** in lockstep (coding rules). Tests: strict-parse fixtures per `platform`
and `api_mode`; schema validates each valid fixture.

#### `[ ]` T156: config validation + migration error

Semantic validation (`internal/config/validator.go`): `enrichment_forge` optional with one forge
(defaults to it), **required** with >1, error on unknown name; `api_mode: graphql` requires a
resolvable token (error, with hint, when only a job token is available); `release.targets[].forge`
must reference a known forge (optional with a single forge). Emit a **clear migration error** when
the removed `changelog.remote` / `release.platforms` / `commits.remote_metadata` keys are present,
mapping old → new with a before/after hint (no silent alias). Tests: table-driven validation +
migration-error fixtures.

#### `[ ]` T157: forge identity resolution (config / CI / git / ambiguity)

Resolve `{host, apiURL, project, token, tokenKind}` per forge with precedence **explicit config →
CI env → git `origin` → none**. CI detection: GitLab (`GITLAB_CI` → `CI_SERVER_URL` / `CI_API_V4_URL`
/ `CI_PROJECT_PATH` / `CI_JOB_TOKEN`→Job), GitHub (`GITHUB_ACTIONS` → `GITHUB_SERVER_URL` /
`GITHUB_API_URL` / `GITHUB_REPOSITORY` / `GITHUB_TOKEN`→Private), Azure (`TF_BUILD` →
`SYSTEM_COLLECTIONURI` / `SYSTEM_TEAMPROJECT` / `SYSTEM_ACCESSTOKEN`). Zero-config resolves exactly
one forge; **fail loud** on ambiguity (nothing pins a single type + multiple forge tokens). git
`origin` parse yields host + project for known public hosts. Tests: table-driven with `t.Setenv`;
CI-env, origin-parse, precedence, and ambiguity-error cases.

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
are adapted to *feed* the resolver (transports unchanged). Remove the now-dead `changelog.remote` /
`release.platforms` config paths and the `enrich()` GitLab-graphql-via-glab path. Tests:
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

---

## Phase 4 (last) — `heraut init` wizard

Deliberately last: the wizard codifies the config shape, so it lands only after the new config is
**battle-tested in real pipelines** (P1–P3), to avoid churning it against a moving schema.

#### `[ ]` T164: `heraut init` generates the forge config

Update the scaffold wizard (`internal/scaffold`) to generate `forges:` / `release.targets:` /
`commits.enrichment_forge` / `commits.enrichment_policy`, with auto-detection defaults and an
`api_mode` prompt. Tests: wizard-output fixtures validate against `schema.json`.
