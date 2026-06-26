# `heraut commit check --from-latest-tag` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--from-latest-tag` to `heraut commit check` so it auto-resolves the latest tag and checks `<tag>..HEAD` without the user having to look up the tag manually.

**Architecture:** A new `ResolveFromLatestTag` function in the app layer resolves the tag (strategy-aware via `CurrentTag` when config is present, `git describe` fallback when not), returns a rev-range string, and signals "no tags" via a bool. The cmd layer adds the flag, enforces mutual exclusion with the positional arg, and passes the resolved range to the existing `CheckCommitRange` unchanged.

**Tech Stack:** Go, `github.com/adaouat/forge/exec/exectest` (MockRunner), `testutil.RealGitRepo`, `github.com/stretchr/testify`.

## Global Constraints

- TDD: write the failing test before implementation code in every task.
- `go test ./...` must pass after each task.
- `hk check` must pass before each commit (run `hk fix` if it fails, never `--no-verify`).
- All errors wrapped with `fmt.Errorf("…: %w", err)` — `%w` is mandatory.
- No `os.Exit` below `cmd/` — errors propagate up via `return`.
- `internal/cmd/` never imports from `internal/app/` sub-packages directly (use `app.*`).
- Conventional commit message, scope matches the changed package.
- `docs/tasks/roadmap.md` task entry: add T121, flip to `[x]` with completion note in the final commit of Task 2.

---

## File Map

| File | Change |
|---|---|
| `internal/app/commit_check.go` | Add `ResolveFromLatestTag` |
| `internal/app/commit_check_test.go` | Add `TestResolveFromLatestTag` table-driven test |
| `internal/cmd/commit.go` | Add `--from-latest-tag` flag + mutual exclusion + resolver call in `newCommitCheckCmd` |
| `internal/cmd/commit_test.go` | Add 3 new test functions |
| `docs/tasks/roadmap.md` | Add T121 entry, mark `[x]` with completion note |

---

## Task 1: App layer — `ResolveFromLatestTag`

**Files:**
- Modify: `internal/app/commit_check.go`
- Test: `internal/app/commit_check_test.go`

**Interfaces:**
- Consumes: `CurrentTag(runner port.Runner, cfg *config.Config, env string) (string, error)` (already in `internal/app/current.go`)
- Produces: `ResolveFromLatestTag(runner port.Runner, cfg *config.Config, env string) (revRange string, noTags bool, err error)`
  - `("v1.2.3..HEAD", false, nil)` on success
  - `("", true, nil)` when no tags exist
  - `("", false, err)` on unexpected git failure

---

- [ ] **Step 1: Write the failing test**

Add to `internal/app/commit_check_test.go` (after the existing `TestCheckCommitRange_GitLogError_ReturnsWrappedError` function):

```go
func TestResolveFromLatestTag(t *testing.T) {
	semverCfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver"},
	}

	tests := []struct {
		name        string
		cfg         *config.Config
		env         string
		queueStdout string
		queueStderr string
		queueErr    error
		wantRange   string
		wantNoTags  bool
		wantErr     bool
	}{
		{
			name:        "cfg present, tag found",
			cfg:         semverCfg,
			queueStdout: "v1.2.3\nv1.2.2\n",
			wantRange:   "v1.2.3..HEAD",
		},
		{
			name:       "cfg present, no tags",
			cfg:        semverCfg,
			queueStdout: "", // git tag -l returns empty → CurrentTag returns "no tags found"
			wantNoTags: true,
		},
		{
			name:        "cfg nil, tag found via git describe",
			cfg:         nil,
			queueStdout: "v2.0.0\n",
			wantRange:   "v2.0.0..HEAD",
		},
		{
			name:        "cfg nil, no tags (git describe: No names found)",
			cfg:         nil,
			queueStderr: "fatal: No names found, cannot describe anything.",
			queueErr:    errors.New("exit status 128"),
			wantNoTags:  true,
		},
		{
			name:        "cfg nil, git describe unexpected error",
			cfg:         nil,
			queueStderr: "fatal: not a git repository",
			queueErr:    errors.New("exit status 128"),
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.queueStdout, tc.queueStderr, tc.queueErr)

			gotRange, gotNoTags, err := app.ResolveFromLatestTag(mr, tc.cfg, tc.env)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRange, gotRange)
			assert.Equal(t, tc.wantNoTags, gotNoTags)
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/app/... -run TestResolveFromLatestTag -v
```

Expected: `FAIL` — `app.ResolveFromLatestTag undefined`.

- [ ] **Step 3: Implement `ResolveFromLatestTag` in `internal/app/commit_check.go`**

Add this function after `CheckCommitRange` in `internal/app/commit_check.go`. The import block already has `fmt`, `strings`, `port`, `config` — add no new imports beyond what's already there:

```go
// ResolveFromLatestTag returns a rev-range string of the form "<tag>..HEAD".
// With cfg it delegates to CurrentTag (strategy-aware); without cfg it falls
// back to git describe --tags --abbrev=0.
// Returns ("", true, nil) when no tags exist — the caller should warn and check
// full history. Returns ("", false, err) on unexpected git failures.
func ResolveFromLatestTag(runner port.Runner, cfg *config.Config, env string) (string, bool, error) {
	if cfg != nil {
		tag, err := CurrentTag(runner, cfg, env)
		if err != nil {
			if strings.Contains(err.Error(), "no tags found") {
				return "", true, nil
			}
			return "", false, fmt.Errorf("resolving latest tag: %w", err)
		}
		return tag + "..HEAD", false, nil
	}

	stdout, stderr, err := runner.Run("git", "describe", "--tags", "--abbrev=0")
	if err != nil {
		if strings.Contains(stderr, "No names found") || strings.Contains(stderr, "No tags can describe") {
			return "", true, nil
		}
		return "", false, fmt.Errorf("resolving latest tag via git describe: %w", err)
	}
	return strings.TrimSpace(stdout) + "..HEAD", false, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/app/... -run TestResolveFromLatestTag -v
```

Expected: all 5 subtests `PASS`.

- [ ] **Step 5: Run the full app test suite**

```bash
go test ./internal/app/...
```

Expected: all tests pass, no regressions.

- [ ] **Step 6: Commit**

```bash
hk fix
git add internal/app/commit_check.go internal/app/commit_check_test.go
git commit -m "feat(app): add ResolveFromLatestTag for commit check"
```

---

## Task 2: Cmd layer — `--from-latest-tag` flag + roadmap

**Files:**
- Modify: `internal/cmd/commit.go`
- Test: `internal/cmd/commit_test.go`
- Modify: `docs/tasks/roadmap.md`

**Interfaces:**
- Consumes: `app.ResolveFromLatestTag(runner port.Runner, cfg *config.Config, env string) (string, bool, error)` (Task 1)
- Consumes: `app.CheckCommitRange(runner port.Runner, cfg *config.Config, revRange string) ([]app.CommitCheckResult, error)` (unchanged)

---

- [ ] **Step 1: Write the failing tests**

Add to `internal/cmd/commit_test.go` (after `TestCommitCreate_NonTTYErrors`):

```go
func TestCommitCheck_FromLatestTagAndRevRange_MutuallyExclusive(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	_, err := executeRoot("commit", "check", "v1.0.0..HEAD", "--from-latest-tag", "--config", missingCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use both")
}

func TestCommitCheck_FromLatestTag_NoTags_WarnsAndChecksFullHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	t.Chdir(dir)

	// Real git repo with one conventional commit, no tag.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "feat: initial")

	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	out, err := executeRoot("commit", "check", "--from-latest-tag", "--config", missingCfg)
	require.NoError(t, err)
	assert.Contains(t, out, "no tags found")
}

func TestCommitCheck_FromLatestTag_HappyPath_ChecksOnlyCommitsAfterTag(t *testing.T) {
	testutil.RealGitRepo(t, "v0.1.0")

	// Add a conventional commit after the tag — this should be checked.
	addCmd := exec.Command("git", "commit", "--allow-empty", "-m", "feat: post-release feature")
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	out, err := executeRoot("commit", "check", "--from-latest-tag", "--config", missingCfg)
	require.NoError(t, err)
	assert.Contains(t, out, "0 of 1 commits invalid")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test ./internal/cmd/... -run "TestCommitCheck_FromLatestTag|TestCommitCheck_FromLatestTagAndRevRange" -v
```

Expected: `FAIL` — `unknown flag: --from-latest-tag` (or similar).

- [ ] **Step 3: Wire the flag and logic in `internal/cmd/commit.go`**

Replace `newCommitCheckCmd` with the following (changes from current: add `fromLatestTag` var, flag declaration, mutual exclusion block, and resolution block before the `CheckCommitRange` call):

```go
func newCommitCheckCmd() *cobra.Command {
	var fromLatestTag bool
	cmd := &cobra.Command{
		Use:   "check [rev-range]",
		Short: "Validate every commit in a range (or full history) against the conventional-commit grammar",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromLatestTag && len(args) == 1 {
				return exitcode.Wrap(exitcode.Usage, errors.New("cannot use both --from-latest-tag and a rev-range argument"))
			}

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

			if fromLatestTag {
				env, _ := cmd.Flags().GetString("env")
				resolved, noTags, err := app.ResolveFromLatestTag(runner, cfg, env)
				if err != nil {
					return exitcode.Wrap(exitcode.Usage, err)
				}
				if noTags {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Warn(cmd.OutOrStdout(), "no tags found — checking full history"))
				} else {
					revRange = resolved
				}
			}

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
	cmd.Flags().BoolVar(&fromLatestTag, "from-latest-tag", false, "check commits since the latest tag (mutually exclusive with rev-range)")
	return cmd
}
```

Note: `ui` is already imported in `internal/cmd/commit.go` (used in `printCommitCheckResults`). Verify the import block includes `"github.com/adaouat/heraut/internal/ui"` — if not, add it.

- [ ] **Step 4: Run the tests to verify they pass**

```bash
go test ./internal/cmd/... -run "TestCommitCheck_FromLatestTag|TestCommitCheck_FromLatestTagAndRevRange" -v
```

Expected: all 3 tests `PASS`.

- [ ] **Step 5: Run the full cmd test suite**

```bash
go test ./internal/cmd/...
```

Expected: all tests pass, no regressions.

- [ ] **Step 6: Add T121 to roadmap**

Open `docs/tasks/roadmap.md`. Find the `#### [x] T120` section (around line 5145) and insert the following block immediately after it, before the `## Risks and mitigations` section:

```markdown
#### `[x]` T121: `heraut commit check --from-latest-tag`

`cog check --from-latest-tag` equivalent. Adds `--from-latest-tag` bool flag to
`newCommitCheckCmd`. New `app.ResolveFromLatestTag(runner, cfg, env)` resolves the
latest tag: strategy-aware via `CurrentTag` when cfg is present; `git describe --tags
--abbrev=0` fallback when cfg is nil. No-tags condition returns `("", true, nil)` — cmd
layer warns and falls back to full history. Mutual exclusion with the positional rev-range
arg enforced in the cmd layer (error: "cannot use both --from-latest-tag and a rev-range
argument"). `CheckCommitRange` unchanged.

**Completion note:** Implemented in 2 commits (app layer, then cmd layer). Key decision:
string-match on `"no tags found"` to detect the no-tag sentinel from `CurrentTag` — same
pattern already used in `current_test.go:105,194`. No new sentinel error exported; the
match is internal to `app` package. `git describe` no-tag detection checks stderr for
`"No names found"` (git 2.x) or `"No tags can describe"`.

**Scope:** S. **Dependencies:** T119 (`heraut commit check` base command).
```

- [ ] **Step 7: Commit**

```bash
hk fix
git add internal/cmd/commit.go internal/cmd/commit_test.go docs/tasks/roadmap.md
git commit -m "feat(cmd): add --from-latest-tag to heraut commit check"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Covered by |
|---|---|
| `--from-latest-tag` flag on `heraut commit check` | Task 2 Step 3 |
| Mutual exclusion with positional arg → error | Task 2 Step 3 (guard), Task 2 Step 1 (test) |
| No config → `git describe --tags --abbrev=0` | Task 1 Step 3 |
| No tags → warn + full history | Task 1 Step 3 (return `noTags=true`), Task 2 Step 3 (warn path), Task 2 Step 1 (test) |
| Strategy-aware (via `CurrentTag`) when cfg present | Task 1 Step 3 |
| `--env` flows through to `CurrentTag` | Task 2 Step 3 (`cmd.Flags().GetString("env")`) |
| `CheckCommitRange` unchanged | Confirmed — not modified in either task |
| Roadmap T121 entry | Task 2 Step 6 |

No gaps found.

**Placeholder scan:** No TBDs, no "add error handling", all code blocks complete.

**Type consistency:**
- `ResolveFromLatestTag` returns `(string, bool, error)` — defined in Task 1, consumed in Task 2 as `resolved, noTags, err`. Consistent.
- `CheckCommitRange` signature unchanged — `(port.Runner, *config.Config, string)`. Consistent.
