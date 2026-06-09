# T75 — Fat-injection / thin git-cliff templates

> Rename this file to `t75-fat-injection-thin-templates.md` before the first commit that
> references it (plan-file naming convention).

## Context

User idea (2026-06-09): push the `HERAUT_*` approach to its conclusion so the embedded
git-cliff templates become **branch-free pure interpolation** — no `pr_link` `if/else`, no
`get_env` fallback chains, no `remote_url()` macro. heraut computes the per-platform URL
**prefixes** in Go and injects them; the template just interpolates.

When T75 was first refined it was *parked* behind the host-targeting thread, on the
assumption that the ambient-CI host fallback had to stay in Tera. **That assumption was too
conservative:** heraut can read the same ambient vars (`CI_PROJECT_URL`,
`GITHUB_SERVER_URL`+`GITHUB_REPOSITORY`) **in Go**. Relocating that fallback into Go lets the
templates go fully thin **now**, no host-targeting needed, and makes the fallback
table-testable. The ADR-0020 `base_url` **publish-gate is a separate concern** (it's about
`gh`/`glab` *publishing* to a self-hosted host) and is **left untouched**.

Confirmed decisions (2026-06-09): thin now via Go relocation; **fully thin** (no Tera
fallback, `default=""` only to avoid hard errors on standalone git-cliff runs); **git-cliff
only** (cocogitto already renders branch-free from cog's native `repository_url`); **keep**
`HERAUT_REMOTE_URL` + `HERAUT_PLATFORM` (emitted for custom templates even though the
default templates no longer read them).

## Variable contract (6 `HERAUT_*` env vars, full prefixes)

`{remote}` = the resolved repo root (`{base}/{owner}/{repo}`). `{base}` = host resolved in
Go (see below).

| Variable | GitHub | GitLab | Default template uses |
|----------|--------|--------|----------------------|
| `HERAUT_REMOTE_URL` | `{remote}` | `{remote}` | no (custom only) |
| `HERAUT_PLATFORM` | `github` | `gitlab` | no (custom only) |
| `HERAUT_COMMIT_URL` | `{remote}/commit/` | `{remote}/-/commit/` | yes |
| `HERAUT_PR_URL` | `{remote}/pull/` | `{remote}/-/merge_requests/` | yes |
| `HERAUT_PR_LABEL` | `#` | `!` | yes |
| `HERAUT_COMPARE_URL` | `{remote}/compare/` | `{remote}/-/compare/` | yes (changelog only) |

Thin template usage: commit `{{ HERAUT_COMMIT_URL }}{{ commit.id }}`; PR/MR
`[{{ HERAUT_PR_LABEL }}{{ n }}]({{ HERAUT_PR_URL }}{{ n }})`; compare
`{{ HERAUT_COMPARE_URL }}{{ previous.version }}..{{ version }}`.

## Host resolution (the relocated fallback, in Go)

Per generation, the effective `LinkContext` host is resolved:

1. **explicit `base_url`** (non-default) — currently never (ADR-0020 gate); future-ready.
2. **ambient CI var** — `CI_PROJECT_URL` → gitlab, else `GITHUB_SERVER_URL`+`GITHUB_REPOSITORY`
   → github. Used **only** for the single-platform release-notes case and the changelog
   (where ambient unambiguously describes the one target / origin). **Not** for
   multi-platform — ambient describes only the CI runner's platform and would
   cross-contaminate the other target.
3. **per-type default** (`github.com` / `gitlab.com`) + config `repository`/`project`.

Ambient resolver returns a `*port.LinkContext` with `BaseURL` = the full ambient root
(e.g. `CI_PROJECT_URL`) and **empty** `Owner`/`Repo` — `linkEnv` already composes
`{remote}` as `BaseURL[/Owner][/Repo]`, so a full root in `BaseURL` with empty owner/repo
yields the correct `{remote}` with no URL-splitting. Returns `nil` when no ambient is set.

Per-generation context selection:
- **multi-platform release notes**: each `plat.LinkContext()` (base_url-derived) — unchanged.
- **single-platform release notes**: `ambient() ?? plat.LinkContext()`.
- **changelog** (singular, origin): `ambient() ?? nil` — matches today's "ambient else no
  links" behaviour; `nil` → thin template renders empty link prefixes (`default=""`).

## Design / steps

1. **Extend `linkEnv`** (`internal/generators/gitcliff/generator.go`) to emit all 6 vars,
   computing the `/-/` (gitlab) vs `/` (github) path shapes from `lc.Platform`. This is the
   testability win — path-shape logic moves out of untestable Tera into a Go table test.
2. **Ambient resolver** — a small Go helper reading the CI env vars → `*port.LinkContext`
   (or `nil`). Home: pipeline-adjacent. Prefer resolving in the **app layer** and passing
   the resolved contexts into `pipeline.Config` (keeps `pipeline` env-pure and tests
   `t.Setenv`-free); acceptable alternative is a `pipeline` helper. Decide in implementation.
3. **Release pipeline** (`internal/pipeline/release.go`): the single-platform branch passes
   the resolved single context instead of `nil`; multi-platform unchanged. **Step structure
   and `releaseStepTotal` unchanged** — only the context *value* changes.
4. **Changelog pipeline** (`internal/pipeline/changelog.go:113`): pass the ambient-resolved
   context instead of `nil`.
5. **Rewrite both embedded TOMLs** (`cliff.changelog.toml`, `cliff.release-notes.toml`) to
   thin interpolation: delete the `remote_url()` and `pr_link()` macros and the `pr_link`
   `if/else`; replace each link with `{{ get_env(name="HERAUT_*_URL", default="") }}…`
   (and the `#`/`!` via `HERAUT_PR_LABEL`). Per [ADR-0010](../../docs/adr/0010-embedded-cliff-toml-default.md)
   these are user-facing byte changes — document the diff.
6. **cocogitto**: no change (already branch-free via `repository_url`).
7. **ADR-0022** (new): records the `HERAUT_*` variable contract, the thin-template design,
   the "embedded template requires heraut to populate the vars; standalone git-cliff gets
   empty prefixes" contract, the relocation of ambient detection from Tera → Go, the
   single-vs-multi ambient rule, that the ADR-0020 publish-gate is untouched, and that this
   **supersedes the T71a macro shape**.

## Behaviour change to call out

T75 **reverses T70b's "single-platform → `nil` context"** (and the changelog's `nil`): both
now pass a resolved context so the thin template has prefixes. This is deliberate. The
pipeline tests that assert `nil` for single-platform / changelog
(`TestRun_SinglePlatform_NotesNilContext`, the changelog `nil`-context assertions) must be
**updated** to assert the resolved context — note the reversal in the ADR + Done notes. The
generator still accepts `nil` (standalone/no-context), so the `gitcliff` `nil` tests stay.

## Files

- `internal/generators/gitcliff/generator.go` — extend `linkEnv` (+ unit table test in
  `generator_test.go`)
- `internal/generators/gitcliff/cliff.changelog.toml`, `cliff.release-notes.toml` — thin rewrite
- ambient resolver — new Go (app or pipeline) + test
- `internal/pipeline/release.go`, `internal/pipeline/changelog.go` — pass resolved context
- `internal/pipeline/release_test.go` (+ reporter test), `internal/pipeline/changelog_test.go` —
  update single/changelog context assertions
- `docs/adr/0022-*.md` (new), `docs/adr/README.md`, `docs/tasks/roadmap.md` (T75 done note;
  flip T70b note to reference the reversal)

## Suggested split (mirrors T70/T71 precedent)

- **T75a**: ADR-0022 + `linkEnv` + ambient resolver + release-pipeline always-inject +
  `cliff.release-notes.toml` thin + Go/byte tests + manual PoC.
- **T75b**: changelog-pipeline inject + `cliff.changelog.toml` thin + tests + PoC.

(One task is fine too; split keeps commits focused.)

## Verification

- **Go unit tests** (the payoff): `linkEnv` table test — `github`/`gitlab` `LinkContext`
  → the exact 6 values incl. the `/-/` paths and `#`/`!`. Ambient-resolver test —
  `CI_PROJECT_URL` → gitlab full-root ctx; `GITHUB_SERVER_URL`+`GITHUB_REPOSITORY` → github;
  neither → `nil` (`t.Setenv`).
- **Byte assertions** (`EffectiveReleaseNotesConfig`/`EffectiveChangelogConfig`): reference
  `HERAUT_COMMIT_URL`/`HERAUT_PR_URL`/`HERAUT_PR_LABEL`/`HERAUT_COMPARE_URL`; **no**
  `remote_url`/`pr_link` macro and **no** platform `if`. (`default=""` is allowed — it's an
  empty-guard, not a fallback chain.)
- **Pipeline tests**: multi → N per-platform contexts (unchanged); single → the resolved
  (ambient-or-platform) context, **not** `nil`; changelog → ambient-resolved context.
- **Manual real-git-cliff render PoC** (suite has no real-binary tests): multi-platform
  (distinct hosts), single-platform self-hosted via `CI_PROJECT_URL` (relocated ambient →
  correct self-hosted links — the non-regression), single-platform public (default), and
  the changelog. Confirm GitLab `/-/` paths and `#`/`!` labels render.
- **Update the T72 end-to-end integration test** if its single/multi expectations shift.
- `go test ./...` + `mise run lint:go:check` green; confirm single-platform & changelog
  links are still correct (now via Go-resolved context) — the load-bearing non-regression.
