# ADR-0025: Multi-Instance Same-Platform Releases

- **Status**: Accepted
- **Date**: 2026-06-12
- **Deciders**: bchatard

---

## Context

[ADR-0020](0020-platform-base-url.md) introduced `base_url` as the per-platform source of
truth for a platform's web host, but deferred two of its three consumers (`ReleaseURL`/
`LinkContext` and CLI host targeting via `GH_HOST`/`GITLAB_HOST`) to a "multi-instance
thread", and — until that thread landed — the validator rejected any `base_url` that
differed from the platform-type default:

```
release.platforms[1].base_url: self-hosted hosts are not yet supported
```

This ADR is that thread. The motivating scenario: a project publishes releases to both a
public GitLab instance (`gitlab.com`, e.g. for a CI/CD Catalog component) and a private
self-hosted GitLab instance (`gitlab.example.com`, the project's primary remote), in one
`heraut release` run. `release.platforms` already supports a list, so two entries of type
`gitlab` are structurally possible — but three things break with two entries of the same
type:

1. **`base_url` is gated** to the type default (ADR-0020) — a self-hosted second instance
   cannot be configured at all.
2. **`gh`/`glab` are not told which host to talk to.** Both CLIs default to
   `github.com`/`gitlab.com`; without `GH_HOST`/`GITLAB_HOST` (and, for GitHub Enterprise,
   `GH_ENTERPRISE_TOKEN`), a release "published" to the self-hosted entry would actually
   hit the public host.
3. **Two entries of the same type are indistinguishable.** `internal/app/check.go`'s
   `findPlatformCfg` returns the *first* match by type, so `heraut check runtime` would
   report on only one of the two GitLab entries — silently dropping the other — and
   `Platform.Name()` returned a bare `"gitlab"`/`"github"` constant, so even error
   messages and the `heraut check runtime` table couldn't tell the two apart.

## Decision

### 1. `config.Platform` gains a required, unique `name` field

```yaml
release:
  platforms:
    - platform: gitlab
      name: gitlab-com
      project: acme/widget-catalog
    - platform: gitlab
      name: gitlab-internal
      project: acme/widget
      base_url: https://gitlab.example.com
```

`name` is a free-form label (not a platform type) that **must be unique** within each
`release.platforms` list — checked independently for the top-level list and for each
environment override's list (per [ADR-0019](0019-perenv-content-driver-merge.md), env
overrides replace the list wholesale, so uniqueness is scoped to whichever list is
actually in effect). It is the label used in `heraut check runtime`'s Platforms section
and in any per-entry error message. There is no default — a missing `name` is a config
error, because a silently-generated name (e.g. `"gitlab-2"`) would be useless in error
output and would change if entries are reordered.

### 2. The `base_url`-equals-default gate is lifted

`validatePlatformBaseURL`'s check that `base_url` must equal the platform-type default is
removed entirely. The remaining shape check (`isValidBaseURL`: must be an absolute
`http(s)://` URL) is kept — heraut still rejects garbage values, just not non-default
*hosts*.

### 3. `hostEnv()` — per-platform CLI host targeting

Both `internal/platforms/{github,gitlab}` gain:

```go
func (p *Platform) selfHosted() bool {
    return p.cfg.BaseURL != "" && p.cfg.BaseURL != <type default>
}

func (p *Platform) hostEnv() []string // nil for the default host
```

- **GitLab self-hosted**: `hostEnv()` returns `["GITLAB_HOST=<host>"]`.
- **GitHub self-hosted (GHES)**: `hostEnv()` returns `["GH_HOST=<host>",
  "GH_ENTERPRISE_TOKEN=<token>"]`, where `<token>` is the value of the platform's
  configured `token_env` — GitHub Enterprise Server requires `GH_ENTERPRISE_TOKEN` rather
  than `GH_TOKEN` for non-`github.com` hosts.

`hostEnv()` is merged into every `RunEnv` call the platform makes (`CreateRelease`,
`UploadAssets`, and the `Check()` API-auth probe). For the default host, `hostEnv()`
returns `nil`, so `RunEnv(append(tokenEnv, nil...), ...)` is byte-identical to the
pre-T84/T85 `RunEnv(tokenEnv, ...)` call — existing single-instance configs are
unaffected.

### 4. `Check()` skips CI autologin for self-hosted instances

Both `gh` and `glab` support ambient CI autologin (`GITHUB_ACTIONS`/`GITHUB_TOKEN` and
`GITLAB_CI`/`CI_JOB_TOKEN` respectively) — but that autologin always targets the CI
runner's *own* host (`github.com` / `gitlab.com` for hosted runners), never a self-hosted
target configured separately. So when `selfHosted()` is true, `Check()` skips the
CI-autologin branch entirely and always validates the configured `token_env` via
`RunEnv(tokenEnvSlice + hostEnv, "gh"/"glab", "api", ...)`. This is a hard requirement: a
self-hosted entry with no token configured is a `Check()` failure, even in CI, because
there is no autologin path that could possibly reach it.

> **Update (2026-08-25, T215):** narrowed for `glab` only. The premise above — "autologin
> always targets the CI runner's own host, never a self-hosted target configured
> separately" — assumed the self-hosted target is necessarily a *different* host from the
> one CI is running on. It doesn't hold when a project runs its own GitLab CI entirely on
> its own self-hosted instance and publishes back to that same instance: `CI_SERVER_URL`
> (the CI runner's own host) and the configured `base_url` are then identical, and `glab`'s
> CI-native detection reaches that host correctly, same as it does for `gitlab.com`. This
> regressed silently between v0.56.0 and v0.57.0: pre-T163, `release.platforms[].base_url`
> was never auto-filled from CI env, so an unset `base_url` always read as "not
> self-hosted" and CI autologin was (accidentally) trusted; T163's correct host resolution
> made `selfHosted()` fire for the first time in that same zero-`base_url` shape, newly
> requiring an explicit `token_env` a working self-hosted-CI pipeline hadn't needed before.
> `internal/platforms/gitlab/platform.go`'s `inCIAutologin()` now trusts CI autologin when
> self-hosted *and* the configured `base_url` names the same host as `CI_SERVER_URL`
> (`sameGitLabHost`) — a self-hosted `base_url` naming a genuinely different instance (this
> ADR's actual multi-instance scenario) still hits the hard requirement above unchanged.
> GitHub Enterprise Server (`internal/platforms/github`) was not part of T215's scope and
> keeps the original blanket self-hosted requirement described above; GHES's own CI
> (`GITHUB_ACTIONS` on a GHES-hosted runner) was not evaluated for the same narrowing.

### 5. `Name()`, `ReleaseURL()`, `LinkContext()` honor configured values

- `Name()` returns `p.cfg.Name` (was a hardcoded `"github"`/`"gitlab"`).
- `ReleaseURL()` and `LinkContext()` use `p.cfg.BaseURL`, falling back to the type default
  (`https://github.com` / `https://gitlab.com`) when unset — this was ADR-0020's deferred
  "consumer 2", now wired alongside consumer 3 since both touch the same files.

### 6. `heraut check runtime` — one Platforms row per configured entry

`internal/app/check.go`'s Platforms section previously dispatched exactly two fixed rows
("glab", "gh") via `configuredPlatforms`/`findPlatformCfg` (first-match-by-type). It now
dispatches **one row per `release.platforms` entry**, labeled by that entry's `name`,
running that entry's full `Check()`. `configuredPlatforms` and `findPlatformCfg` are
removed — they assumed at most one entry per platform type. When no platforms are
configured (or `cfg` is `nil`), the section falls back to a binary-only probe of `glab`
and `gh`, matching the pre-existing nil-config / no-`release`-block behavior.

**Deferred**: checking each CLI binary (`glab`/`gh`) only once per type and reusing the
result across same-type entries (e.g. two `gitlab` entries sharing one `glab --version`
probe) is *not* implemented — each entry's `Check()` runs its own binary probe. This is
functionally harmless (an extra subprocess spawn per duplicate-type entry) and can be
revisited if it becomes a real cost.

## Consequences

**Positive**

- Two instances of the same platform type — the entire point of this ADR — now work:
  configurable, individually named, individually checked, and individually targeted by
  `gh`/`glab`.
- `ReleaseURL`/`LinkContext` and `heraut check runtime` no longer silently collapse
  multiple entries of the same type into one.
- Single-instance configs are byte-for-byte unaffected: default `base_url` → `hostEnv()`
  is `nil` → identical `RunEnv` calls; `name` becomes required but is a one-line addition
  to existing configs (and `heraut init` defaults/dedupes it — see T83).

**Negative / trade-offs**

- `name` is a new required field — a wire-compatibility break for existing `.heraut.yml`
  files (mitigated: `heraut init`-generated configs are updated, and the migration is a
  single added line per platform entry; `heraut check config` reports the missing field
  with a clear hint).
- The binary-presence-dedup optimization described in the original design note is
  deferred (see "Deferred" above) — `heraut check runtime` makes one extra `--version`
  call per additional same-type entry. Negligible in practice.
- `GH_ENTERPRISE_TOKEN` vs `GH_TOKEN` is a real `gh` CLI distinction that heraut now has to
  track per platform entry — a small but permanent piece of GitHub-specific knowledge in
  `internal/platforms/github`.

## Alternatives considered

- **Auto-generate names from type + index (e.g. `gitlab-1`, `gitlab-2`).** Rejected: names
  appear in error messages and the `heraut check runtime` table; an index-derived name is
  meaningless to a user and changes if entries are reordered. An explicit, required `name`
  is one line and is permanently stable.
- **Keep `findPlatformCfg`'s first-match semantics and add a second type-keyed map for
  "extra" instances.** Rejected: every consumer (`heraut check runtime`, error messages,
  `ReleaseURL`) would need to handle "the first one" vs "the others" differently, doubling
  the surface area for a problem that a per-entry loop solves uniformly.
- **Deep-merge env-level platform overrides instead of replace.** Rejected: out of scope —
  [ADR-0019](0019-perenv-content-driver-merge.md) already settled list-replace semantics
  for `release.platforms`, and revisiting it here would couple two unrelated decisions.
- **Implement binary-presence dedup now.** Rejected for this ADR: see "Deferred" above —
  the bookkeeping cost is real and the benefit (avoiding one extra `--version` subprocess
  call) is marginal; flagged as a candidate follow-up rather than blocking this ADR.
