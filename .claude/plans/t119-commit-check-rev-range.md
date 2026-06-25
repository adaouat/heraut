# T119: `heraut commit check` — Rev-Range Conventional-Commit Validation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `heraut commit check [rev-range]` — validate every (non-merge, non-fixup) commit in a range, or the full history when no range is given, against the same conventional-commit grammar and type allow-list `heraut commit verify` already enforces, reporting every invalid commit found in one run.

**Architecture:** A new `internal/app/commit_check.go` enumerates the range via `git log` (NUL/SOH-delimited, mirroring the parsing pattern already in `internal/versioning/semver/resolver.go`) and calls the existing `VerifyCommit` per commit — zero new validation logic. `internal/cmd/commit.go` gains a `check` subcommand that renders the results (failures + summary by default, every commit under `--verbose`) and maps any invalid commit to `exitcode.Usage`.

**Tech Stack:** Go 1.26, `stretchr/testify`, `github.com/adaouat/forge/exec/exectest.MockRunner` (contract tests), cobra. No new dependencies.

## Global Constraints

- Per [ADR-0030](../../docs/adr/0030-commit-check-rev-range-validation.md): `VerifyCommit` (T116, `internal/app/commit.go`) is reused unchanged — this plan adds a caller, never new validation policy.
- TDD: failing test before implementation, every task (`testing.md`, `.claude/rules/claude.md`).
- Every `if err != nil` propagating an existing error wraps with `%w` (`coding.md`).
- Never `os.Exit` below `cmd/heraut/`. `internal/cmd/` returns errors; exit-code mapping happens via `internal/exitcode.Wrap`.
- Layer rules (`coding.md`): `internal/app/` may import `internal/{port,config,conventionalcommit}`; `internal/cmd/` calls into `internal/app` only, never `internal/conventionalcommit` or git mechanics directly.
- No new config field, no new exit code, no new global flag — `commit_lint.types`, `exitcode.Usage`, and the existing root-level `--verbose` flag are reused as-is.
- No JSON/machine-readable output — plain text only, consistent with every existing heraut command.
- Table-driven tests preferred (`testing.md`). No filesystem outside `t.TempDir()`; no real git execution needed anywhere in this plan's tests — contract tests via `MockRunner` fully cover the git-log mechanics, and the cobra-level test exercises only the non-git error path (a non-repo directory), per `testing.md`'s "reach for real-CLI sparingly" guidance.
- Conventional-commit subject lines per `workflow.md`'s type table. Co-author trailer required on every commit a Claude session authors.
- Reference docs: [ADR-0030](../../docs/adr/0030-commit-check-rev-range-validation.md), [the approved spec](../../docs/superpowers/specs/2026-06-25-commit-check-design.md), [ADR-0027](../../docs/adr/0027-builtin-conventional-commit-checker.md).

---

### Task 1: `internal/app/commit_check.go` — `CheckCommitRange`

**Files:**
- Create: `internal/app/commit_check.go`
- Create: `internal/app/commit_check_test.go`
- Modify: `docs/tasks/roadmap.md` (add the `[ ]` T119 task stub — see Step 1)

**Interfaces:**
- Consumes: `app.VerifyCommit(cfg *config.Config, message string) error` (existing, `internal/app/commit.go`, unchanged); `port.Runner.Run(name string, args ...string) (string, string, error)` (existing).
- Produces: `app.CommitCheckResult{SHA, Subject string; Err error}` and `app.CheckCommitRange(runner port.Runner, cfg *config.Config, revRange string) ([]CommitCheckResult, error)` — both consumed by Task 2's `internal/cmd/commit.go`.

- [ ] **Step 1: Add the roadmap task stub**

Add this entry to `docs/tasks/roadmap.md` immediately after T118's entry and before the
"Future ideas (not yet scoped)" paragraph (the paragraph itself is trimmed in this same
edit — see below):

```markdown
#### `[ ]` T119: `heraut commit check` — rev-range conventional-commit validation

Per [ADR-0030](../adr/0030-commit-check-rev-range-validation.md). T116 shipped
single-message `heraut commit verify`; this is the `cog check` equivalent — validating an
entire commit range (or full history) for use as a CI gate on a PR branch.

**Implementation:**

1. `internal/app/commit_check.go`: `CommitCheckResult{SHA, Subject string; Err error}` and
   `CheckCommitRange(runner port.Runner, cfg *config.Config, revRange string)
   ([]CommitCheckResult, error)`. Enumerates via `git log [rev-range]
   --format=%h%x01%s%x01%B%x00`, extending `internal/versioning/semver/resolver.go`'s
   existing NUL-delimited parsing pattern with one extra `\x01`-delimited field. Calls
   `VerifyCommit` unchanged per commit — no new validation logic, no new merge/fixup
   handling (already covered by `VerifyCommit`'s existing skip).
2. `internal/cmd/commit.go`: new `check` subcommand, `heraut commit check [rev-range]`.
   Default output prints only failing commits + a summary count; `--verbose` (existing
   root flag) prints every commit. `exitcode.Usage` when any commit is invalid or when
   `CheckCommitRange` itself errors (bad range, git not found) — same classification
   ADR-0027 used for single-message `verify`.
3. `docs/specs/03-commands.md`: document the new command.

**Tests:** contract tests for `CheckCommitRange` (range arg shape, multi-commit parsing
including footers/blank lines, merge/fixup skip-through via `VerifyCommit`'s existing
behavior, collect-all-not-fail-fast, configured type allowlist, git-log error path); a
white-box test for the rendering helper (failures-only vs. verbose); a cobra-level test for
the non-git-repo error path.

**Files:** `internal/app/commit_check.go` (new), `internal/app/commit_check_test.go` (new),
`internal/cmd/commit.go`, `internal/cmd/commit_test.go`, `internal/cmd/commit_internal_test.go`
(new), `docs/specs/03-commands.md`.
**Scope:** S. **Dependencies:** T116.
```

Then delete the now-redundant `heraut commit check` half of the "Future ideas" paragraph
that follows T118's closing note, leaving only the still-unscoped wizard idea. Change:

```markdown
**Future ideas (not yet scoped):** two follow-ons surfaced while designing T116/T117 —
`heraut commit check <rev-range>` (the `cog check` equivalent: validate a whole commit
range/history, for CI on a PR branch, vs. T116's single-message `verify`) and an
interactive commit wizard (e.g. `heraut commit create`, akin to
[meteor](https://github.com/stefanlogue/meteor), reusing T116's `conventionalcommit`
package and `commit_lint` config). See
[ADR-0027](../adr/0027-builtin-conventional-commit-checker.md)'s "Related future work".
Neither is a task yet — each needs its own brainstorming session before a T-id is assigned.
```

to:

```markdown
**Future ideas (not yet scoped):** an interactive commit wizard (e.g. `heraut commit
create`, akin to [meteor](https://github.com/stefanlogue/meteor)), reusing T116's
`conventionalcommit` package and `commit_lint` config — see
[ADR-0027](../adr/0027-builtin-conventional-commit-checker.md)'s "Related future work". Not
a task yet — needs its own brainstorming session before a T-id is assigned.
```

Commit this alone first (it is pure doc setup, no test cycle of its own):

```bash
git add docs/tasks/roadmap.md
git commit -m "docs(roadmap): add T119 task stub for heraut commit check"
```

- [ ] **Step 2: Write the failing tests — range/parsing/collect-all/allowlist table**

Create `internal/app/commit_check_test.go`:

```go
package app_test

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCommitRange_NoRange_OmitsRangeArg(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("abc1234\x01feat: a\x01feat: a\x00", "", nil)

	_, err := app.CheckCommitRange(mr, nil, "")
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"log", "--format=%h%x01%s%x01%B%x00"}, mr.Calls[0].Args)
}

func TestCheckCommitRange_WithRange_AppendsRangeArg(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("abc1234\x01feat: a\x01feat: a\x00", "", nil)

	_, err := app.CheckCommitRange(mr, nil, "main..HEAD")
	require.NoError(t, err)

	assert.Equal(t, []string{"log", "main..HEAD", "--format=%h%x01%s%x01%B%x00"}, mr.Calls[0].Args)
}

func TestCheckCommitRange_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		cfg        *config.Config
		wantCount  int
		wantValid  []bool // parallel to wantCount, true = Err nil
	}{
		{
			name:      "all valid, default types",
			stdout:    "aaa1111\x01feat: a\x01feat: a\x00bbb2222\x01fix: b\x01fix: b\x00",
			wantCount: 2,
			wantValid: []bool{true, true},
		},
		{
			name:      "invalid commit does not stop the scan (collect-all)",
			stdout:    "aaa1111\x01not conventional\x01not conventional\x00bbb2222\x01fix: b\x01fix: b\x00",
			wantCount: 2,
			wantValid: []bool{false, true},
		},
		{
			name:      "merge and fixup commits are skipped (valid)",
			stdout:    "aaa1111\x01Merge branch 'main'\x01Merge branch 'main' into feature/x\x00bbb2222\x01fixup! feat: a\x01fixup! feat: a\x00",
			wantCount: 2,
			wantValid: []bool{true, true},
		},
		{
			name:   "configured type allowlist rejects out-of-list type",
			stdout: "aaa1111\x01docs: update\x01docs: update\x00",
			cfg: &config.Config{
				CommitLint: &config.CommitLint{Types: []string{"feat", "fix"}},
			},
			wantCount: 1,
			wantValid: []bool{false},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.stdout, "", nil)

			results, err := app.CheckCommitRange(mr, tc.cfg, "")
			require.NoError(t, err)
			require.Len(t, results, tc.wantCount)
			for i, wantValid := range tc.wantValid {
				if wantValid {
					assert.NoError(t, results[i].Err, "result %d", i)
				} else {
					assert.Error(t, results[i].Err, "result %d", i)
				}
			}
		})
	}
}

func TestCheckCommitRange_BodyWithBreakingChangeFooterAndBlankLines_ParsesIntact(t *testing.T) {
	mr := exectest.NewMockRunner()
	body := "feat: a\n\nSome body text.\n\nBREAKING CHANGE: this changes the API\x00"
	mr.QueueResponse("aaa1111\x01feat: a\x01"+body, "", nil)

	results, err := app.CheckCommitRange(mr, nil, "")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.NoError(t, results[0].Err)
	assert.Equal(t, "aaa1111", results[0].SHA)
	assert.Equal(t, "feat: a", results[0].Subject)
}

func TestCheckCommitRange_GitLogError_ReturnsWrappedError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "fatal: bad range", errors.New("exit status 128"))

	results, err := app.CheckCommitRange(mr, nil, "bad..range")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad..range")
	assert.Nil(t, results)
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/app/... -run TestCheckCommitRange -v`
Expected: FAIL — `app.CheckCommitRange` and `app.CommitCheckResult` do not exist yet (compile error).

- [ ] **Step 4: Implement `CheckCommitRange`**

Create `internal/app/commit_check.go`:

```go
package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// CommitCheckResult records the outcome of validating one commit in a range.
type CommitCheckResult struct {
	SHA     string // git's abbreviated hash (%h) — length varies with repo size/core.abbrev
	Subject string // first line of the message
	Err     error  // nil = valid (or skipped merge/fixup)
}

// CheckCommitRange validates every commit in revRange (or the full history reachable from
// HEAD when revRange is "") against the same grammar and type allow-list VerifyCommit
// applies to a single message. Every commit is evaluated — an invalid commit does not
// stop the scan. Merge and fixup commits are skipped via VerifyCommit's existing,
// unconditional skip — no separate handling needed here.
func CheckCommitRange(runner port.Runner, cfg *config.Config, revRange string) ([]CommitCheckResult, error) {
	args := []string{"log"}
	if revRange != "" {
		args = append(args, revRange)
	}
	args = append(args, "--format=%h%x01%s%x01%B%x00")

	stdout, _, err := runner.Run("git", args...)
	if err != nil {
		return nil, fmt.Errorf("listing commits in range %q: %w", revRange, err)
	}

	var results []CommitCheckResult
	for _, record := range strings.Split(stdout, "\x00") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		fields := strings.SplitN(record, "\x01", 3)
		if len(fields) != 3 {
			continue
		}
		results = append(results, CommitCheckResult{
			SHA:     fields[0],
			Subject: fields[1],
			Err:     VerifyCommit(cfg, fields[2]),
		})
	}
	return results, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/app/... -run TestCheckCommitRange -v`
Expected: PASS — all 5 test functions green, including the 4 subtests inside
`TestCheckCommitRange_TableDriven`.

- [ ] **Step 6: Run the full `internal/app` package test suite**

Run: `go test ./internal/app/... -v`
Expected: PASS — no regression in existing `commit_test.go`, `pipeline_test.go`, `check_test.go`.

- [ ] **Step 7: Commit**

```bash
git add internal/app/commit_check.go internal/app/commit_check_test.go
git commit -m "feat(app): add CheckCommitRange for rev-range commit validation (ADR-0030)"
```

---

### Task 2: `heraut commit check` cobra command

**Files:**
- Modify: `internal/cmd/commit.go`
- Modify: `internal/cmd/commit_test.go`
- Create: `internal/cmd/commit_internal_test.go`

**Interfaces:**
- Consumes: `app.CheckCommitRange(runner port.Runner, cfg *config.Config, revRange string) ([]app.CommitCheckResult, error)` and `app.CommitCheckResult{SHA, Subject string; Err error}` (Task 1, unchanged).
- Produces: `newCommitCheckCmd() *cobra.Command` and `printCommitCheckResults(results []app.CommitCheckResult, verbose bool, out io.Writer) int` (both unexported, package `cmd` — no external consumers).

- [ ] **Step 1: Write the failing white-box test for the renderer**

Create `internal/cmd/commit_internal_test.go`:

```go
package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/stretchr/testify/assert"
)

func TestPrintCommitCheckResults_AllValid_NoVerbose_OnlySummary(t *testing.T) {
	var buf bytes.Buffer
	results := []app.CommitCheckResult{
		{SHA: "aaa1111", Subject: "feat: a", Err: nil},
		{SHA: "bbb2222", Subject: "fix: b", Err: nil},
	}

	failed := printCommitCheckResults(results, false, &buf)

	assert.Equal(t, 0, failed)
	out := buf.String()
	assert.NotContains(t, out, "aaa1111")
	assert.NotContains(t, out, "bbb2222")
	assert.Contains(t, out, "0 of 2 commits invalid")
}

func TestPrintCommitCheckResults_SomeInvalid_NoVerbose_OnlyFailuresShown(t *testing.T) {
	var buf bytes.Buffer
	results := []app.CommitCheckResult{
		{SHA: "aaa1111", Subject: "not conventional", Err: errors.New("validating commit message: boom")},
		{SHA: "bbb2222", Subject: "fix: b", Err: nil},
	}

	failed := printCommitCheckResults(results, false, &buf)

	assert.Equal(t, 1, failed)
	out := buf.String()
	assert.Contains(t, out, "aaa1111")
	assert.Contains(t, out, "not conventional")
	assert.NotContains(t, out, "bbb2222")
	assert.Contains(t, out, "1 of 2 commits invalid")
}

func TestPrintCommitCheckResults_Verbose_ShowsEveryCommit(t *testing.T) {
	var buf bytes.Buffer
	results := []app.CommitCheckResult{
		{SHA: "aaa1111", Subject: "not conventional", Err: errors.New("boom")},
		{SHA: "bbb2222", Subject: "fix: b", Err: nil},
	}

	failed := printCommitCheckResults(results, true, &buf)

	assert.Equal(t, 1, failed)
	out := buf.String()
	assert.Contains(t, out, "aaa1111")
	assert.Contains(t, out, "bbb2222")
	assert.Contains(t, out, "1 of 2 commits invalid")
}
```

- [ ] **Step 2: Write the failing black-box cobra-wiring tests**

Extend `internal/cmd/commit_test.go`. Change the existing structural test:

```go
func TestCommitCmd_Exists(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var commitCmd, verifyCmd bool
	for _, c := range root.Commands() {
		if c.Use == "commit" {
			commitCmd = true
			for _, sc := range c.Commands() {
				if strings.HasPrefix(sc.Use, "verify") {
					verifyCmd = true
				}
			}
		}
	}
	assert.True(t, commitCmd, "commit command missing")
	assert.True(t, verifyCmd, "commit verify missing")
}
```

to:

```go
func TestCommitCmd_Exists(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var commitCmd, verifyCmd, checkCmd bool
	for _, c := range root.Commands() {
		if c.Use == "commit" {
			commitCmd = true
			for _, sc := range c.Commands() {
				if strings.HasPrefix(sc.Use, "verify") {
					verifyCmd = true
				}
				if strings.HasPrefix(sc.Use, "check") {
					checkCmd = true
				}
			}
		}
	}
	assert.True(t, commitCmd, "commit command missing")
	assert.True(t, verifyCmd, "commit verify missing")
	assert.True(t, checkCmd, "commit check missing")
}
```

Then add two new test functions at the end of the file:

```go
func TestCommitCheck_NonGitDirectory_ErrorsWithUsageExit(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	missingCfg := filepath.Join(dir, ".heraut.yml")

	_, err := executeRoot("commit", "check", "--config", missingCfg)
	require.Error(t, err)
}

func TestCommitCheck_AcceptsOptionalRevRangeArg(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	missingCfg := filepath.Join(dir, ".heraut.yml")

	_, err := executeRoot("commit", "check", "main..HEAD", "--config", missingCfg)
	require.Error(t, err) // still errors — dir is not a git repo — but the arg parses
}
```

- [ ] **Step 3: Run all new tests to verify they fail**

Run: `go test ./internal/cmd/... -run "TestPrintCommitCheckResults|TestCommitCmd_Exists|TestCommitCheck" -v`
Expected: FAIL — `printCommitCheckResults` doesn't exist (compile error in
`commit_internal_test.go`); `TestCommitCmd_Exists`'s new `checkCmd` assertion fails (no
`check` subcommand registered yet); the two new `TestCommitCheck_*` tests fail to compile
for the same reason once the package fails to build. All of this is one compile-time
failure surface — expected at this stage, since nothing in `commit.go` has changed yet.

- [ ] **Step 4: Add `newCommitCheckCmd` and `printCommitCheckResults`**

In `internal/cmd/commit.go`, add the import (alongside the existing ones):

```go
	execadapter "github.com/adaouat/forge/exec"
	"io"

	"github.com/adaouat/heraut/internal/ui"
```

Register the new subcommand in `NewCommitCmd`:

```go
func NewCommitCmd() *cobra.Command {
	commitCmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit message tooling",
	}
	commitCmd.AddCommand(newCommitVerifyCmd())
	commitCmd.AddCommand(newCommitCheckCmd())
	return commitCmd
}
```

Add the new subcommand constructor and renderer (place after `newCommitVerifyCmd`):

```go
func newCommitCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [rev-range]",
		Short: "Validate every commit in a range (or full history) against the conventional-commit grammar",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var revRange string
			if len(args) == 1 {
				revRange = args[0]
			}

			cfgPath, _ := cmd.Flags().GetString("config")
			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
				}
				cfg = nil
			}
			if cfg != nil {
				if errs := config.Validate(cfg); len(errs) > 0 {
					printConfigErrors(errs, cmd.OutOrStdout())
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", len(errs)))
				}
			}

			verbose, _ := cmd.Flags().GetBool("verbose")
			runner := execadapter.New(false, verbose)
			results, err := app.CheckCommitRange(runner, cfg, revRange)
			if err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}

			failed := printCommitCheckResults(results, verbose, cmd.OutOrStdout())
			if failed > 0 {
				return exitcode.Wrap(exitcode.Usage, fmt.Errorf("%d of %d commits invalid", failed, len(results)))
			}
			return nil
		},
	}
	return cmd
}

// printCommitCheckResults renders results to out: failing commits always print
// (SHA, subject, reason); verbose additionally prints every valid/skipped commit.
// Returns the number of invalid commits.
func printCommitCheckResults(results []app.CommitCheckResult, verbose bool, out io.Writer) int {
	var failed int
	for _, r := range results {
		switch {
		case r.Err != nil:
			failed++
			_, _ = fmt.Fprintln(out, ui.Err(out, fmt.Sprintf("%s  %s — %s", r.SHA, r.Subject, r.Err)))
		case verbose:
			_, _ = fmt.Fprintln(out, ui.Success(out, fmt.Sprintf("%s  %s", r.SHA, r.Subject)))
		}
	}
	_, _ = fmt.Fprintf(out, "%d of %d commits invalid\n", failed, len(results))
	return failed
}
```

- [ ] **Step 4: Run the renderer tests to verify they pass**

Run: `go test ./internal/cmd/... -run TestPrintCommitCheckResults -v`
Expected: PASS

- [ ] **Step 5: Run all new tests to verify they pass**

Run: `go test ./internal/cmd/... -run "TestPrintCommitCheckResults|TestCommitCmd_Exists|TestCommitCheck" -v`
Expected: PASS — all green, including `TestCommitCmd_Exists`'s new `checkCmd` assertion and
both new `TestCommitCheck_*` tests (each still returns an error, since the temp dir is not
a git repo — but the error now comes from `git log` failing inside `CheckCommitRange`, not
from a missing subcommand or a flag-parsing failure).

- [ ] **Step 6: Run the full `internal/cmd` package test suite**

Run: `go test ./internal/cmd/... -v`
Expected: PASS — no regression in any other `internal/cmd` test file.

- [ ] **Step 7: Commit**

```bash
git add internal/cmd/commit.go internal/cmd/commit_test.go internal/cmd/commit_internal_test.go
git commit -m "feat(cmd): add heraut commit check (ADR-0030)"
```

---

### Task 3: Docs + final gate + roadmap closure

**Files:**
- Modify: `docs/specs/03-commands.md`
- Modify: `docs/tasks/roadmap.md`

**Interfaces:** None — documentation and roadmap closure only.

- [ ] **Step 1: Document the new command in `docs/specs/03-commands.md`**

Insert a new section immediately after the existing `## \`heraut commit verify\`` section
(right before `## \`heraut check\``):

```markdown
## `heraut commit check`

Validate every commit in a range — or the full history reachable from `HEAD` when no
range is given — against the same grammar and type allow-list `heraut commit verify`
checks for a single message (see [ADR-0030](../adr/0030-commit-check-rev-range-validation.md)).

```
heraut commit check [rev-range]
```

`rev-range` is passed straight through to `git log` — `A..B`, `A...B`, a single ref, or
omitted entirely for every commit reachable from `HEAD`. No heraut-specific range syntax;
git's own range syntax and its own errors on a malformed range are reused as-is.

Every commit in the range is evaluated — an invalid commit does not stop the scan. Merge
and fixup commits are skipped (the same unconditional skip `heraut commit verify` already
applies). By default only invalid commits are printed (short SHA, subject, reason) plus a
summary line (`N of M commits invalid`); the global `--verbose` flag additionally prints
every commit, valid ones included. Exits with the Usage code (1) if any commit is invalid,
or if the range itself cannot be resolved (e.g. malformed `rev-range`, git not on `PATH`)
— same classification as `heraut commit verify`'s single-message case.
```

- [ ] **Step 2: Run the full gate**

Run: `go build ./... && go test ./... && mise run lint:check`
Expected: clean build, all tests pass, all linters pass. If this fails, STOP — fix before
proceeding; do not flip the roadmap checkbox on a red gate.

- [ ] **Step 3: Flip the roadmap checkbox and add the closing note**

In `docs/tasks/roadmap.md`, change:

```markdown
#### `[ ]` T119: `heraut commit check` — rev-range conventional-commit validation
```

to:

```markdown
#### `[x]` T119: `heraut commit check` — rev-range conventional-commit validation
```

Then append a closing paragraph after the task's `**Scope:** S. **Dependencies:** T116.`
line, naming the actual commits from Tasks 1-2 and any real deviations encountered during
implementation (do not write a generic "implemented exactly as planned" note if anything
differed — name the actual deviation, the way T116/T117/T118's closing notes do).

- [ ] **Step 4: Commit**

```bash
git add docs/specs/03-commands.md docs/tasks/roadmap.md
git commit -m "docs: document heraut commit check and close T119 (ADR-0030)"
```
