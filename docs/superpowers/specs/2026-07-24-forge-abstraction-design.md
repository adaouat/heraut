# Forge Abstraction — unified, auto-configured remotes

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-07-24
- **Author**: bchatard (with Claude)
- **Related ADRs (extended / superseded)**: 0006 (naming), 0020 (base_url), 0023 (remote-metadata
  policy), 0025 (multi-instance platforms), 0026 (changelog.remote), 0034 (native enrichment via
  CLIs), 0035 (Azure native http), 0039 (commit-author attribution), 0040 (changelog.remote for
  native + base_url), 0041 (remote_metadata required + force), 0042 (GitLab GraphQL enrichment)
- **New ADR required**: yes — "Forge abstraction + config unification" (drafted during P1)

---

## Problem

Three problems, one root cause — the "remote" is not a first-class, self-configuring concept.

1. **`CI_JOB_TOKEN` cannot enrich.** heraut's GitLab enrichment is 100% GraphQL via `glab api
   graphql` (ADR-0042). GitLab's GraphQL API **structurally rejects job tokens** ("You cannot use
   job tokens to authenticate GraphQL requests" — GitLab docs), and `glab` sends whatever token it
   is given as `PRIVATE-TOKEN`, which GitLab also rejects for job tokens (the correct header is
   `JOB-TOKEN`). So in GitLab CI, the free, always-present `CI_JOB_TOKEN` produces no attribution
   and no MR links — the user must manually create a Personal/Project Access Token. That manual step
   is the pain we are removing.

2. **Config naming mismatch.** Enrichment is configured under `changelog.remote` ("remote"),
   publishing under `release.platforms` ("platform"). They describe the *same* underlying thing —
   a forge heraut talks to — under two names, in two places, with overlapping fields
   (`base_url`, `token_env`, `project`/`repository`).

3. **No clean extension point for new forges.** Enrichment lives as per-platform free functions
   (`enrichGitHub` / `enrichGitLab` / `enrichAzure`) dispatched in a `switch`; link-building is
   scattered; identity/auth resolution is ad hoc. Adding Gitea/Bitbucket/Forgejo means touching
   many call sites rather than implementing one interface.

## Goals

- **Zero-effort in CI.** With an empty (or absent) `forges:` block, `heraut changelog` / `heraut
  release` produce a fully enriched changelog/release notes using only CI-provided variables —
  no token creation, no host/project config.
- **One concept, one name.** A single top-level `forges:` list replaces `changelog.remote` +
  `release.platforms`. Each command decides what to *do* with a forge (enrich vs publish).
- **A `Forge` port** each platform implements, owning the three responsibilities the user named:
  **resolve config from the environment**, **build links**, **call the API (enrichment)**.
- **Fail loud on ambiguity.** Auto-detection resolves exactly one forge or errors; it never guesses.
- **GitLab works with `CI_JOB_TOKEN` out of the box** (REST), with an opt-in GraphQL path for users
  who provide a `read_api` token and want linked commit-author handles.

## Non-goals

- **Publishing rework in P1.** The `port.Platform` publisher (`glab/gh release create`) keeps its
  current interface; it is merely *fed* the resolved forge identity so publishing also benefits from
  CI autodetection. Folding publishing into `Forge` is P3.
- **Reading `glab`/`gh` stored credentials** as a token fallback. A possible later nicety; out of
  scope. Local auth is `token_env` → `GITLAB_TOKEN`/`GITHUB_TOKEN` → offline.
- **New public forges (Gitea/Bitbucket).** The interface is designed to admit them; none are
  implemented here.
- **Rewriting GitHub/Azure transports in P1.** GitHub keeps `gh api graphql` (its `GITHUB_TOKEN`
  already works everywhere); Azure keeps its `net/http` client. Both are migrated onto `port.Forge`
  in P2.

## Spike findings (authoritative — GitLab)

Established against GitLab's published docs:

1. **GraphQL never accepts job tokens.** REST is mandatory for the `CI_JOB_TOKEN` path.
2. **Token header depends on token kind.** A job token must be sent as `JOB-TOKEN`; a
   Personal/Project Access Token as `PRIVATE-TOKEN`. `glab`/`GITLAB_TOKEN` only ever sends
   `PRIVATE-TOKEN`, so `glab` cannot carry a job token — a native client that sets the correct
   header is required.
3. **The endpoints we need are on the `CI_JOB_TOKEN` allowlist — but not the ones git-cliff uses:**
   - ✅ `GET /projects/:id/merge_requests` (list)
   - ✅ `GET /projects/:id/repository/commits/:sha/merge_requests` (MRs that introduced a commit)
   - ❌ `GET /projects/:id/repository/commits` (**list**) — git-cliff calls this; **we do not need
     it**: heraut already has every commit from local `git log`.
4. **REST commits carry only the git author name**, not a linked GitLab `@username` (that field
   exists only in GraphQL). So the REST path renders `by @<git-author-name>` (identical in spirit to
   Azure, ADR-0035/T151); the GraphQL path renders the linked `@username` (today's ADR-0039/0042
   fidelity). MR author/merger handles carry a real `username` in **both** REST and GraphQL.

## Design

### 1. The `Forge` port

A new interface in `internal/port` (imports nothing from heraut, per the layer rules). One
implementation per type lives in a new `internal/forge/{gitlab,github,azure}` package. (Note: the
Go import path `github.com/adaouat/forge` is the unrelated CLI-framework dependency; the new local
package is `internal/forge` — different import path, no collision.)

Representative shape (exact types settled in the plan):

```go
package port

// Forge is one code-hosting platform heraut talks to. It resolves its own identity from
// config/CI/git, builds web links, and fetches PR/MR enrichment metadata for a set of commits.
type Forge interface {
    Type() string                                   // "github" | "gitlab" | "azure_devops"
    Identity() ForgeIdentity                        // resolved host/apiURL/project/token/apiMode
    CommitURL(sha string) string                    // links…
    ChangeURL(number int) string                    // PR "#N" or MR "!N"
    CompareURL(from, to string) string
    Enrich(commits []Commit) (Enrichment, error)    // per-commit PR/MR + author metadata
}

type ForgeIdentity struct {
    Type      string
    Host      string    // web host, e.g. https://gitlab.example.com
    APIURL    string    // API base, e.g. https://gitlab.example.com/api/v4
    Project   string    // owner/repo or group/subgroup/project
    Token     string
    TokenKind TokenKind // Job | Private — selects the auth header
    APIMode   string    // "rest" | "graphql"
}
```

`Enrichment` is the existing per-commit `{prs map[sha]PullRequest, authors map[sha]string}` result,
lifted into `port` (or a shared leaf package) so the native generator consumes it through the port
instead of owning it. The generator's job narrows to **rendering**; the forge owns **remote data**.

### 2. Config model — `forges:` (connection) + `release.targets:` (publish)

A forge entry describes only **how to reach a forge** — identity and transport. What to publish and
how (draft, assets) is release behavior and lives under `release.targets`, referencing a forge by
name. One top-level `forges:` list replaces `changelog.remote` and the connection half of
`release.platforms`.

```yaml
forges:                            # connection / identity only
  - name: Primary GitLab           # unique label (was Platform.Name)
    platform: gitlab               # type discriminator (unchanged key)
    # everything below optional — inferred from CI/git when omitted:
    # project:   group/subgroup/project
    # base_url:  https://gitlab.example.com
    # api_url:   https://gitlab.example.com/api/v4
    # api_mode:  rest              # default; or "graphql" (needs a read_api token)
    # token_env: GITLAB_API_TOKEN  # name of the env var holding the token

commits:
  # enrichment_forge: Primary GitLab  # which forge fetches PR/MR metadata (→ forges[].name);
  #                                     #   optional with one forge (defaults to it), required when >1
  enrichment_policy: optional          # how hard we try (disabled|optional|required); was remote_metadata

release:
  notes:
    generator: native
  targets:                         # publish behavior; references a forge by name
    - forge: Primary GitLab        # → forges[].name; optional when exactly one forge exists
      draft: false
      prerelease: false
      assets: ["dist/*"]
```

Forge fields (connection only):

| Field       | Was                          | Notes |
|-------------|------------------------------|-------|
| `name`      | `Platform.Name`              | unique within `forges:` (ADR-0025 multi-instance) |
| `platform`  | `Platform.Type` / `Remote.Type` | type discriminator, unchanged |
| `project` / `repository` | same | GitLab uses `project`, GitHub uses `repository`, Azure `organization/project` + repo |
| `base_url`  | both                         | web host override |
| `api_url`   | *new*                        | API base override (self-managed non-default API path) |
| `api_mode`  | *new*                        | `rest` (default) \| `graphql` — GitLab only in P1 |
| `token_env` | both                         | env var **name**, never a raw token |

The enrichment **source** is *not* a forge field — it is `commits.enrichment_forge` (§4), beside the
enrichment policy `commits.enrichment_policy`, keeping forge entries pure connection info.

Release-target fields (publish behavior):

| Field        | Was                          | Notes |
|--------------|------------------------------|-------|
| `forge`      | *new*                        | references `forges[].name`; optional when exactly one forge exists (defaults to the sole forge), required with >1 |
| `draft`      | `Platform.Draft`             | unchanged semantics |
| `prerelease` | `Platform.Prerelease`        | unchanged |
| `assets`     | `Platform.Assets` / `Release.Assets` | per-target globs; top-level `release.assets` still applies to all targets |

**Zero-config:**
- Omit `forges:` entirely → heraut resolves **one** forge from the environment (§3).
- Omit `release.targets` → `heraut release` publishes to the single resolved forge with default
  options. Today's "release requires ≥1 platform" becomes **"release requires ≥1 resolvable
  forge"** (explicit or auto-detected); zero resolvable forges is still a config error, not a
  silent no-op. Explicit `release.targets` are needed only to set publish options, select a subset,
  or publish to multiple forges.

`versioning` and `changelog` are otherwise unchanged; `commits` gains `enrichment_forge` and
renames `remote_metadata` → `enrichment_policy` (§4).

### 3. Resolution — from config, CI, or git

For each forge, `{host, apiURL, project, token, tokenKind}` is resolved with this precedence, field
by field (explicit config always wins per field):

1. **Explicit `forges:` config** (`base_url`, `api_url`, `project`, `token_env`).
2. **CI environment** — detected by the CI's own marker; the CI system pins the type unambiguously:

   | Forge  | Detect           | host                  | api                    | project             | token / kind              |
   |--------|------------------|-----------------------|------------------------|---------------------|---------------------------|
   | GitLab | `GITLAB_CI`      | `CI_SERVER_URL`       | `CI_API_V4_URL`        | `CI_PROJECT_PATH`   | `CI_JOB_TOKEN` → **Job**  |
   | GitHub | `GITHUB_ACTIONS` | `GITHUB_SERVER_URL`   | `GITHUB_API_URL`       | `GITHUB_REPOSITORY` | `GITHUB_TOKEN` → Private   |
   | Azure  | `TF_BUILD`       | `SYSTEM_COLLECTIONURI`| derived from host      | `SYSTEM_TEAMPROJECT`+repo | `SYSTEM_ACCESSTOKEN` |

3. **git `origin`** — parse `git remote get-url origin` → host + `owner/repo`. Type inferred from the
   host for **known public hosts** (github.com, gitlab.com, dev.azure.com). Token from `token_env` →
   `GITLAB_TOKEN`/`GITHUB_TOKEN` (kind **Private**) → none.
4. **none** → offline (no enrichment, no remote links).

**Ambiguity → hard error.** Auto-detection (no explicit `forges:`) must resolve exactly one forge
**type**. It fails when nothing pins a single type — e.g. not in CI, `origin` missing/unrecognized,
and tokens for more than one forge are present:

```
ambiguous forge: detected candidates [gitlab, github] and no CI/origin to disambiguate.
Declare a `forges:` block to choose explicitly.
```

CI and a recognized `origin` are authoritative — they win over stray tokens (a `GITHUB_TOKEN` lying
in the shell does not make a GitLab-origin repo ambiguous). **Publishing to multiple forges always
requires an explicit `forges:` block** — zero-config is single-forge by definition.

### 4. Multi-forge & the enrichment source

Commits have exactly one home — the forge that hosts their MRs/PRs — so enrichment uses a single
forge, selected by **`commits.enrichment_forge`** (a reference to `forges[].name`). It sits beside
`commits.enrichment_policy` because both are global enrichment settings governing **changelog *and*
release notes** uniformly; the forge entries stay pure connection info.

- **1 forge** (explicit or zero-config auto-detected) → `commits.enrichment_forge` is optional and defaults
  to that forge.
- **>1 forges** → `commits.enrichment_forge` is **required**; absent, or naming an unknown forge → validation
  error.
- Publishing iterates the **`release.targets`** forges (mirror-publish to GitLab + GitHub stays
  supported, ADR-0025) — independent of the enrichment source.
- In CI, `origin` is the CI forge, so the single detected forge is used with no `commits.enrichment_forge`
  needed.

### 5. GitLab forge — transport & fidelity

Native `net/http` for both modes (no `glab` for GitLab enrichment). Auth header follows
`TokenKind`: **Job → `JOB-TOKEN`**, **Private → `PRIVATE-TOKEN`**.

- **`api_mode: rest` (default).**
  - Commit-author `by @` = the **local git author name** (from `rawCommit`, no API call — the
    list-commits endpoint is not job-token-allowed and carries no linked handle anyway). Same render
    as Azure.
  - MR association: for each commit SHA, `GET /projects/:id/repository/commits/:sha/merge_requests`
    (job-token-allowed) → MR number, title, labels, author/merger `username`. (Per-commit; a
    list-and-invert optimization is possible later but not needed for correctness.)
  - Works with `CI_JOB_TOKEN` (Job header) **and** a PAT (Private header).
- **`api_mode: graphql` (opt-in).**
  - Requires a `read_api` token (`token_env`/`GITLAB_TOKEN`, kind Private). If only a job token is
    available → **validation error** with a clear hint (GraphQL rejects job tokens; use `rest` or
    supply a token).
  - Reuses the existing batched query logic (ADR-0042: `commits(ref:){author{username}}` +
    `mergeRequests`), POSTed via native `net/http` with `PRIVATE-TOKEN`. Renders the **linked
    `@username`** commit-author handle (today's fidelity).

### 6. Links

`CommitURL` / `ChangeURL` / `CompareURL` / `UserURL` are built from the resolved `{host, project}`,
centralizing logic currently scattered across the native generator's link context. The `!N` (MR) /
`#N` (PR) prefix is per-type.

### 7. Enrichment policy — unchanged

`commits.enrichment_policy: disabled | optional | required` (renamed from `remote_metadata`,
ADR-0023/0041) stays top-level and governs the `commits.enrichment_forge` forge. `--offline` still
forces `disabled`. Default `optional`
(tries, degrades gracefully, warns once) — which is what makes zero-config "beautiful by default":
it attempts enrichment and silently renders without it when the environment can't supply a
host/token.

## Error handling

- Ambiguous auto-detection → typed config/resolution error (§3), surfaced before any network call.
- `api_mode: graphql` with only a job token → validation error (§5) at config-validation time.
- `>1` forge with `commits.enrichment_forge` absent or naming an unknown forge → `ValidationErrors`
  (path `commits.enrichment_forge`, with a hint).
- Enrichment transport failures flow through `enrichment_policy` (formerly `remote_metadata`):
  `required` fatal, `optional` degrade — behavior unchanged from ADR-0041.
- All wrapped with `%w`; sentinel/typed errors at package boundaries per the coding rules.

## Testing

- **Resolution** (table-driven, `t.Setenv`): CI env → identity per forge; git-origin parse; explicit
  config precedence; ambiguity → error; single-forge implicit source; multi-forge `commits.enrichment_forge`
  required / unknown-name rules.
- **GitLab REST** (native http via `httptest.Server`): per-commit `commits/:sha/merge_requests`
  request shape + `JOB-TOKEN` vs `PRIVATE-TOKEN` header selection by `TokenKind`; MR author/merger
  username mapping; `by @git-name` author render.
- **GitLab GraphQL** (native http via `httptest.Server`): `PRIVATE-TOKEN` header; linked `@username`;
  job-token → validation error (no network call).
- **Config/schema**: `forges:` fixtures for each `platform` and `api_mode`; validation errors for
  missing/unknown `commits.enrichment_forge` and graphql-without-token; migration error for the old keys.
- **Determinism**: no network (httptest only), no clock (inject `now`), `t.TempDir()`, `t.Setenv`.
- **No real data**: synthetic placeholders only (`gitlab.example.com`, `group/subgroup/project`,
  `alice`) — never real hosts/orgs/usernames.

## Migration (breaking, pre-v1.0)

`changelog.remote` and `release.platforms` are **removed** in favor of `forges:` (connection) plus
`release.targets:` (publish behavior), and `commits.remote_metadata` is renamed
`commits.enrichment_policy`. Pre-v1.0 (build phase) this breaking change is acceptable *with an ADR*.
Rather than a silent alias, the loader emits a **clear migration error** mapping the old keys to
`forges:` / `release.targets:` / `commits.enrichment_policy` (with a before/after snippet in the
hint), so existing configs fail fast with actionable guidance. `schema.json`,
`docs/heraut.sample.yml`, and the specs are updated in lockstep (coding rules).

## Phasing

Each phase is its own implementation plan; the **design** (this spec) is landed once.

- **P1 — GitLab-first, end-to-end zero-config.** `port.Forge` + `internal/forge` + the `forges:` /
  `release.targets:` config + resolution (CI/git/ambiguity) + the GitLab native `net/http` forge
  (REST default / GraphQL opt-in, `TokenKind` header) + links + migration error + new ADR. GitHub
  and Azure are temporarily adapted to *feed* the resolver (their transports unchanged). Ships the
  pain relief.
- **P2 — GitHub + Azure onto `port.Forge`.** Migrate GitHub (`gh api graphql`) and Azure
  (`net/http`) to implement `Forge`; retire the enrichment `switch`.
- **P3 — Fold publishing into `Forge`.** Retire `release.platforms` internals; a forge becomes the
  single object for enrich + links + publish. **The publishing HTTP client is decided in P3's own
  ADR** — stdlib `net/http` vs official SDKs (`github.com/google/go-github`,
  `gitlab.com/gitlab-org/api/client-go`) for release-create + asset-upload — informed by the P1/P2
  build. A generic client like `resty` is rejected: an SDK gives typed endpoints for the same
  dependency cost. Enrichment (P1/P2) stays stdlib; because impls live behind `port.Forge`, adopting
  an SDK later would swap a forge's internals with no consumer churn.
- **P4 (last) — `heraut init` wizard.** Update the scaffold wizard (`internal/scaffold`) to generate
  `forges:` / `release.targets:` / `commits.enrichment_forge` (auto-detection defaults, `api_mode` prompt).
  Deliberately the **final** phase: the wizard is a convenience that codifies the config shape, so
  it lands only after the new config has been **battle-tested in real pipelines** (P1–P3), to avoid
  churning the wizard against a still-moving schema.

**Docs sync note:** `schema.json` and `docs/heraut.sample.yml` are updated in lockstep **with each
phase** (they are required for the config to function and validate — coding rules). Only the
interactive `heraut init` wizard is deferred to P4.

## Open items deferred to the plan

- Exact `Enrichment`/`Commit` type placement (`port` vs a shared leaf) and the `port.LinkContext` →
  `ForgeIdentity` transition (GitHub still uses `APIEnv()` for `gh` in P1).
- Per-commit vs list-and-invert for GitLab REST MR lookup (correctness-equivalent; per-commit first).
