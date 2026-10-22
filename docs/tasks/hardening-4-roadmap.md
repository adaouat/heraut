# Héraut — Hardening 4 Roadmap (e2e coverage)

> Status: In progress (drafted 2026-10-22)
> Source: the verification question of the refreshed modernization brief
> (`analysis/heraut/MODERNIZATION_BRIEF.md` § 7), answered 2026-10-22: the built binary's scenarios
> (`e2e/`, ADR-0066) should cover the behavior changed by [`hardening-roadmap.md`](hardening-roadmap.md) and
> [`hardening-2-roadmap.md`](hardening-2-roadmap.md). Main roadmap: tracked as Phase 77 in
> [`roadmap.md`](roadmap.md).

An audit of `e2e/` against the changes made since `71acdc5` found these behaviors with no scenario at all.
Each is already pinned by unit or pipeline tests; what is missing is a run of the **built binary** that
asserts stdout, the literal exit code and the repository state, which is what the e2e layer is for.

## Conventions

- Task IDs **continue the global sequence** (`T413+`).
- This file is the **single source of truth** for task status. Markers: `[ ]` not started, `[x]` done.
  Two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): write the scenario, run it, flip the
  marker and add a one-paragraph completion note.
- One task per session. Order is a recommendation.
- Lane A scenarios (`e2e/*_test.go`, no build tag) are hermetic: real `git` under `t.TempDir()`, no network,
  literal exit codes, no import of an `internal/` package. CalVer scenarios use the `heraut_testclock`
  binary and `HERAUT_TEST_NOW`. Lane B (`//go:build e2e_forge`) is opt-in and is never run by the agent
  without your say-so.
- **A scenario that passes on the first run proves nothing about its own sensitivity.** For each task,
  break the behavior it covers in a scratch copy (or reason about the exact line) and confirm the scenario
  fails, then record that in the completion note.
- A failing scenario is fixed at the root cause, never loosened. No scenario or test row is removed.

## Baseline

Recorded at `905de72`, Go 1.27.3 (the toolchain bump that fixed GO-2026-6809): `go test -count=1 ./...` passes in
every package; `go test ./e2e` passes (335 run entries, including subtests, in about 33 s). Phase exit
criteria include "the same command still passes".

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 77 | Built-binary scenarios for the changed behavior | T413–T418 | In progress |

---

### Phase 77 — Built-binary scenarios for the changed behavior

Goal: close the Lane A gaps the audit found, then (opt-in) re-run the asset gate against the real forges.
Entry: baseline above recorded.

#### `[ ]` T413: a canonical CalVer `--set-version` (T406, ADR-0074 addendum)

Under `calver` and `calver-per-env`, `--set-version` must parse under the format and equal its rendering.
Scenarios through `version next --set-version` and `release --dry-run`: a canonical value passes and prints
the tag (with and without a `tag_prefix`, and under `calver-per-env` with `--env`); `2026.5.3`, `2026.05.03`,
`x$(id)` and `2026.05.3;id` are refused with the Config exit code (2) and name the format; a refused value
leaves the repository untouched.

#### `[ ]` T414: command-line values that reach tags and hooks (T400, ADR-0074)

Scenarios: `--set-build-id` accepts `158404`, `run.12-a_b` and rejects `x$(id)`, `a/b` and a value with a
space (Config exit code, nothing tagged); `--env prod` under `semver` or `calver` is a Config error, while
`--env auto` keeps its own message. Also a hook that echoes `{{ .Tag }}` proves a refused value never reaches
a hook.

#### `[ ]` T415: the built-in `security` commit type (T404, ADR-0075)

Scenarios: `heraut commit verify` accepts `security: patch the token leak` and `security(auth): …`; a
changelog generated over a `security:` commit has a `Security` section right after `Bug Fixes` and before
`Refactor`; a commit of an unconfigured type with "security" in its body lands in `Other`; a repository that
sets `commits.types: [{name: security, remove: true}]` rejects `security:` in `commit verify`.

#### `[ ]` T416: the changelog write is atomic (T387)

Scenario: a `CHANGELOG.md` that is writable inside a directory made read-only (mode `0555`) is refused with a
Runtime exit code and a message, and the file's content is byte-identical afterwards; a normal run leaves no
`.CHANGELOG.md.tmp-*` file behind. Skipped when the process can write anyway (running as root).

#### `[ ]` T417: `version sprint bump` stays inside the `versioning:` block (T405)

Scenarios through `heraut version sprint bump`: the value changes with a trailing comment on the
`versioning:` line and on the `sprint:` line (both kept); a missing key is inserted with the block's
indentation; a `sprint:` line inside a hook script elsewhere in the file is untouched; an inline
`versioning: {…}` is refused and the file is unchanged.

#### `[ ]` T418: re-run the asset gate against the real forges (Lane B, opt-in)

`//go:build e2e_forge`, run only on your word with the sandbox tokens (see the e2e epic's notes in
`roadmap.md`). Scenarios: a pattern matching nothing is refused before anything reaches the remote (no tag, no
release, no extra commit); `release.allow_missing_assets: true` publishes without the asset; matching files
are attached, on GitHub and on GitLab. I write the scenarios; **you** decide when they run, because they
create and delete refs on the private sandbox repositories.

**Phase 77 exit:** `go test -count=1 ./...` passes with the new scenarios, each one confirmed to fail when the
behavior it covers is broken (noted per task), and lint is clean.

---

## Considered, not scheduled

- **`calver-per-env` highest-tag choice (T384, T399) through the binary.** The bug needs a tag set that git's
  `--sort=-version:refname` misorders, which a hermetic scenario cannot create with ordinary tags (T384's own
  note says the same). Covered by the resolver and `perenv` unit tests with a mocked tag order.
- `--set-version` on a taken tag (ADR-0073), UTC dates, the `stage` rules, the empty-changelog warning and
  the asset gate already have Lane A scenarios.
