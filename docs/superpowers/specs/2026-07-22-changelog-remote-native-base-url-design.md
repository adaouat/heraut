# `changelog.remote` for native + `base_url` host override — Design

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-07-22
- **Author**: bchatard (with Claude)
- **Related**: ADR-0026 (changelog.remote), ADR-0035 (Azure native enrichment), ADR-0039 (commit-author attribution), roadmap T150 (GitLab commit-author handle)

---

## Problem

Running `heraut changelog --regenerate` **locally** against a self-hosted GitLab repo
(config: `changelog.generator: native`, one `release.platforms` GitLab entry, no
`changelog.remote`) produces a changelog with **no attribution, no commit links, and no
compare/tag links**. Verified in `/tmp/release-notes` (remote
`git@gitssh.cross-systems.ch:internal-tools/ci-cd/catalog/release-notes.git`): the
`--verbose` trace shows **zero `glab api` calls** — the native generator received a `nil`
`LinkContext` and skipped all link rendering and enrichment.

### Root cause

The changelog-only pipeline resolves its link context with a **shorter** fallback chain
than the release pipeline:

| | Release pipeline (`heraut release`) | Changelog-only pipeline (`heraut changelog`) |
|---|---|---|
| Code | `Pipeline.changelogLinkContext()` (`internal/pipeline/linkctx.go`) | inline in `ChangelogPipeline.Run()` (`internal/pipeline/changelog.go:121-124`) |
| Chain | `changelog.remote` → ambient CI → **single configured platform** | `changelog.remote` → ambient CI → **stops (nil)** |

Locally there is no `changelog.remote`, and no CI env vars (`CI_SERVER_URL` /
`GITHUB_SERVER_URL`), so the chain runs out and `changelogCtx == nil`.

The natural fix — declare a `changelog.remote` block — is **blocked two ways**:

1. **Validator gate** (`internal/config/validator.go:408`): `changelog.remote` is rejected
   unless the generator is `git-cliff`. A `native` config cannot use it at all.
2. **Hardcoded host** (`internal/pipeline/linkctx.go`, `remoteLinkContext()`): the `github`
   and `gitlab` branches always set `BaseURL` to the platform default (`github.com` /
   `gitlab.com`). There is no field to point them at a self-hosted host. (`azure_devops`
   has an `api_url` override; `github`/`gitlab` have nothing.)

Everything downstream already works: `remoteLinkContext()` is step 1 of *both* pipelines;
`LinkContext.APIEnv()` already derives `GITLAB_HOST` / `GH_HOST` from a non-default
`BaseURL`; native's GitLab enrichment already routes `glab` through `lc.APIEnv()`; and both
generators receive the resolved context via the same `Generate(tag, changelogCtx)` call.
Only the two blockers above stand in the way.

## Goal

Make `changelog.remote` usable with the `native` generator, and let it declare a
self-hosted host via a single `base_url` field applied to **all three** remote types, so
that a local (and CI) changelog regeneration renders correct commit links, compare/tag
links, and `in [!N]` MR references against a self-hosted GitLab (or GHES, or on-prem Azure
DevOps) instance.

## Non-goals (explicitly out of scope)

- **`by @` commit-author attribution for GitLab.** GitLab commit lines still render no
  `by @<handle>` — resolving a GitLab commit-author handle needs an API spike and remains
  **T150**. (An *offline* attribution fallback from the per-commit git author was raised
  and **deferred** to a later discussion, after this change.)
- **A single-platform fallback for `heraut changelog`.** We deliberately keep the
  changelog-only pipeline's chain as `changelog.remote → ambient`; the explicit
  `changelog.remote` block is the mechanism, not an implicit fallback to
  `release.platforms`. (The asymmetry with the release pipeline is intentional and noted.)
- **git-cliff behavior changes.** git-cliff already consumes the same `LinkContext`; it
  benefits from `base_url` for free, but nothing git-cliff-specific is added or altered.

## Design

### 1. Config schema change (breaking — allowed pre-v1.0)

On the `changelog.remote` block (the `config.Remote` struct), **remove `api_url` and add
`base_url`** — one web/API host override that applies to **all three** remote types:

```yaml
changelog:
  generator: native            # previously rejected with a remote block; now allowed
  output: CHANGELOG.md
  remote:
    type: gitlab
    project: internal-tools/ci-cd/catalog/release-notes
    base_url: https://git.cross-systems.ch   # was hardcoded to gitlab.com
    # token_env: GITLAB_TOKEN (default)
```

Per-type semantics of `base_url` (optional; falls back to the per-type default when unset):

| type | `base_url` overrides | default |
|---|---|---|
| `github` | web/API host (fixes GHES) | `https://github.com` |
| `gitlab` | web/API host (self-managed) | `https://gitlab.com` |
| `azure_devops` | web/API host (on-prem Server) — **takes over exactly what `api_url` did** | `https://dev.azure.com` |

`api_url` is **removed**, not deprecated (pre-v1.0; no migration shim). Any config still
setting `api_url` fails strict parsing (`unknown key`), which is the desired loud failure.

`config.Remote` after the change:

```go
type Remote struct {
    Type       string `yaml:"type"`
    Project    string `yaml:"project,omitempty"`
    Repository string `yaml:"repository,omitempty"`
    TokenEnv   string `yaml:"token_env,omitempty"`
    BaseURL    string `yaml:"base_url,omitempty"` // replaces APIURL; host override for all types
}
```

### 2. `remoteLinkContext()` behavior (`internal/pipeline/linkctx.go`)

Each of the three branches sets `BaseURL` from `r.BaseURL` when non-empty, else the
per-type default. A small helper keeps it uniform:

```go
func remoteBaseURL(configured, platformType string) string {
    if configured != "" {
        return strings.TrimRight(configured, "/")
    }
    return config.DefaultBaseURL(platformType) // "" for azure → azureDevOpsDefaultBaseURL applied in-branch
}
```

- `github`: `BaseURL: remoteBaseURL(r.BaseURL, "github")`, Owner/Repo from `Repository`.
- `gitlab`: `BaseURL: remoteBaseURL(r.BaseURL, "gitlab")`, Owner/Repo split from `Project`.
- `azure_devops`: `BaseURL` from `r.BaseURL` else `azureDevOpsDefaultBaseURL`
  (`https://dev.azure.com`) — the current `r.APIURL` line, renamed. Owner = `Project`,
  Repo = `Repository`.

Nothing else in these branches changes (token resolution, `Platform` tag, owner/repo
split). Because `BaseURL` already drives both rendered links and `LinkContext.APIEnv()`
(`GITLAB_HOST` / `GH_HOST` for non-default hosts), self-hosted enrichment works with **no
new plumbing** in the generators.

Note `DefaultBaseURL("azure_devops")` returns `""` today; azure's default
(`azureDevOpsDefaultBaseURL`) is applied in the azure branch, exactly as the current code
does — the helper is used for github/gitlab, and the azure branch keeps its explicit
default. (Implementation may inline rather than force the helper to know about azure.)

### 3. Validator changes (`internal/config/validator.go`)

Two edits in `validateContentDriverRemote`:

1. **Lift the generator gate.** Remove the block that rejects a remote when
   `d.Generator != "git-cliff"` (lines ~408-414). `changelog.remote` becomes valid for the
   changelog driver regardless of generator (`git-cliff` and `native` are the only
   generators; both consume the `LinkContext`). The existing guard that rejects a remote on
   the **release-notes** driver (`.notes`) stays.
2. **Rename the host validation.** The existing `api_url` URL check (lines ~461-466) becomes
   a `base_url` check: when `r.BaseURL != ""`, it must be an absolute `http(s)` URL
   (reuse `isValidBaseURL`). Error path `changelog.remote.base_url`, hint updated (e.g.
   `base_url must be an absolute http(s) URL, e.g. https://git.example.com`).

Type/required-field validation (project/repository per type) is unchanged.

### 4. Data flow (unchanged plumbing, newly reachable)

```
config.Load  →  ContentDriver.Remote (type, project/repository, base_url, token_env)
     │
ChangelogPipeline.Run  →  remoteLinkContext(cfg.ChangelogRemote)
     │                        → LinkContext{BaseURL(self-hosted), Owner, Repo, Platform, Token}
     ▼
Generator.Generate(tag, lc)   (native OR git-cliff — same call)
     ├─ links:      version compare + commit URLs built from lc.BaseURL/Owner/Repo
     └─ enrichment: native → glab/gh via lc.APIEnv() (GITLAB_HOST/GH_HOST from BaseURL)
                    git-cliff → GITHUB_REPO/GITLAB_REPO + remote host from lc
```

## Error handling

- **Removed `api_url`:** strict loader raises `unknown key "api_url"` with a line number —
  the intended loud failure guiding users to `base_url`.
- **Invalid `base_url`:** semantic validation returns a `ValidationError` at
  `changelog.remote.base_url` with a fix hint. No silent fallback to a default host.
- **`native` + remote:** now passes validation; no error.
- **Missing token / unreachable host at generation time:** unchanged — governed by the
  `remote_metadata` policy (`optional` degrades with a warning via `Degraded()`; `required`
  is a hard error). Not part of this change.

## Testing

Unit + contract, TDD (failing test first) per repo rules:

- **`remoteLinkContext` (`linkctx_internal_test.go`):**
  - `github` with `base_url` → `BaseURL` honored; without → `github.com`.
  - `gitlab` with `base_url` → honored (incl. self-hosted host like
    `https://git.cross-systems.ch`) and subgroup owner/repo split preserved; without →
    `gitlab.com`.
  - `azure_devops` with `base_url` → replaces the old `api_url` case; without →
    `https://dev.azure.com`.
- **Validator (`validator_test.go`):**
  - `changelog.remote` + `generator: native` → **valid** (was the rejection case).
  - `changelog.remote` + `generator: git-cliff` → still valid.
  - `base_url` not an http(s) URL → error at `changelog.remote.base_url`.
  - remote on `.notes` driver → still rejected.
  - existing type/required-field cases unchanged.
- **Strict loader:** a config with the removed `api_url` key → `unknown key` error.
- **End-to-end native enrichment (self-hosted host):** a native-generator test with a
  `gitlab` `LinkContext` whose `BaseURL` is self-hosted asserts the `glab` call carries
  `GITLAB_HOST=<host>` (via `APIEnv`) and that links render against that host. (Extends the
  existing `enrich_gitlab` contract tests; MockRunner, no network.)
- **Schema (`schema_test.go` + fixtures):** a `native` + `remote` (with `base_url`) fixture
  validates against `schema.json`; an `api_url` fixture, if any, is removed/renamed.

## Documentation & records to sync (per coding rules)

- **`schema.json`:** in `definitions.Remote`, remove `api_url`, add `base_url`
  (description: host override for github/gitlab/azure_devops with per-type defaults); update
  the `ContentDriver.remote` description from "git-cliff only" to "git-cliff or native".
- **`docs/heraut.sample.yml`:** update the commented `remote:` example — `api_url` →
  `base_url`, note it applies to all three types, show a gitlab self-hosted example, and
  reflect that `native` is supported.
- **`docs/specs/05-generators-and-platforms.md`:** update the `changelog.remote` section
  (native support, `base_url` replacing `api_url`).
- **ADR-0040 (new):** "`changelog.remote` for native + `base_url` host override" — records
  (a) lifting the git-cliff-only restriction, (b) unifying host override on `base_url` and
  removing `api_url`, and (c) that GitLab commit-author `by @` stays out of scope (T150).
  References/updates ADR-0026 and ADR-0035 (whose `api_url` mention is superseded).
- **Roadmap:** add a task entry (with the two-step completion note on completion). The
  self-hosted-local-changelog fix is the deliverable; T150 and the offline-attribution
  fallback are logged as separate follow-ups.

## Migration note

Breaking, intentionally (pre-v1.0, per ADR-less "not stable yet" latitude the user
granted): configs using `changelog.remote.api_url` must rename it to `base_url`. Only
Azure DevOps Server users set `api_url` today; the loader's `unknown key` error makes the
required rename obvious.
