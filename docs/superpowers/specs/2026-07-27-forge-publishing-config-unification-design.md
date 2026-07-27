# Forge P3 — publishing config unification (`release.targets`) — Design

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-07-27
- **Author**: bchatard (with Claude)
- **Related**: ADR-0043 (forge abstraction), ADR-0025 (multi-instance platforms), ADR-0020/0021/0022
  (per-platform base_url / release notes / link context), ADR-0016 (bundled Docker image),
  roadmap T163
- **Supersedes (in part)**: the P3 framing in
  [`2026-07-24-forge-abstraction-design.md`](2026-07-24-forge-abstraction-design.md), which assumed
  P3 would also replace the publishing transport — see "Scope decision" below.

---

## Problem

The forge epic set out to replace two names for one concept. P1 and P2 delivered the enrichment
half: a single top-level `forges:` list, identity auto-resolved from CI env or git origin, and all
three platforms (GitHub, GitLab, Azure DevOps) enriching through `port.Forge`.

The publishing half is still on the old model. `release.platforms[]` carries its own `base_url`,
`token_env`, `repository`/`project` — duplicating what a forge entry already declares — and
publishing resolves none of it from the environment. So a user in CI must still spell out
coordinates that heraut can already detect, and the config still has two words for one thing.

`release.targets[]` was added in T155 and is parsed and validated today, but **nothing consumes
it**. This design makes it the publishing surface and retires `release.platforms`.

## Scope decision — config unification, not transport replacement

The original P3 framing also assumed publishing would move off the `gh`/`glab` CLIs onto a native
client (with an ADR choosing stdlib vs official SDKs). **That is explicitly not this design**
(user decision, 2026-07-27). Reasoning:

- **The epic's user-visible goal doesn't require it.** One config concept is achievable while
  keeping the current transport; the two goals are independent.
- **The original pain is already solved.** The epic began with `CI_JOB_TOKEN` enrichment failing;
  P1 fixed that. Publishing has no equivalent live defect — and `glab` already auto-authenticates
  on gitlab.com CI (`inCIAutologin`), so token-free publishing works there today.
- **Risk asymmetry.** P2 shipped two Critical defects (a wrong GHES GraphQL path, a duplicated
  Azure organization) that a fully green suite missed, because `httptest` validates no request
  shape. Publishing is the worst place to repeat that: `gh`/`glab` handle GitHub's separate
  `uploads.` host and GitLab's package-registry two-step correctly, for free.

**Therefore:** `port.Platform`, `internal/platforms/{github,gitlab}`, and the `gh`/`glab` runtime
dependency (including the Docker bundle, ADR-0016) are all **unchanged**. Only how those drivers are
*configured and constructed* changes.

Deliberately left open for a future, separately-motivated task: native publishing (which would drop
the CLI runtime dependency, shrink the image, and enable `CI_JOB_TOKEN` publishing on **self-hosted**
GitLab CI — the one gap `glab`'s autologin does not cover). Nothing here forecloses it: the drivers
sit behind `port.Platform`, so the transport can be swapped later without touching config.

## Goals

- `release.targets[]` drives publishing; each entry references a `forges[].name` and carries only
  publish behaviour (`draft`, `prerelease`, `assets`).
- Publishing **inherits identity resolution** — a target needs no host/project/token config when CI
  env or git origin can supply them.
- `release.platforms` is removed, with a clear migration error mapping old → new.
- Per-environment overrides keep working.
- Multi-instance publishing (ADR-0025) keeps working.

## Non-goals

- Replacing the publishing transport (see Scope decision).
- Azure DevOps publishing — Azure has never had a publish driver and remains enrichment-only.
- The `heraut init` wizard **redesign** — see "Wizard boundary".

## Design

### 1. Config

`release.targets[]` (already in `internal/config/config.go` from T155) becomes the publish list:

```yaml
forges:
  - name: Primary GitLab
    platform: gitlab
    # project / base_url / token_env optional — inferred from CI or git origin

commits:
  enrichment_forge: Primary GitLab      # which forge enriches
  enrichment_policy: optional

release:
  notes:
    generator: native
  targets:
    - forge: Primary GitLab             # → forges[].name
      draft: false
      prerelease: false
      assets: ["dist/*"]
```

Rules (matching the validation already written in T156):

- `forge` is optional when exactly one forge is configured/resolved, required when more than one.
- Omitting `release.targets` entirely publishes to the **single resolved forge** with default
  options. Today's "release requires ≥1 platform" becomes "release requires ≥1 resolvable forge";
  zero resolvable forges remains a config error, not a silent no-op.
- Top-level `release.assets` still applies to every target (with the existing lenient-glob
  semantics).

### 2. Per-environment overrides

`config.EffectivePlatforms(cfg, env)` resolves `environments.<env>.release.platforms` as a **full
replacement** of the top-level list. A sibling `config.EffectiveTargets(cfg, env)` mirrors that
exactly — same replacement (not merge) semantics — so per-env publishing is not silently lost.

### 3. Wiring

`internal/app` is the only place that constructs concrete implementations, and remains so:

1. resolve the forges (`forge.Resolve`, already used for enrichment),
2. for each effective target, look up its forge identity by name,
3. construct the existing `internal/platforms/{github,gitlab}` driver **from that
   `port.ForgeIdentity`** (host, project/repository, token) rather than from a `config.Platform`.

`port.Platform` — `Name`, `ReleaseURL`, `ReleaseURLFromContext`, `LinkContext`, `Check`,
`CreateRelease`, `HasAssets`, `UploadAssets` — is **unchanged**, so every existing platform contract
test still applies and proves the transport did not move. The drivers' internal config struct is fed
from the identity; their `gh`/`glab` invocations are untouched.

Consequence: publishing gains CI/git auto-detection for free, and the per-platform link context
(ADR-0021/0022) now derives from the same resolved identity as enrichment, so notes and links stay
consistent across both halves.

### 4. Migration

`release.platforms` is removed. The loader's existing removed-key mechanism (`ErrRemovedConfigKey`,
added in T160) gains entries for the top-level and per-env forms, each naming the replacement:
declare a `forges:` entry and reference it from `release.targets[].forge`, moving `base_url`,
`token_env`, and `repository`/`project` onto the forge and keeping `draft`/`prerelease`/`assets` on
the target. No silent aliasing.

`schema.json` and `docs/heraut.sample.yml` are updated in lockstep; `docs/specs/02-configuration.md`
(the behavioural authority) gains the `release.targets` documentation that T165 deliberately left
out while the key was inert.

### 5. Wizard boundary (P4 stays P4)

Removing `release.platforms` forces `internal/scaffold/` to change — it currently emits that block
(~10 sites across `wizard.go`, `generate.go`, `dropped.go`). The change here is **mechanical only**:
the wizard keeps its current prompts, flow, and `PlatformAnswer` internals, but *emits* `forges:` +
`release.targets` and round-trips from those keys. Generating valid config is a non-negotiable
consequence of the breaking change.

The **redesign** — forge-aware prompts, an `api_mode` question, auto-detection defaults so the
wizard can offer "detected GitLab CI, use it?" — remains **P4/T164**, deliberately after the new
config has been battle-tested.

### 6. `heraut check`

The runtime check still verifies `gh`/`glab` (the transport is unchanged). The config check's
platform section reads effective **targets** instead of platforms, and reports the resolved forge
identity per target so a user can see what auto-detection produced before releasing.

## Error handling

- Removed keys → `ErrRemovedConfigKey` with the old→new mapping, from both loader entry points and
  for per-env occurrences.
- `release.targets[].forge` naming an unknown forge, or absent with >1 forge → `ValidationErrors`
  (already implemented in T156).
- Zero resolvable forges on `heraut release` → config error naming the fix (declare `forges:`, or
  run where CI/origin can be detected).
- Existing publish-time errors (`gh`/`glab` failures, asset globs) are unchanged.

## Testing

- **Config**: `EffectiveTargets` per-env replacement (incl. empty-env and no-override cases);
  strict-parse fixtures; schema fixtures for `release.targets`.
- **Migration**: top-level and per-env `release.platforms` → `ErrRemovedConfigKey` naming the
  replacement.
- **Wiring**: a target resolves its forge by name and produces a driver carrying the identity's
  host/project/token; a target omitting `forge` with exactly one forge resolves to it; multi-forge
  publishing (ADR-0025) still constructs one driver per target.
- **Zero-config**: no `forges:`, no `release.targets`, CI env present → one driver constructed from
  the detected identity.
- **Unchanged**: every existing `internal/platforms/{github,gitlab}` contract test must still pass
  untouched — that is the evidence the transport did not move.
- **Hermetic**: resolution tests inject `getenv` (never real env — ambient CI leakage previously
  broke CI); no network; synthetic placeholders only.

## Migration impact (breaking, pre-v1.0)

Every existing config with `release.platforms` fails to load until migrated. Pre-v1.0 this is
acceptable with an ADR; the error carries the exact mapping. heraut's own `.heraut.yml` is migrated
as part of the work, which dogfoods the path.

## Records

- **New ADR** documenting: `release.targets` as the publishing surface, `release.platforms`
  removed, and — importantly — the decision **not** to replace the publishing transport, with the
  reasoning above so it is not re-litigated. It supersedes ADR-0043's P3 framing and updates
  ADR-0025's config surface (multi-instance publishing now means multiple targets/forges).
- Roadmap T163 completion note records the scope decision and the P3/P4 wizard boundary.
