# Design: Multi-instance same-platform releases

- **Date**: 2026-06-12
- **Status**: Proposed
- **Related**: [ADR-0020](../../adr/0020-platform-base-url.md) (per-platform `base_url`,
  "multi-instance thread" deferred), [docs/ideas.md](../../ideas.md) §"Multi-instance
  same-platform releases"

## Problem

`release.platforms` already accepts a list, but two entries of the *same type* (e.g. two
`gitlab` entries — one for `gitlab.com`, one for a private self-hosted instance) don't
work today:

1. `validatePlatformBaseURL` rejects any `base_url` that differs from the platform-type
   default (`https://gitlab.com` / `https://github.com`) — ADR-0020's deliberate gate.
2. Even with a valid `base_url`, `gh`/`glab` are never told which host to talk to —
   `CreateRelease`/`UploadAssets`/`Check()` would still hit the default public host.
3. `findPlatformCfg` looks up the first config entry by `Type` ("gitlab"/"github"), and
   `Platform.Name()` returns a hardcoded type string — both assume at most one entry per
   type, so reporter step labels and `heraut check runtime` rows would collide/ambiguous
   with two same-type entries.

This design lifts all three blockers, enabling configs like "publish to both
`gitlab.com` and a private `gitlab.example.com` in one `heraut release` run."

## Non-goals

- No per-environment merge logic for `release.platforms` (lists stay replace-wholesale,
  per ADR-0019 — unchanged).
- No `gh auth login` / `glab auth login` flows. Host targeting is per-invocation env
  injection only.
- No new platform types. Only `github` and `gitlab` remain valid.
- No changes to release-notes link resolution beyond what already exists
  (`LinkContext.BaseURL` already reads `cfg.BaseURL`; this design only unlocks
  non-default values).

## Design

### 1. Config schema

`internal/config/config.go`:

- Add `Platform.Name string `yaml:"name"`` — **required, non-empty**, for every entry in
  `release.platforms`. (Pre-v1.0: adding a required field is acceptable as a breaking
  change.)
- Validator (`internal/config/validator.go`):
  - Every `release.platforms[i]` must have a non-empty `name`.
  - `name` must be unique across `release.platforms` (case-sensitive exact match).
  - `validatePlatformBaseURL`: remove the "must equal type default" check. Any
    well-formed absolute `http(s)` URL is accepted (existing URL-shape validation stays).
    Empty still means "use the type default."

Example:

```yaml
release:
  platforms:
    - platform: gitlab
      name: gitlab-saas
      project: acme/widget
      # base_url omitted → https://gitlab.com
    - platform: gitlab
      name: gitlab-internal
      project: tools/widget-mirror
      base_url: https://gitlab.example.com
      token_env: GITLAB_INTERNAL_TOKEN
```

### 2. Host targeting

`internal/platforms/{github,gitlab}/platform.go`:

- New helper `hostEnv() []string`, derived from `p.cfg.BaseURL`:
  - Empty or equal to the type default (`https://gitlab.com` / `https://github.com`) →
    `nil` (no injection — today's behavior, byte-for-byte).
  - Otherwise → host is the `base_url` with scheme stripped.
    - GitLab: `["GITLAB_HOST=<host>"]`
    - GitHub: `["GH_HOST=<host>", "GH_ENTERPRISE_TOKEN=<token>"]` — the same token value
      configured via `token_env` is also exported as `GH_ENTERPRISE_TOKEN`, because `gh`
      resolves enterprise-host tokens from that variable and we don't want to depend on
      `gh`'s host-based precedence rules.
- Every `RunEnv` call (`CreateRelease`, `UploadAssets`, and the `Check()` auth probes)
  merges `hostEnv()` with the existing `tokenEnvSlice()`.
- `ReleaseURL()` and `LinkContext()` already read `p.cfg.BaseURL` with a default
  fallback (`gitlabBaseURL`/`githubBaseURL` constants become pure defaults, used only
  when `base_url` is empty) — no further change needed.

### 3. `Check()` / auth probes

Compute `selfHosted := cfg.BaseURL != "" && cfg.BaseURL != <type default>`.

- **`selfHosted == true`**:
  - Skip the CI-autologin branch entirely (`GITLAB_CI`+`CI_PROJECT_ID` /
    `GITHUB_ACTIONS`+`GITHUB_TOKEN`) — that autologin is tied to the *ambient* CI host,
    not necessarily this platform's configured host.
  - `token_env` is required; missing it is a hard `Check()` error (not silently skipped).
  - The auth probe (`glab api projects/{id}/releases` / `gh api repos/{owner}/{repo}/releases`)
    runs via `RunEnv(hostEnv() + tokenEnvSlice(), ...)` against the configured host.
- **`selfHosted == false`**: unchanged — existing CI-autologin branches still apply,
  zero behavioral diff for current single-instance configs.

### 4. Disambiguation & lookups

- `port.Platform.Name()` returns `cfg.Name` (was a hardcoded `"github"`/`"gitlab"`
  literal). This alone fixes pipeline step labels (`"Publish to gitlab-internal"`),
  error-wrapping (`"platform %s: ..."`), the dry-run output, and the summary table — no
  further pipeline changes needed.
- `internal/app/check.go`: `findPlatformCfg(cfg, typ string)` (first-match-by-type) is
  removed. `heraut check runtime`'s "Platforms" section changes from one row per CLI
  type (`glab`, `gh`) to **one row per configured `release.platforms` entry**, labeled by
  `name`. Binary-presence (`glab --version` / `gh --version`) is checked once per
  distinct CLI type and the result reused across same-type entries (not repeated per
  entry).

### 5. Docs / schema / ADR

- `schema.json`: add `name` (required string, `Platform` definition's `required` list);
  rewrite `base_url`'s description to drop the "self-hosted hosts are not yet supported"
  caveat and document host-targeting env vars.
- `docs/heraut.sample.yml`: add `name:` to existing platform example(s); add a
  commented-out second-instance example (self-hosted GitLab).
- `docs/specs/05-generators-and-platforms.md`: document `name`, multi-instance configs,
  and the `GH_HOST`/`GITLAB_HOST`/`GH_ENTERPRISE_TOKEN` injection rules.
- New ADR-0025 superseding ADR-0020's gate: records `name` as required, the gate lift,
  the `hostEnv()` injection rule (non-default `base_url` only), and the `Check()`
  self-hosted auth path. ADR-0020 gets a "Superseded by ADR-0025" note.

### 6. Testing

- **Config/validator** (unit): required `name`, uniqueness across `release.platforms`,
  non-default `base_url` now accepted, malformed `base_url` still rejected.
- **Platform contract tests** (`MockRunner`): `hostEnv()` present/absent for default vs.
  non-default `base_url`, for `CreateRelease` and `UploadAssets`, both platforms;
  `ReleaseURL()` reflects `base_url`.
- **`Check()` contract tests**: self-hosted branch — no CI-autologin, hard error on
  missing `token_env`, probe call carries `hostEnv()`.
- **Pipeline**: existing multi-platform tests updated to add required `name`; new test
  with two `gitlab` entries (one default, one self-hosted) verifying distinct step
  labels, distinct `ReleaseURL`s, and that per-platform `LinkContext`/notes regeneration
  (already implemented) works unchanged for the self-hosted entry.
- **`heraut check runtime`**: test with two same-type platform entries produces two
  distinct rows, binary check not duplicated.
- **Schema**: valid fixture with two same-type platforms + distinct `name`s; invalid
  fixture missing `name`; invalid fixture with duplicate `name`s.

## Open questions

None — all prior open points (disambiguation key, `name` defaulting, `Check()`
auto-login behavior, host-targeting mechanism) were resolved during brainstorming.

## Rollout

This is sized for decomposition into several roadmap tasks (config/validator schema
changes; host-targeting + `Check()` changes per platform; lookup/disambiguation; docs +
ADR). Task breakdown happens in the implementation plan, one roadmap task implemented
per session per project convention.
