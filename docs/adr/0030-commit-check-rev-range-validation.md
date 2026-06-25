# ADR-0030: `heraut commit check` — Rev-Range Conventional-Commit Validation

- **Status**: Accepted
- **Date**: 2026-06-25
- **Deciders**: bchatard

---

## Context

[ADR-0027](0027-builtin-conventional-commit-checker.md) (T116) shipped
`heraut commit verify`, validating a single commit message — the `commit-msg` hook use
case. It explicitly deferred the `cog check` equivalent as unscoped future work:

> **`heraut commit check <rev-range>`** — the `cog check` equivalent: validate an entire
> commit range/history (e.g. all commits on a PR branch) rather than a single message, for
> use in CI. ... range checking is a distinct command with its own design questions (how
> to enumerate the range, how to report multiple failures, merge-commit handling across a
> range vs. a single message).

This ADR resolves those three questions and specs the command. Full design rationale and
the rejected alternatives considered live in the brainstorming spec:
[`docs/superpowers/specs/2026-06-25-commit-check-design.md`](../superpowers/specs/2026-06-25-commit-check-design.md).

## Decision

### Default range semantics

`heraut commit check [rev-range]` takes zero or one positional argument, passed straight
through to `git log <rev-range>` exactly as written — `A..B`, `A...B`, a single ref, or
omitted entirely. No heraut-specific range parsing; git's own syntax and its own errors on
a malformed range are reused as-is. Omitting `rev-range` means no range argument is passed
to `git log` at all — git's own default of "every commit reachable from `HEAD`", not a
heraut-invented default or an attempt at default-branch detection.

### Merge-commit handling — solved by reuse, not new logic

`internal/app/commit.go`'s existing `VerifyCommit` (T116) already skips merge and fixup
commits unconditionally. The range checker calls `VerifyCommit` unchanged, per commit, so
merge-commit handling across a range is identical to the single-message case for free.

### Multi-failure reporting

The checker evaluates every commit in the range — it never stops at the first failure. A
git-log enumeration failure (bad range syntax, git not found) is a usage error that aborts
before any per-commit results exist; a per-commit *validation* failure is data in the
result set, not an aborting error. Default output prints only invalid commits (short SHA,
subject, reason) plus a summary count; the existing root-level `--verbose` flag (already
documented in [`coding.md`](../../.claude/rules/coding.md)) additionally prints every
commit, valid ones included.

### Architecture

New `internal/app/commit_check.go`, peer to `internal/app/commit.go`:

```go
type CommitCheckResult struct {
	SHA     string // git's abbreviated hash (%h) — length varies with repo size/core.abbrev
	Subject string // first line of the message
	Err     error  // nil = valid (or skipped merge/fixup)
}

func CheckCommitRange(runner port.Runner, cfg *config.Config, revRange string) ([]CommitCheckResult, error)
```

Enumeration extends the NUL-delimited multi-line-safe parsing pattern
`internal/versioning/semver/resolver.go` already uses, adding one field for the commit
hash: `git log --format=%h%x01%s%x01%B%x00 [rev-range]`. Records split on `\x00`, fields
within a record split on `\x01` (hash, subject, full body). Per record, `CheckCommitRange`
calls `VerifyCommit(cfg, body)` unchanged and appends a `CommitCheckResult` — no new
validation logic, no duplicated merge/fixup or type-allowlist checks.

`internal/cmd/commit.go` gains `newCommitCheckCmd()` (`heraut commit check [rev-range]`),
registered alongside the existing verify subcommand, reusing its exact config-loading
boilerplate. Exit code is `exitcode.Usage` when any commit is invalid — the same
classification ADR-0027 chose for single-message `verify` ("bad input, not a config or
runtime failure"); no new exit code is introduced. This classification deliberately
diverges from `exitcode.Runtime` — which `wrapRunErr` uses for other git-subprocess
failures — because a git-log failure in this context is most likely a malformed
`rev-range` argument, not an environment or runtime failure, following the same
bad-argument reasoning ADR-0027 applied to `commit verify`.

### Rejected alternatives

- **Refactor `VerifyCommit` to return a richer structured result instead of `error`.**
  Would touch T116's already-shipped, already-reviewed code for a benefit the range
  checker doesn't need — its existing `error` return is sufficient. YAGNI.
- **Implement git-log enumeration directly in `internal/cmd/commit.go`, skipping the `app`
  layer.** Breaks the cmd→app layering ADR-0027 deliberately established for this command
  family, and makes the enumeration logic untestable without spinning up cobra commands.

## Consequences

- `heraut commit verify` and `heraut commit check` share one validation implementation
  (`VerifyCommit`) with zero duplicated grammar/allow-list logic — a grammar or
  allow-list change updates both call sites automatically.
- `docs/specs/03-commands.md` documents the new `heraut commit check` command.
- No new config field, no new exit code, no new global flag — `commit_lint.types`,
  `exitcode.Usage`, and the existing `--verbose` flag are all reused as-is.
- No machine-readable (JSON) output format exists for this command, consistent with every
  other heraut command today; revisit only if a concrete CI-integration need demonstrates
  it (YAGNI).

## Related future work

[ADR-0027](0027-builtin-conventional-commit-checker.md)'s other deferred idea — an
interactive commit wizard (`heraut commit create`) — remains unscoped and is unaffected by
this ADR.
