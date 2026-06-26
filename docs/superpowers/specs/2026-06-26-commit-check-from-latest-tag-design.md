# `heraut commit check --from-latest-tag` — Auto-resolve latest tag as range start

**Date:** 2026-06-26
**Status:** Approved

## Problem

`heraut commit check [rev-range]` (T119) requires the caller to supply an explicit git
rev-range (e.g. `v1.2.3..HEAD`). This is fine for CI pipelines that already know the base
ref, but inconvenient for the common local workflow of "check everything since my last
release" — the user must first look up the latest tag manually, which is strategy- and
env-aware in heraut.

`cog check --from-latest-tag` solves this ergonomically. heraut lacks an equivalent.

## Goals

- Add `--from-latest-tag` to `heraut commit check` so it resolves the latest tag
  automatically and checks `<tag>..HEAD`.
- Respect the active versioning strategy and `--env` flag when resolving the tag
  (strategy-aware via `app.CurrentTag`).
- Work without a config file via a `git describe` fallback.
- Handle missing tags gracefully (warn + check full history).

## Non-goals

- No new validation rules — the same grammar and allow-list `heraut commit verify` enforces.
- No changes to `CheckCommitRange` — it stays a plain `revRange string` consumer.
- No `--from-latest-tag` on `heraut commit verify` (single-message command, not applicable).

## CLI surface

```
heraut commit check [rev-range] [--from-latest-tag]
```

| Invocation | Behaviour |
|---|---|
| `heraut commit check` | check full history (unchanged) |
| `heraut commit check v1.2.3..HEAD` | check explicit range (unchanged) |
| `heraut commit check --from-latest-tag` | resolve latest tag → check `<tag>..HEAD` |
| `heraut commit check v1.2.3..HEAD --from-latest-tag` | **error** — mutually exclusive |

**Mutual exclusion error message:**
`"cannot use both --from-latest-tag and a rev-range argument"`
Wrapped with `exitcode.Usage`.

**No-tags behaviour:**
Print `ui.Warn("no tags found — checking full history")`, then proceed with `revRange = ""`
(same as calling `heraut commit check` with no argument).

**`--env` interaction:**
`--env` is already a root persistent flag. `ResolveFromLatestTag` accepts `env string` and
passes it through to `CurrentTag` for per-env strategies. No additional wiring needed in
the cmd layer.

## App layer

New function in `internal/app/commit_check.go`:

```go
// ResolveFromLatestTag returns a rev-range string of the form "<tag>..HEAD".
// If cfg is non-nil it uses CurrentTag (strategy-aware); otherwise it falls
// back to git describe --tags --abbrev=0.
// Returns ("", true, nil) when no tags exist — the caller should warn and
// check full history.
// Returns ("", false, err) on unexpected git failures.
func ResolveFromLatestTag(runner port.Runner, cfg *config.Config, env string) (revRange string, noTags bool, err error)
```

### Resolution paths

**With config (`cfg != nil`):**
1. Call `CurrentTag(runner, cfg, env)`.
2. If the error message contains "no tags found" (the sentinel `CurrentTag` returns): return `("", true, nil)`.
3. On other errors: propagate as `err`.
4. On success: return `("<tag>..HEAD", false, nil)`.

**Without config (`cfg == nil`):**
1. Run `git describe --tags --abbrev=0`.
2. If git exits non-zero and stderr contains "No names found" or "No tags can describe":
   return `("", true, nil)`.
3. On other non-zero exit: propagate as `err`.
4. On success: `tag = strings.TrimSpace(stdout)`, return `("<tag>..HEAD", false, nil)`.

### `CheckCommitRange` — unchanged

`CheckCommitRange(runner, cfg, revRange string)` is not modified. `ResolveFromLatestTag`
is a pure resolver that hands a resolved range string back to the cmd layer, which passes
it through to `CheckCommitRange`.

## Cmd layer

Changes confined to `newCommitCheckCmd()` in `internal/cmd/commit.go`:

1. Declare `var fromLatestTag bool`; wire `--from-latest-tag` flag.
2. **Mutual exclusion guard** (before any git calls):
   ```go
   if fromLatestTag && len(args) == 1 {
       return exitcode.Wrap(exitcode.Usage, errors.New("cannot use both --from-latest-tag and a rev-range argument"))
   }
   ```
3. When `fromLatestTag`:
   - Read `env` from root persistent flag.
   - Call `app.ResolveFromLatestTag(runner, cfg, env)`.
   - `noTags == true` → print `ui.Warn(…)`, leave `revRange = ""`.
   - `err != nil` → return `exitcode.Wrap(exitcode.Usage, err)`.
   - Otherwise → set `revRange = result`.
4. Call `app.CheckCommitRange(runner, cfg, revRange)` unchanged.

The runner is already constructed before this block (for `--verbose`); no additional
runner construction needed.

## Error handling

| Scenario | Behaviour |
|---|---|
| Both `--from-latest-tag` and positional arg | `exitcode.Usage` error |
| No tags (config or no-config path) | `ui.Warn` + fall through to full history |
| `git describe` fails for non-tag reason | `exitcode.Usage` error with wrapped git error |
| `CurrentTag` fails for non-tag reason | `exitcode.Usage` error with wrapped error |

## Testing

### Unit tests — `internal/app/commit_check_test.go`

New table-driven test `TestResolveFromLatestTag` covering:

| Case | Setup | Expected |
|---|---|---|
| cfg present, tag found | MockRunner queues tag stdout | `("v1.2.3..HEAD", false, nil)` |
| cfg present, no tags | MockRunner queues "no tags found" error | `("", true, nil)` |
| cfg nil, tag found | MockRunner queues `git describe` stdout `"v1.2.3\n"` | `("v1.2.3..HEAD", false, nil)` |
| cfg nil, no tags | MockRunner queues non-zero exit + "No names found" stderr | `("", true, nil)` |
| cfg nil, git error | MockRunner queues unexpected non-zero exit | `("", false, err)` |

All via `testutil.MockRunner` — no real git calls.

### Cmd tests — `internal/cmd/commit_test.go`

| Case | What to assert |
|---|---|
| `--from-latest-tag` + positional arg | exits non-zero, error message contains "cannot use both" |
| `--from-latest-tag` no tags | output contains "no tags found", exits zero (no invalid commits) |
| `--from-latest-tag` happy path | `MockRunner` records `git log v1.2.3..HEAD …` as the log call |

## Files touched

| File | Change |
|---|---|
| `internal/app/commit_check.go` | Add `ResolveFromLatestTag` |
| `internal/app/commit_check_test.go` | Add `TestResolveFromLatestTag` |
| `internal/cmd/commit.go` | Add flag + mutual exclusion + resolver call in `newCommitCheckCmd` |
| `internal/cmd/commit_test.go` | Add 3 new cases |
| `docs/tasks/roadmap.md` | New task entry (T121) |

No new files. No changes to interfaces, config schema, or other commands.
