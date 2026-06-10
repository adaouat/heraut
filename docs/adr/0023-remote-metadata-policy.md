# ADR-0023: Remote-metadata policy for content generation

- **Status**: Accepted
- **Date**: 2026-06-10
- **Deciders**: bchatard

---

## Context

heraut's embedded git-cliff templates reference `commit.remote.username` and
`commit.remote.pr_number` to render `by @author in #PR`. Those two fields are populated
only by git-cliff's GitHub/GitLab **remote integration**: git-cliff auto-detects the repo
from the git origin and fetches `/commits` + `/pulls` from the platform API (~16 calls for
this repo). The fetch is **fatal on failure** — when the API returns non-200, git-cliff
panics (exit 101), which surfaces as `git-cliff rejected config` in `heraut check` and
aborts changelog/release-notes generation.

This bit production: a release workflow's preflight `heraut check` 403'd because the step
exported only `GH_TOKEN` (which `gh`/`glab` read) and not `GITHUB_TOKEN` (which git-cliff
reads), so the call was unauthenticated and hit GitHub's 60/hr shared-IP rate limit. A CI
patch now exports `GITHUB_TOKEN` to the preflight + release steps, authenticating the
fetch. But that only fixes heraut's own CI: any user (or local) run of `heraut
changelog`/`check`/`release` **without** a token — or offline, or rate-limited even when
authenticated — still hits the same hard panic, with no way to opt out.

[ADR-0022](0022-fat-injection-thin-templates.md) moved all *link rendering* into Go-injected
`HERAUT_*` env vars. It never claimed to own *PR metadata* (author handle, PR number),
which only the integration can supply. So the `commit.remote.*` references are a clean,
deliberate boundary — heraut owns the link shapes, git-cliff supplies the PR facts — not a
regression of ADR-0022. The problem is purely that the fetch is **non-optional and
non-graceful**.

## Decision

Add a top-level **`remote_metadata`** policy controlling whether content generators fetch
remote PR/MR metadata. It governs **both** changelog and release-notes generation (both are
`*ContentDriver` sharing the embedded `commit.remote.*` templates), so it is a single
top-level key, not per-generator.

```yaml
remote_metadata: required | optional | disabled   # default: optional
```

| Value | Behaviour |
|----------|-----------|
| `required` | Fetch; a remote-fetch failure stays fatal (strict CI — the pre-T78 de-facto behaviour). |
| `optional` (default) | Try online; on any failure retry git-cliff with `--offline`, mark the run degraded, and warn. If the offline retry **also** fails, surface the original (online) error so a genuine config error still bubbles up. |
| `disabled` | Always pass git-cliff `--offline`; never reach the platform API (offline, no token). |

Backed by git-cliff's built-in `--offline` flag (verified: 0 API calls, no panic, renders
cleanly without the `@author`/`#PR` suffix; commit/compare links survive — they are
heraut-owned via `HERAUT_*`). A persistent **`--offline`** CLI flag forces `disabled` for a
single run.

The CLI override is intentionally a **boolean `--offline`** rather than a
`--remote-metadata <value>` mirror of the config key: it follows git-cliff's own `--offline`
and heraut's runtime-flag convention (`--dry-run` / `--force` are escape hatches, not config
mirrors — heraut has no `--strategy` / `--generator` / `--bump` flag either), and the
`required` / `optional` CLI overrides have no demonstrated use case (YAGNI). A full
`--remote-metadata` mirror can be added later if a need appears, keeping `--offline` as the
shorthand alias for `=disabled`.

The policy is applied to a `ContentDriver` copy by the app layer (`withEnvDerivations` for
the pipeline; `app.CheckCliff` for preflight), mirroring how `HeadingVersionPattern` is
propagated. The gitcliff generator implements the three branches in `runCliff` and exposes
`Degraded()`. Callers surface degradation without coupling to the concrete generator: the
pipeline type-asserts an optional `interface{ Degraded() bool }` and emits a step
sub-result; `heraut check` reports it in the cliff detail line. cocogitto/communique do not
fetch, so they treat non-`required` as a no-op.

### Why `optional` is the default

The goal is that heraut **never panics out of the box**. `optional` degrades a tokenless /
offline / rate-limited run into a successful one (minus PR attribution) plus a visible
warning, while `required` remains available for anyone who wants the fetch to be mandatory.
This turns the tokenless-panic residual into a graceful default.

### Why retry-on-failure, not predict-by-token

`optional` could instead skip the fetch when no `GITHUB_TOKEN`/`GITLAB_TOKEN` is present
(predict-by-token). That is simpler but only covers the missing-token case — a token that is
present yet rate-limited or offline would still panic. The original incident **was** a rate
limit, so retry-on-failure (degrade on *any* remote failure) is the faithful fix. The cost
is one extra git-cliff invocation on the failure path, which is acceptable for a degrade
path.

## Consequences

- heraut no longer hard-fails on remote-metadata problems by default; tokenless and offline
  runs work. This is a behaviour change from the implicit pre-T78 `required` default — an
  existing user who relied on the panic to flag a missing token now gets a warning instead.
- `heraut check` / `release` / `changelog` accept `--offline`; `remote_metadata` is in
  `schema.json` and `docs/heraut.sample.yml`.
- The CI `GITHUB_TOKEN` patch is complementary, not superseded: heraut's own releases stay
  **online + authenticated** (full notes with attribution); `optional` is the safety net for
  everyone else.
- `required` preserves the strict, fail-fast behaviour for callers who want it.
