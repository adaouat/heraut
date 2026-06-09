# ADR-0022: Fat Injection / Thin git-cliff Templates

- **Status**: Accepted
- **Date**: 2026-06-09
- **Deciders**: bchatard

---

## Context

[ADR-0021](0021-per-platform-release-notes.md) made heraut inject a per-platform
`LinkContext`, and [T71a](../tasks/roadmap.md) taught the embedded git-cliff templates to
prefer heraut-injected values with an ambient-CI fallback. That left the templates carrying
real logic: a `remote_url()` macro with a three-level `get_env` fallback chain, and a
`pr_link()` macro that branched on platform (`#`+`/pull/` for GitHub vs `!`+
`/-/merge_requests/` for GitLab). Tera branching is **untestable** in heraut's suite
(heraut doesn't render Tera — the external tool does), duplicated across two TOMLs, and
hard to extend for a new platform or self-hosted path quirk.

The natural conclusion of the `HERAUT_*` approach: have heraut compute the **fully-formed
URL prefixes** in Go and inject them, so the templates become **branch-free pure
interpolation**. This was initially deferred behind the host-targeting thread on the
assumption the ambient-CI host fallback had to stay in Tera. That assumption was wrong:
heraut can read the same ambient vars (`CI_PROJECT_URL`, `GITHUB_SERVER_URL` +
`GITHUB_REPOSITORY`) **in Go**. Relocating that fallback into Go unblocks the thin
templates now and makes the fallback table-testable.

The ADR-0020 `base_url` **publish-gate** (forbidding a non-default `base_url` because
`gh`/`glab` self-hosted publishing isn't wired) is a separate concern and is **left
untouched** — this ADR is about *link rendering*, not publishing.

## Decision

heraut computes the per-platform URL **prefixes** in Go (`internal/generators/gitcliff`,
`linkEnv`) and injects them as env vars; the embedded git-cliff templates interpolate them
with **no `if/else`, no `remote_url()` macro, and no `get_env` fallback chains** (only a
`default=""` empty-guard).

### Variable contract (6 `HERAUT_*` env vars)

`{remote}` = `{base}/{owner}/{repo}` (the resolved repo root).

| Variable | GitHub | GitLab |
|----------|--------|--------|
| `HERAUT_REMOTE_URL` | `{remote}` | `{remote}` |
| `HERAUT_PLATFORM` | `github` | `gitlab` |
| `HERAUT_COMMIT_URL` | `{remote}/commit/` | `{remote}/-/commit/` |
| `HERAUT_PR_URL` | `{remote}/pull/` | `{remote}/-/merge_requests/` |
| `HERAUT_PR_LABEL` | `#` | `!` |
| `HERAUT_COMPARE_URL` | `{remote}/compare/` | `{remote}/-/compare/` |

`HERAUT_REMOTE_URL` and `HERAUT_PLATFORM` are still emitted (from ADR-0021/T71a) but the
default templates no longer read them — they remain available for users writing custom
templates. The default templates read the four prefix vars (compare is changelog-only).
The GitLab `/-/` routing and the `#`/`!` glyph are the only per-platform knowledge, now in
the table-tested `linkEnv`.

### Host resolution (ambient relocated Tera → Go)

The effective host in each `LinkContext` is resolved in Go:

1. explicit non-default `base_url` — currently never (ADR-0020 gate); future-ready.
2. **ambient CI host** — `CI_PROJECT_URL` → gitlab, else `GITHUB_SERVER_URL` +
   `GITHUB_REPOSITORY` → github. The ambient `LinkContext` carries the full root in
   `BaseURL` with empty `Owner`/`Repo`; `linkEnv` composes the same `{remote}` from it.
3. per-type default (`github.com` / `gitlab.com`) + config `repository`/`project`.

Per generation:

- **multi-platform release notes** — each platform's own `LinkContext()` (base_url-derived).
  Ambient is **not** consulted: it describes only the CI runner's platform and would
  cross-contaminate the other target.
- **single-platform release notes** — ambient **only if it matches the target platform**
  (`amb.Platform == platform`), else the platform's own context. The match guard prevents a
  mismatched CI (e.g. a GitHub release built in GitLab CI) from stamping the wrong host —
  it also fixes a latent bug in the old Tera detection, which keyed purely on
  `CI_PROJECT_URL` presence.
- **committed changelog** (singular, tied to origin, generated in both the release pipeline
  Step 2 and the changelog pipeline) — the ambient host (origin), else `nil`.

### Standalone contract

The embedded templates are heraut's defaults and **require heraut to populate the
`HERAUT_*` vars**. Running git-cliff standalone against them (no heraut) yields empty link
prefixes (`default=""` → e.g. `([abc1234](abc1234…))`) — degraded but **not an error**.
Users who invoke git-cliff directly keep their own template.

## Consequences

**Positive**

- The per-platform path-shape logic (the source of the old `if/else`) is now in Go, **unit
  table-tested** (`TestLinkEnv`), as is the ambient resolver (`TestAmbientLinkContext`).
- The templates are branch-free interpolation — flatter, identical-shaped across both
  TOMLs, trivial to extend for a new platform or self-hosted path quirk (a Go change).
- Single-platform self-hosted CI is preserved (ambient read in Go, with a platform-match
  guard that *improves* on the old Tera behaviour).

**Negative / trade-offs**

- **Reverses [ADR-0021]/T70b's "single-platform → `nil` context".** Single-platform
  releases and the changelog now always pass a resolved context (so the thin template has
  prefixes). Deliberate; the pipeline tests asserting `nil` were updated. The generator
  still accepts `nil` (standalone).
- **Supersedes the T71a/T74 macro shape.** The `remote_url()`/`pr_link()` macros, the
  platform `if/else`, the `get_env` fallback chain, and the literal `/pull/` (T74) are all
  removed from the templates; the `/pull/` vs `/pulls/` correctness now lives in `linkEnv`
  (`TestLinkEnv`).
- User-facing embedded-template byte change ([ADR-0010](0010-embedded-cliff-toml-default.md)):
  anyone who copied the old defaults keeps the old behaviour; anyone relying on the embedded
  defaults gets the new thin rendering (functionally equivalent for heraut-driven runs).
- The pipeline now reads ambient CI env vars (in `ambientLinkContext`). Contained in one
  `t.Setenv`-tested helper.

## Alternatives considered

- **Keep parked behind host-targeting** (the prior decision). Rejected once it was clear the
  ambient fallback can move to Go: host-targeting is only needed for self-hosted
  *publishing*, not link rendering.
- **Hybrid** (Go prefixes for multi-platform, keep the Tera ambient fallback for
  single-platform). Rejected: not fully thin, keeps the branch, more surface — strictly
  dominated by relocating the fallback to Go.
- **Static `default()` for standalone** (e.g. default to GitHub shape). Rejected in favour
  of empty prefixes: a static default is wrong for standalone GitLab and the embedded
  template's contract is "heraut populates the vars" anyway.
- **cocogitto parity changes.** None needed — cocogitto already renders branch-free from
  cog's native `repository_url`.
