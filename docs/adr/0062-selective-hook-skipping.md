# ADR-0062: Selective hook skipping — `--skip-hook` and `HERAUT_SKIP_HOOKS`

- **Status**: Accepted
- **Date**: 2026-09-20
- **Deciders**: bchatard

---

## Context

[ADR-0053](0053-release-lifecycle-hooks.md) gave `heraut release` and `heraut changelog` a
`--no-hooks` flag: skip every configured hook for one run, without editing `.heraut.yml`. It is
all-or-nothing. A run that needs the `pre_tag` build gate but not the `post_release` Slack
notification — a re-run after a partial failure, a staging release that shouldn't notify, a CI
job that has already run the build gate — has to either edit config or lose every hook.

Hook steps carry no identifier (`run` and, per [ADR-0061](0061-hook-file-staging.md), `stage` —
nothing else), so "disable some hooks" has two possible units: a hook **point**
(`post_bump`, `pre_changelog`, `pre_tag`, `post_tag`, `pre_release`, `post_release`) or an
individual **step** inside a point.

## Decision

**Skip by hook point.** `heraut release` and `heraut changelog` gain `--skip-hook <point>` — a
repeatable, comma-separable string-slice flag — and both read the same comma-separated list from
the `HERAUT_SKIP_HOOKS` environment variable when the flag is absent. No config-schema change.

- **Valid values.** `heraut release` accepts all six points; `heraut changelog` accepts the four
  it runs (`post_bump`, `pre_changelog`, `pre_tag`, `post_tag`). Naming `pre_release` or
  `post_release` on `changelog` is a config error — that pipeline never publishes, so the entry
  can only be a mistake — reported separately from an unknown name because the fix differs. Entries
  are trimmed, blanks dropped, duplicates collapsed. Any invalid entry fails the run before the
  config file is read (exit code `Config`), including when it came from the env var.
- **`--skip-hook` with `--no-hooks` is an error.** They contradict each other: `--no-hooks`
  already skips everything. The message says to drop one.
- **The env var is an ambient default, not a request.** An explicit `--skip-hook` *replaces* the
  env var rather than merging with it — the same precedence `--config` has over `HERAUT_FILE`. An
  explicit `--no-hooks` simply wins over the env var and is *not* an error, so a CI-wide
  `HERAUT_SKIP_HOOKS` never turns a one-off `--no-hooks` into a failure. An empty or
  whitespace-only value is treated as unset.
- **Applied by emptying, not flagging.** `internal/app` empties each skipped point's step list
  after translating config to `pipeline.Config`/`pipeline.ChangelogConfig`. Every consumer
  already treats an empty list as "nothing configured" — `shouldRunHooks`, the `[N/total]` step
  counters, dry-run `[dry-run] would run:` lines, and collection of declared `stage` patterns — so
  a skipped point is indistinguishable from an absent one and `internal/pipeline` needed no change.
  A skipped step's `stage` patterns are therefore not staged, which follows from its `run` never
  executing.

## Consequences

- **Additive, zero config-schema change.** `config.go`, `schema.json` and `heraut.sample.yml` are
  untouched; nothing changes for a run that passes neither the flag nor the variable.
- **`--dry-run` respects it.** A skipped point prints no `[dry-run] would run:` lines, the same
  as `--no-hooks` and as a point that was never configured.
- **A shared `HERAUT_SKIP_HOOKS` is strict about `changelog`.** Setting
  `HERAUT_SKIP_HOOKS=post_release` for a whole CI job makes any `heraut changelog` step in that job
  fail with a clear message rather than silently ignore the entry. That keeps typo-catching
  uniform across the flag and the variable; the cost is that a job mixing both commands must scope
  the variable per step. Relaxing the env var alone to ignore release-only points on `changelog`
  is a small, backward-compatible change if this proves painful.
- **A skipped point leaves no trace in the output.** There is no "skipped" status line — consistent
  with `--no-hooks` — so a wrapper script that wants an audit trail must log its own.

## Alternatives considered

- **Skip individual steps by name.** Needs an optional `name:` on every hook step: changes to
  `config.go`, `schema.json`, the sample, validation for name uniqueness, and a decision on
  whether names are unique per point or globally. Real cost for a need nobody has yet stated;
  the point-level flag covers the motivating cases and does not preclude adding step names later
  (the flag's value space would simply grow).
- **Accept point names and step names in one flag.** Everything above plus a namespace-collision
  rule between the two. Rejected with the option above.
- **Make `--skip-hook` + `--no-hooks` silently resolve to `--no-hooks`.** Rejected: passing both
  is almost certainly a wrapper-script bug, and a loud, specific error is cheaper than a run that
  quietly ignores half of what was asked.
- **Merge the env var with the flag (union).** Rejected: with a union there is no way for one
  invocation to run a point the environment skips, which is exactly what an override is for.
- **Use cobra's `MarkFlagsMutuallyExclusive` for the conflict.** Rejected: its message names the
  flag group rather than saying what to do, and it bypasses heraut's `Config` exit code.
