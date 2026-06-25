# `heraut commit check` — Range/History Conventional-Commit Validation

**Date:** 2026-06-25
**Status:** Approved

## Problem

[ADR-0027](../../adr/0027-builtin-conventional-commit-checker.md) (T116) shipped
`heraut commit verify`, which validates a single commit message — the `commit-msg` hook
use case. It explicitly deferred the `cog check` equivalent — validating a whole commit
range/history, e.g. for a CI gate on a PR branch — as unscoped future work, flagging three
open design questions: how to enumerate the range, how to report multiple failures, and
how merge-commit handling differs across a range vs. a single message.

This design resolves those three questions and specs `heraut commit check`.

## Goals

- Validate every (non-merge, non-fixup) commit in a given range or the full history against
  the same conventional-commit grammar and type allow-list `heraut commit verify` already
  enforces, with zero duplicated validation logic.
- Usable both as a CI gate (PR branch range) and as an ad-hoc local history audit (no range
  given → full history from `HEAD`).
- Report every invalid commit found in one run, not just the first — a contributor with
  three bad commits should not need three CI runs to find them all.

## Non-goals

- No new validation rules beyond what `heraut commit verify` (T116) already enforces —
  this command only adds a new *caller* of the existing grammar/allow-list check, never new
  policy.
- No JSON or other machine-readable output format. No existing heraut command has one
  (confirmed: no `--json` flag anywhere in `internal/cmd/`); plain text matches the
  project's established convention and nothing has demonstrated a need for more.
- No default-branch detection. "No range given" means full history reachable from `HEAD` —
  the same thing `git log` with no arguments already means — not an attempt to infer a PR's
  base branch.

## Decision

### Default range semantics

`heraut commit check [rev-range]` takes zero or one positional argument, passed straight
through to `git log <rev-range>` exactly as written — `A..B`, `A...B`, a single ref, or
omitted entirely. No heraut-specific range parsing or validation; git's own range syntax
and its own error messages on a malformed range are reused as-is.

When `rev-range` is omitted, no range argument is passed to `git log` at all, which is
exactly equivalent to "every commit reachable from `HEAD`" — git's own default, not a
heraut-invented default.

### Merge-commit handling

`heraut commit verify`'s existing `VerifyCommit` already skips merge and fixup commits
unconditionally (`internal/app/commit.go`, T116). The range checker calls `VerifyCommit`
unchanged, per commit — so merge-commit handling across a range is identical to the
single-message case for free. No new skip logic is needed.

### Multi-failure reporting

The range checker collects a result for every commit in the range — never stops early. A
single git-log enumeration failure (bad range syntax, git not found) is a usage error and
aborts before any per-commit results exist; a per-commit *validation* failure is data, not
an aborting error.

Default output prints only the invalid commits (short SHA, subject, reason) plus a summary
line (`N of M commits invalid`). The existing root-level `--verbose` flag (already
documented in `coding.md`) additionally prints a line for every commit in the range,
valid or skipped ones included — reusing an existing global flag rather than inventing a
new one.

### Architecture

New `internal/app/commit_check.go`, peer to the existing `internal/app/commit.go`:

```go
package app

type CommitCheckResult struct {
	SHA     string // git's abbreviated hash (%h) — length varies with repo size/core.abbrev
	Subject string // first line of the message
	Err     error  // nil = valid (or skipped merge/fixup)
}

func CheckCommitRange(runner port.Runner, cfg *config.Config, revRange string) ([]CommitCheckResult, error)
```

`CheckCommitRange` enumerates commits with:

```go
args := []string{"log", "--format=%h%x01%s%x01%B%x00"}
if revRange != "" {
	args = append(args, revRange)
}
stdout, _, err := runner.Run("git", args...)
```

— extending the NUL-delimited, multi-field parsing pattern `internal/versioning/semver/resolver.go`
already uses for safe parsing of multi-line commit bodies (records split on `\x00`, fields
within a record split on `\x01`: short hash, subject, full body in that order). A git-log
failure returns `(nil, fmt.Errorf("listing commits in range %q: %w", revRange, err))` —
note this still surfaces if `revRange` is `""`, with a clear `""`-quoted message rather
than a confusing blank.

For each parsed record, `CheckCommitRange` calls `VerifyCommit(cfg, body)` unchanged and
appends `CommitCheckResult{SHA: hash, Subject: subject, Err: err}` — no new validation
logic, no duplicated merge/fixup or type-allowlist checks.

### CLI surface

`internal/cmd/commit.go` gains `newCommitCheckCmd()`, registered alongside the existing
verify subcommand in `NewCommitCmd()`:

```
heraut commit check [rev-range]
```

Config loading mirrors `newCommitVerifyCmd()`'s existing boilerplate exactly (resolve
path, load, validate if present, `cfg = nil` on `os.ErrNotExist`) — no new config-loading
pattern.

Rendering: default prints only failing `CommitCheckResult` entries plus the summary line;
`--verbose` prints every entry. Exit code is `exitcode.Usage` when any commit is invalid —
the same classification ADR-0027 chose for single-message `verify` ("bad input, not a
config or runtime failure, so no new exit code is introduced"); this design introduces no
new exit code either.

### Rejected alternatives

- **Refactor `VerifyCommit` to return a richer structured result instead of `error`.**
  Rejected: would touch T116's already-shipped, already-reviewed code for a benefit the
  range checker doesn't need — its existing `error` return is sufficient to build
  `CommitCheckResult.Err`. YAGNI.
- **Implement git-log enumeration directly in `internal/cmd/commit.go`, skipping the `app`
  layer.** Rejected: breaks the cmd→app layering ADR-0027 deliberately established for
  this command family (`coding.md`'s layer table), and makes the enumeration logic
  untestable without spinning up cobra commands.

## Testing

- `internal/app/commit_check_test.go` — contract tests via `exectest.MockRunner`:
  - Exact `git log` args, with and without a `rev-range` argument.
  - Multi-commit parsing, including a body containing a `BREAKING CHANGE:` footer and
    embedded blank lines, confirming the `\x00`/`\x01` delimiters survive intact.
  - Merge and fixup commits in the range produce a `nil`-`Err` result (skipped), not a
    validation failure.
  - A type-allowlist violation partway through the range still allows later commits in the
    same range to be evaluated (collect-all behavior, not fail-fast).
  - Bad range syntax / git-not-found surfaces as the function's `error` return, not a
    `CommitCheckResult`.
- `internal/cmd/commit_test.go` — cobra-level test for the new `check` subcommand: flag
  wiring, failures-only-by-default output, `--verbose` showing every commit, and the
  `exitcode.Usage` exit code when invalid commits are found.
- No new test coverage needed for `VerifyCommit` itself — T116 already covers its
  grammar/allow-list behavior; this feature only adds a caller.
