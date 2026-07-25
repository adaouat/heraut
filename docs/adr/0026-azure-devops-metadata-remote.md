# ADR-0026: `changelog.remote` — explicit metadata remote for git-cliff

- **Status**: Accepted
- **Date**: 2026-06-22
- **Deciders**: bchatard

---

> **Update (2026-07-24):** The `changelog.remote` block this ADR defines was **removed** by
> [ADR-0043](0043-forge-abstraction.md), which unifies it with the connection half of
> `release.platforms` into a single top-level `forges:` list. A `forges` entry supplies the same
> connection/identity facts (`platform`/`type`, `project`/`repository`, `base_url`, `api_url`,
> `token_env`) plus the new `api_mode`; `commits.enrichment_forge` names which `forges` entry
> supplies changelog/release-notes PR/MR metadata, replacing the implicit changelog-only fallback
> chain this ADR introduced. Loading a config with `changelog.remote` now fails with a migration
> error pointing at `forges:` / `commits.enrichment_forge`.

## Context

[T113](../tasks/roadmap.md) taught heraut to auto-inject git-cliff's `[remote.github]` /
`[remote.gitlab]` sections plus `GITHUB_REPO`/`GITLAB_REPO` env vars, deriving owner/repo
from whichever entry in `release.platforms` matches via `port.Platform.LinkContext()`.

git-cliff 2.13 also supports an Azure DevOps remote (confirmed empirically and via
git-cliff's own CLI arg docs):

- `[remote.azure_devops]` with `owner = "<organization>/<project>"` and
  `repo = "<repository>"` — Azure DevOps repos are addressed by a **3-segment**
  `organization/project/repository` path, unlike GitHub/GitLab's 2-segment `owner/repo`.
- `AZURE_DEVOPS_TOKEN` (token) and `AZURE_DEVOPS_REPO` (`organization/project/repository`)
  env vars, mirroring `GITHUB_TOKEN`/`GITHUB_REPO` and `GITLAB_TOKEN`/`GITLAB_REPO`.

heraut does not publish releases to Azure DevOps and nothing in this request asks for that
— the only need is **PR/author metadata enrichment** for git-cliff, the same value T113
already delivers for GitHub/GitLab. Routing this through `release.platforms`/`port.Platform`
would mean adding `"azure_devops"` to the `Platform.Type` enum and implementing
`CreateRelease`/`UploadAssets` as dead code purely to unlock `LinkContext()`.

Separately, there is a real, pre-existing gap T113 didn't address: `changelogLinkContext()`
(`internal/pipeline/linkctx.go:36`) resolves the **changelog's** link context via ambient CI
env vars → the sole configured platform (if exactly one) → `nil` (bare hashes, no PR
metadata). This is the *only* content driver with this ambiguity — release notes never hit
it, because `platformLinkContext()` is always invoked **per platform being published to**;
there is always a concrete platform. The changelog, being a single shared artifact not tied
to one publish target, has no equivalent anchor and today has no explicit config knob to pin
one — it is purely env/ambient-derived.

These two threads converge: Azure DevOps metadata is exactly the kind of remote that needs
an explicit pin (it is never a `release.platforms` entry), and the changelog is exactly the
content driver that lacks one.

## Decision

Add **`changelog.remote`**, a single, type-discriminated object (not a list — "only one
remote" reflects that a project's commit/PR history has exactly one source of truth):

```yaml
changelog:
  generator: git-cliff
  remote:
    type: azure_devops              # github | gitlab | azure_devops (room for bitbucket/gitea later)
    project: my-org/my-project      # azure_devops: "organization/project" — matches
                                     # git-cliff's own azure_devops "owner" shape exactly
    repository: my-repo             # shared field name across types
    token_env: AZURE_DEVOPS_TOKEN   # default per type; overridable
    api_url: https://dev.azure.com  # optional — Azure DevOps Server (on-prem) only
```

`type` is generic and reuses the same discriminator pattern as `Platform.Type` — it is not
Azure-DevOps-only. This also lets a user explicitly pin `github`/`gitlab` when
`changelogLinkContext()`'s ambient/single-platform fallback is ambiguous today (multiple
platforms configured, or a CI host `changelogLinkContext` doesn't recognize) — a free
improvement to an existing minor gap, served by the same injection point, not scope creep.

`changelogLinkContext()` checks `cfg.Changelog.Remote` first, before its existing
ambient → single-platform → `nil` chain, so behavior is unchanged when it's absent.

**Release notes are untouched.** `release.notes` keeps relying on `release.platforms` via
`platformLinkContext()` — it has no equivalent gap, so no `release.notes.remote` is added.

**git-cliff only**, like `tickets:` (ADR-0024) — `changelog.remote` with a non-git-cliff
generator is a config error.

Field naming mirrors `Platform`'s existing convention of one combined-path field per type
rather than splitting it: `repository` (github, `owner/repo` string), `project` (gitlab,
`namespace/.../repo` string; azure_devops, `organization/project` string). Exactly two
fields per type (`project`/`repository` for azure_devops), matching git-cliff's own
`owner`/`repo` shape one-for-one — there is no separate `organization` field.

### Why not `release.platforms`

`release.platforms`/`port.Platform` model *where heraut publishes a release*. Azure DevOps
here is *where git-cliff reads PR metadata from* for the changelog — a different concern
that doesn't require publish capability. Adding a publish-shaped enum value purely to reach
a metadata method pollutes a contract every other implementor treats as "this type can
publish a release." If Azure DevOps release publishing is wanted later, that's a separate,
larger decision (a real `internal/platforms/azuredevops/` package implementing
`port.Platform`) with its own ADR.

### Why `changelog.remote`, not a top-level block

An earlier draft of this ADR proposed a top-level `remote:` block, mirroring `tickets:` and
`remote_metadata:` (both top-level, propagated to every content driver). That precedent
doesn't actually fit here: `tickets`/`remote_metadata` solve problems that are identical for
changelog and release notes (which tickets to link; whether to fetch at all). The
remote-context-selection problem is **not** identical — release notes already has a
deterministic remote (the platform being published to); changelog does not. Scoping the
config to where the actual gap lives avoids inventing a meaningless
`release.notes.remote` and keeps the YAML honest about what each block does.

## Consequences

- `changelogLinkContext()` gains a first-priority branch reading `cfg.Changelog.Remote`,
  constructing a matching `port.LinkContext` before falling through to the existing chain.
- `internal/generators/gitcliff/generator.go`'s `injectRemote`/`linkEnv` — currently a
  two-armed `if lc.Platform == "gitlab" {...} else {...}` — are generalized to a small table
  keyed by remote type (TOML `owner`/`repo` shape + env var names), reused by both the
  auto-derived path (release.platforms → T113) and this new explicit `changelog.remote` path.
- `schema.json`, `docs/heraut.sample.yml`, `docs/specs/`, and `internal/config/validator.go`
  need the standard config-field checklist treatment ([`coding.md`](../../.claude/rules/coding.md)):
  type enum (`github`/`gitlab`/`azure_devops`), required-field-per-type validation, git-cliff
  gating.
- No release-to-Azure-DevOps capability is added or implied.
