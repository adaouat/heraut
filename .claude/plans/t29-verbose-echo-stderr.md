# T29 — Verbose output echo + stderr on failure

## Context

Two transparency gaps in the exec runner (`internal/adapter/exec/runner.go`), surfaced
during T22 spec reconciliation:

1. **`--verbose` is half-implemented.** It logs `[exec] <cmd> <args>` *before* running a
   command but never echoes what the command produced. A user debugging a release sees
   which commands ran, but not their output.
2. **Failures swallow stderr.** On a non-zero exit the runner returns
   `fmt.Errorf("%s: %w", name, err)` and every caller discards the returned `stderr`
   string. So a failed `gh release create` surfaces only `gh: exit status 1` — the
   actual reason (e.g. `HTTP 404: Not Found`, a bad token, a missing repo) is lost.

Goal: make verbose runs show command output, and make *every* command failure carry the
tool's stderr in the error chain so failures are diagnosable without re-running.

## Current behavior (`runner.go` → `RunEnv`)

- Dry-run: prints `[dry-run] …` and returns early (no exec). Unchanged.
- Verbose: prints `[exec] …` before exec only.
- Output is captured into `stdout`/`stderr` buffers (these are the return values — e.g.
  git-cliff's stdout becomes the changelog, so they must keep being returned as-is).
- Failure: `return stdout, stderr, fmt.Errorf("%s: %w", name, err)`.

## Approach

All changes are in `RunEnv` (and a small unexported helper) — `Run` delegates to it, so
both paths are covered. Dry-run path is untouched.

After `err := cmd.Run()`:

1. **Verbose echo (success and failure).** If `r.Verbose`, write the captured stdout and
   stderr to `r.writer()` (same sink as `[exec]`, i.e. `r.Out` or `os.Stderr`), each
   non-empty line indented by two spaces, via a new helper `echoOutput(w, stdout, stderr)`.
   Skip blocks that are empty. Buffered, not streamed (output is only available after the
   command returns — required because we capture it for the return values).

2. **Stderr in the failure error (always, regardless of verbose).** Trim the captured
   stderr; when non-empty, append it to the wrapped error, keeping `%w` so
   `errors.Is`/`errors.As` still work:
   - with stderr:    `fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr))`
   - without stderr: `fmt.Errorf("%s: %w", name, err)` (unchanged — avoids a dangling `: `)

   Return values (`stdout`, `stderr`) are unchanged; only the error message gains detail.

`strings` is already imported. No interface/signature changes; `port.Runner` and
`testutil.MockRunner` are unaffected (the change lives only in the real exec adapter).

## Files

- `internal/adapter/exec/runner.go` — the two changes above + `echoOutput` helper.
- `internal/adapter/exec/runner_test.go` — new tests (TDD, written first).
- `docs/specs/03-commands.md` — update the `--verbose` row to note it also echoes each
  command's captured output (indented), reversing part of the T22 wording.
- `docs/tasks/roadmap.md` — flip T29 `[ ]`→`[x]` with a completion note.

## Tests (write first — red, then implement)

Follow the existing patterns in `runner_test.go` (`testutil.FakeBin` + `r.Out = &buf`):

1. `TestRunner_Run_verbose_echoesOutput` — fake bin prints to stdout and stderr;
   `Verbose=true`, `Out=&buf`. Assert `buf` contains `[exec]`, the stdout text, and the
   stderr text (indented).
2. `TestRunner_Run_failure_includesStderr` — fake bin `echo "boom detail" >&2; exit 1`.
   Assert `err.Error()` contains `boom detail` (and still contains the binary name).
3. `TestRunner_Run_failure_noStderr_cleanMessage` — fake bin `exit 1` with no stderr.
   Assert `err.Error()` has no trailing `": "` artifact (e.g. ends with `exit status 1`).

Existing `TestRunner_Run_verbose` (asserts `[exec]` line present) and
`TestRunner_Run_failure` (asserts returned `stderr == "oops\n"`) must still pass.

## Verification

- `go test ./internal/adapter/exec/` — new + existing runner tests pass.
- `go test ./...` — no regressions. Pay attention to `internal/pipeline/*_test.go` and the
  platform/generator contract tests; they assert on error substrings (command names) that
  this change preserves, and most use `MockRunner` (which bypasses this wrapping), so they
  should be unaffected.
- `golangci-lint run ./...` clean; `hk check` clean.
- Manual, on the built binary in a git repo:
  - `heraut version current --verbose` → stderr shows `[exec] git tag -l …` followed by
    the indented tag output.
  - Trigger a failure (e.g. `heraut version current --verbose` in a repo with no tags, or
    a release against a misconfigured platform) → the error line carries the tool's stderr
    instead of a bare `exit status 1`.
