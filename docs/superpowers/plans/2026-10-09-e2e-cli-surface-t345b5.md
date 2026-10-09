# E2E CLI surface (T345b5) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cover the remaining Lane A CLI surface through the real binary: `check config` (valid, invalid value, unknown key, malformed YAML, missing file), config discovery and `~` expansion for `--config` and `HERAUT_FILE` (regression for `4d17b19`), `check runtime` (binary, token, git identity, no-config degrade), bare `check` exit-code precedence, `commit verify` and `commit check`.

**Architecture:** Harness gains `Home()`, `BinDir()` and `RunStdin`. Table-driven scenarios use the existing runner; tests that need PATH control, home files or stdin are standalone. No production change.

**Tech Stack:** Go 1.27, testify, the e2e harness.

**Spec:** `docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md` (Lane A "CLI surface"); behaviour source `docs/specs/03-commands.md` § `heraut check`, § `heraut commit verify`/`check`, `docs/specs/01-overview.md` § Exit codes, `docs/specs/02-configuration.md` § config discovery.

## Global Constraints

- TDD: failing test first, see it fail; characterising tests are proven able to fail by a deliberate mutation of one expectation, restored from a backup copy (never `sed 0,/x/` on macOS).
- Never `--no-verify`; fix lint with `hk fix`. Conventional commits, subject ≤ 72 characters (count it), trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, never `Claude-Session:`. Commits on `main`; do not push.
- `e2e/` imports no heraut `internal/` package. Error text is matched with `normalize` (lower-cased, whitespace-collapsed). Pin only documented exit codes: config 2, runtime 3, usage 1 (Spec 01 and Spec 03).
- No network. `check runtime` runs only fake `gh`; the real `gh`/`glab` must never be reachable from these tests: tests that care about binary presence set `PATH` explicitly (a directory holding only a `git` symlink, plus the fake directory when wanted).
- Every expected value below was produced by the real binary on 2026-10-09. If a test fails, investigate before touching the expectation.

## Review Focus

1. `~` in `--config` and `HERAUT_FILE` must expand to the home directory (the `4d17b19` regression), and precedence must be `--config` > `HERAUT_FILE` > `.config/heraut.yml` > `.heraut.yml`.
2. `check` with both a broken config and a failing runtime must exit 2 (config wins); with only a failing runtime, 3.
3. `check runtime` must fail on each of: gh missing, token unset, git identity unset, each as its own message.
4. A merge or `fixup!` commit message must be skipped by `commit verify` (exit 0, no output).
5. `commit check` must scan every commit (an invalid commit does not stop the scan) and report the count.

---

### Task 1: Harness `Home`, `BinDir`, `RunStdin`

**Files:**
- Modify: `e2e/harness/run.go`
- Modify: `e2e/harness/fakecli.go`
- Modify: `e2e/harness/repo.go`
- Modify: `e2e/harness/harness_test.go`

**Interfaces:**
- Consumes: `Repo`, `runEnv` (T345b4c).
- Produces: `(*Repo).Home() string`, `(*Repo).WriteHomeFile(rel, content string)`, `(*Repo).BinDir() string` (the fake directory, "" if none), `(*Repo).RunStdin(bin string, env []string, stdin string, args ...string) Result` (`Run` becomes `RunStdin` with empty stdin).

- [ ] **Step 1: Write the failing test**

Append to `e2e/harness/harness_test.go`:

```go
func TestRepo_HomeBinDirAndStdin(t *testing.T) {
	r := NewRepo(t)

	r.WriteHomeFile("cfg/x.yml", "hello\n")
	res := r.Run("/bin/sh", nil, "-c", "cat ~/cfg/x.yml")
	assert.Equal(t, "hello\n", res.Stdout, "the child sees the same HOME")
	assert.Equal(t, r.Home()+"/cfg/x.yml", strings.TrimSpace(r.Run("/bin/sh", nil, "-c", "echo $HOME/cfg/x.yml").Stdout))

	assert.Empty(t, r.BinDir())
	r.FakeCLI("gh")
	assert.NotEmpty(t, r.BinDir())
	assert.FileExists(t, r.BinDir()+"/gh")

	res = r.RunStdin("/bin/cat", nil, "from stdin\n")
	assert.Equal(t, "from stdin\n", res.Stdout)
}
```

Run: `go test ./e2e/harness -run TestRepo_HomeBinDirAndStdin`
Expected: FAIL to compile, `r.WriteHomeFile undefined`.

- [ ] **Step 2: Implement**

In `e2e/harness/repo.go` append:

```go
// Home is the HOME directory the repository's commands run with.
func (r *Repo) Home() string { return r.home }

// WriteHomeFile writes content to a path relative to HOME, creating directories.
func (r *Repo) WriteHomeFile(rel, content string) {
	r.t.Helper()
	path := filepath.Join(r.home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatalf("creating directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatalf("writing %s: %v", rel, err)
	}
}
```

In `e2e/harness/fakecli.go` append:

```go
// BinDir is the directory holding the fake CLIs and their call log ("" until FakeCLI is used).
func (r *Repo) BinDir() string { return r.binDir }
```

In `e2e/harness/run.go` rename the existing `Run` body into `RunStdin` with a `stdin string` parameter (`if stdin != "" { cmd.Stdin = strings.NewReader(stdin) }`, add the `strings` import) and make `Run` a one-line wrapper:

```go
func (r *Repo) Run(bin string, env []string, args ...string) Result {
	r.t.Helper()
	return r.RunStdin(bin, env, "", args...)
}
```

Run: `go test -count=1 ./e2e/harness`
Expected: PASS.

- [ ] **Step 3: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e/harness
git commit -F - <<'EOF'
test(e2e): expose HOME, the fake dir and stdin in the harness

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: CLI surface scenarios

**Files:**
- Modify: `e2e/scenario_test.go` (add `exitUsage = 1`)
- Create: `e2e/cli_surface_test.go`

**Interfaces:**
- Consumes: Task 1 helpers; `scenario`/`runScenarios`/`commit`/`tag`, `normalize`, `exitOK/exitConfig/exitRuntime`, `harness.Binary/NewRepo/FakeCLI/Run/RunStdin/WriteFile/Git`.
- Produces: `exitUsage`, `gitOnlyPath(t) string`.

- [ ] **Step 1: Add `exitUsage` and write the first test (red: the file does not exist)**

In `e2e/scenario_test.go`'s const block add `exitUsage = 1` (keep gofmt alignment via `hk fix`).

Create `e2e/cli_surface_test.go`:

```go
package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestCheckConfig(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "a valid config prints its path and source", config: semverCfg(""),
			args: []string{"check", "config"}, wantOut: ".heraut.yml  (from .heraut.yml)\n✓ config: ok"},
		{name: "an invalid value names the key and the valid choices", config: "version: \"1\"\nversioning:\n  strategy: semvr\n",
			args:     []string{"check", "config"},
			wantExit: exitConfig, wantText: []string{`versioning.strategy: "semvr" is not a valid strategy`, "valid strategies: semver, calver, semver-per-env, calver-per-env", "1 error(s)"}},
		{name: "an unknown key reports its line", config: "version: \"1\"\nversioning:\n  strategy: semver\n  bogus_key: 1\n",
			args:     []string{"check", "config"},
			wantExit: exitConfig, wantText: []string{"line 4: field bogus_key not found in type config.versioning"}},
		{name: "malformed YAML is a config error", config: "version: [unclosed",
			args:     []string{"check", "config"},
			wantExit: exitConfig, wantText: []string{"yaml: line 1"}},
	})
}
```

Run: `go test -count=1 ./e2e -run TestCheckConfig`
Expected: the first row may fail on the exact `wantOut` (stdout layout). If so, print the real stdout, and set `wantOut` to it exactly after confirming it is only whitespace/layout (the binary prints two lines; the first line is indented by two spaces and the runner compares the *trimmed* stdout). If the layout cannot be matched stably, drop `wantOut` for that row by giving it `wantText: []string{"✓ config: ok", "(from .heraut.yml)"}` and `wantOut: ""` is NOT allowed (the runner compares it): use a standalone test instead. Do not weaken the other rows.

- [ ] **Step 2: Add the remaining tests**

Append to `e2e/cli_surface_test.go`:

```go
func TestConfigDiscoveryAndTildeExpansion(t *testing.T) {
	bin := harness.Binary(t)
	valid := "version: \"1\"\nversioning:\n  strategy: semver\n"
	check := func(t *testing.T, repo *harness.Repo, env []string, args ...string) (string, int) {
		t.Helper()
		res := repo.Run(bin, env, append([]string{"check", "config"}, args...)...)
		return normalize(res.Stdout + " " + res.Stderr), res.ExitCode
	}

	t.Run(".config/heraut.yml is found when .heraut.yml is absent", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteFile(".config/heraut.yml", valid)

		out, code := check(t, repo, nil)

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, "(from .config/heraut.yml)")
	})
	t.Run("HERAUT_FILE beats the default file", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteConfig(valid)
		repo.WriteFile("other.yml", valid)

		out, code := check(t, repo, []string{"HERAUT_FILE=other.yml"})

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, "other.yml (from heraut_file)")
	})
	t.Run("--config beats HERAUT_FILE", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteFile("a.yml", valid)
		repo.WriteFile("b.yml", valid)

		out, code := check(t, repo, []string{"HERAUT_FILE=b.yml"}, "--config", "a.yml")

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, "a.yml (from --config)")
	})
	t.Run("~ in --config expands to the home directory", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteHomeFile("cfg/x.yml", valid)

		out, code := check(t, repo, nil, "--config", "~/cfg/x.yml")

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, normalize(repo.Home()+"/cfg/x.yml (from --config)"))
	})
	t.Run("~ in HERAUT_FILE expands to the home directory", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteHomeFile("cfg/x.yml", valid)

		out, code := check(t, repo, []string{"HERAUT_FILE=~/cfg/x.yml"})

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, normalize(repo.Home()+"/cfg/x.yml (from heraut_file)"))
	})
	t.Run("a missing file after expansion names the expanded path", func(t *testing.T) {
		repo := harness.NewRepo(t)

		out, code := check(t, repo, []string{"HERAUT_FILE=~/nope.yml"})

		require.Equal(t, exitConfig, code, out)
		assert.Contains(t, out, normalize(repo.Home()+"/nope.yml"))
	})
	t.Run("no config anywhere is a config error for check config", func(t *testing.T) {
		repo := harness.NewRepo(t)

		out, code := check(t, repo, nil)

		require.Equal(t, exitConfig, code, out)
		assert.Contains(t, out, "reading config")
	})
}

// gitOnlyPath is a PATH holding nothing but git, so the real gh/glab can never be found.
func gitOnlyPath(t *testing.T) string {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	require.NoError(t, os.Symlink(gitPath, filepath.Join(dir, "git")))
	return dir
}

const githubCfg = `version: "1"
versioning:
  strategy: semver
forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN
release:
  targets:
    - forge: github
`

func TestCheckRuntime(t *testing.T) {
	bin := harness.Binary(t)
	newRepo := func(t *testing.T, cfg string) *harness.Repo {
		repo := harness.NewRepo(t)
		if cfg != "" {
			repo.WriteConfig(cfg)
		}
		repo.Commit("feat: one")
		return repo
	}
	run := func(t *testing.T, repo *harness.Repo, path string, env []string, args ...string) (string, int) {
		t.Helper()
		res := repo.Run(bin, append([]string{"PATH=" + path}, env...), args...)
		return normalize(res.Stdout + " " + res.Stderr), res.ExitCode
	}

	t.Run("everything present passes", func(t *testing.T) {
		repo := newRepo(t, githubCfg)
		repo.FakeCLI("gh")

		out, code := run(t, repo, repo.BinDir()+string(os.PathListSeparator)+gitOnlyPath(t), []string{"GH_TOKEN=x"}, "check", "runtime")

		require.Equal(t, exitOK, code, out)
		assert.Contains(t, out, "all checks passed")
	})
	t.Run("a missing gh is its own failure", func(t *testing.T) {
		repo := newRepo(t, githubCfg)

		out, code := run(t, repo, gitOnlyPath(t), []string{"GH_TOKEN=x"}, "check", "runtime")

		require.Equal(t, exitRuntime, code, out)
		assert.Contains(t, out, "gh not found")
		assert.NotContains(t, out, "environment variable gh_token is not set")
	})
	t.Run("a missing token is its own failure", func(t *testing.T) {
		repo := newRepo(t, githubCfg)
		repo.FakeCLI("gh")

		out, code := run(t, repo, repo.BinDir()+string(os.PathListSeparator)+gitOnlyPath(t), nil, "check", "runtime")

		require.Equal(t, exitRuntime, code, out)
		assert.Contains(t, out, "environment variable gh_token is not set")
		assert.NotContains(t, out, "gh not found")
	})
	t.Run("an unset git identity fails", func(t *testing.T) {
		repo := newRepo(t, githubCfg)
		repo.FakeCLI("gh")
		repo.Git("config", "--unset", "user.name")

		out, code := run(t, repo, repo.BinDir()+string(os.PathListSeparator)+gitOnlyPath(t), []string{"GH_TOKEN=x"}, "check", "runtime")

		require.Equal(t, exitRuntime, code, out)
		assert.Contains(t, out, "git user.name — not configured")
	})
	t.Run("with no config every tool is required", func(t *testing.T) {
		repo := newRepo(t, "")

		out, code := run(t, repo, gitOnlyPath(t), nil, "check", "runtime")

		require.Equal(t, exitRuntime, code, out)
		assert.Contains(t, out, "no config found at .heraut.yml — all tools checked as required")
		assert.Contains(t, out, "gh not found")
	})
}

func TestCheck_ConfigErrorsOutrankRuntimeFailures(t *testing.T) {
	bin := harness.Binary(t)
	newRepo := func(t *testing.T, cfg string) *harness.Repo {
		repo := harness.NewRepo(t)
		repo.WriteConfig(cfg)
		repo.Commit("feat: one")
		return repo
	}
	path := "PATH=" + gitOnlyPath(t)

	t.Run("only the runtime fails: exit 3", func(t *testing.T) {
		repo := newRepo(t, githubCfg)

		res := repo.Run(bin, []string{path}, "check")

		require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout), "config: ok")
	})
	t.Run("config and runtime fail: the config code wins", func(t *testing.T) {
		repo := newRepo(t, "version: \"1\"\nversioning:\n  strategy: nope\nforges:\n  - name: github\n    platform: github\n    repository: acme/widget\n    token_env: GH_TOKEN\n")

		res := repo.Run(bin, []string{path}, "check")

		require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		out := normalize(res.Stdout + " " + res.Stderr)
		assert.Contains(t, out, `"nope" is not a valid strategy`)
		assert.Contains(t, out, "gh not found", "the runtime section still ran and reported")
	})
}

func TestCommitVerify(t *testing.T) {
	bin := harness.Binary(t)
	cfg := semverCfg("")
	runScenarios(t, bin, nil, []scenario{
		{name: "a valid message prints its parts", config: cfg,
			args:     []string{"commit", "verify", "feat(cmd): add x"},
			wantText: []string{"commit message is valid", "type: feat", "scope: cmd", "breaking: false", "description: add x"},
			wantOut:  "✓ commit message is valid\n  type:        feat\n  scope:       cmd\n  breaking:    false\n  description: add x"},
		{name: "a bang marks the commit breaking", config: cfg,
			args:     []string{"commit", "verify", "feat!: big"},
			wantText: []string{"breaking: true"}, wantOut: "✓ commit message is valid\n  type:        feat\n  breaking:    true\n  description: big"},
		{name: "a malformed header is a usage error", config: cfg,
			args:     []string{"commit", "verify", "bad message"},
			wantExit: exitUsage, wantText: []string{`invalid conventional commit header "bad message"`, `expected "type(scope)!: description"`}},
		{name: "a type outside the allow-list is a usage error", config: cfg,
			args:     []string{"commit", "verify", "wip: x"},
			wantExit: exitUsage, wantText: []string{`commit type "wip" is not allowed`}},
		{name: "a merge commit is skipped", config: cfg,
			args: []string{"commit", "verify", "Merge branch 'x' into main"}, wantOut: ""},
		{name: "a fixup commit is skipped", config: cfg,
			args: []string{"commit", "verify", "fixup! feat: x"}, wantOut: ""},
		{name: "neither a message nor --file is a usage error", config: cfg,
			args:     []string{"commit", "verify"},
			wantExit: exitUsage, wantText: []string{"provide a commit message as an argument or via --file"}},
		{name: "both a message and --file is a usage error", config: cfg,
			args:     []string{"commit", "verify", "feat: x", "--file", "msg"},
			wantExit: exitUsage, wantText: []string{"not both"}},
		{name: "a missing --file is a usage error", config: cfg,
			args:     []string{"commit", "verify", "--file", "nope.txt"},
			wantExit: exitUsage, wantText: []string{"reading commit message from nope.txt"}},
	})

	t.Run("--file reads a message from disk", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteConfig(cfg)
		repo.WriteFile("msg.txt", "fix: from file\n")

		res := runOK(t, repo, bin, "commit", "verify", "--file", "msg.txt")

		assert.Contains(t, normalize(res.Stdout), "description: from file")
	})
	t.Run("--file - reads a message from stdin", func(t *testing.T) {
		repo := harness.NewRepo(t)
		repo.WriteConfig(cfg)

		res := repo.RunStdin(bin, nil, "fix: via stdin\n", "commit", "verify", "--file", "-")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout), "description: via stdin")
	})
}

func TestCommitCheck(t *testing.T) {
	bin := harness.Binary(t)
	history := []step{
		commit("feat: good one"), commit("oops not conventional"), tag("v0.1.0"),
		commit("fix: good two"), commit("wip: bad type"),
	}
	runScenarios(t, bin, nil, []scenario{
		{name: "the full history is scanned and every invalid commit reported", config: semverCfg(""), history: history,
			args:     []string{"commit", "check"},
			wantExit: exitUsage, wantText: []string{"oops not conventional", "wip: bad type", "2 of 4 commits invalid"}},
		{name: "--from-latest-tag scans only what is after the tag", config: semverCfg(""), history: history,
			args:     []string{"commit", "check", "--from-latest-tag"},
			wantExit: exitUsage, wantText: []string{"wip: bad type", "1 of 2 commits invalid"}, notText: []string{"oops not conventional"}},
		{name: "a clean range passes", config: semverCfg(""), history: history,
			args:     []string{"commit", "check", "HEAD~2..HEAD~1"},
			wantText: []string{"all commits follow conventional commits"}, wantOut: "✓ all commits follow conventional commits!\n  1 commits analysed"},
		{name: "a range and --from-latest-tag are mutually exclusive", config: semverCfg(""), history: history,
			args:     []string{"commit", "check", "--from-latest-tag", "HEAD"},
			wantExit: exitUsage, wantText: []string{"cannot use both --from-latest-tag and a rev-range argument"}},
		{name: "an unresolvable range is reported as such", config: semverCfg(""), history: history,
			args:     []string{"commit", "check", "nonsense..range"},
			wantExit: exitUsage, wantText: []string{`listing commits in range "nonsense..range"`}},
	})
}
```

Run: `go test -count=1 ./e2e -run 'TestCheck|TestConfigDiscovery|TestCommit'`
Expected: PASS after those two fixes. If a row fails, copy the output and stop: investigate before touching the expectation. Then prove the tests can fail: back up the file, exact-replace `"2 of 4 commits invalid"` with `"3 of 4 commits invalid"` and `"gh not found")\n\t\tassert.NotContains(t, out, "environment variable gh_token is not set")` with `"gh found")\n\t\tassert.NotContains(t, out, "environment variable gh_token is not set")`, run `-run 'TestCommitCheck|TestCheckRuntime'`, confirm both fail showing the real text, restore from the backup.

- [ ] **Step 3: Full suite, lint, commit**

Run: `go test -count=1 ./... && hk fix && hk check`

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): cover check, config discovery, ~ expansion and commit linting

check config (valid, invalid value, unknown key, malformed YAML),
discovery precedence and ~ expansion for --config and HERAUT_FILE,
check runtime failures one by one, config-over-runtime exit precedence,
and commit verify/check including stdin, skips and range errors.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Close T345b5 and T345b

**Files:**
- Modify: `docs/tasks/roadmap.md`

- [ ] **Step 1: Flip and annotate**

Change `- \`[ ]\` **T345b5**:` to `- \`[x]\` **T345b5**:` and the parent heading line `#### T345b: remaining Lane A scenarios (split into five parts)` to `#### \`[x]\` T345b: remaining Lane A scenarios (split into five parts)`. Add an indented completion note under T345b5 with real facts: counts (`go test -count=1 -v ./e2e -run 'TestCheck|TestConfigDiscovery|TestCommit' 2>&1 | grep -c -- '--- PASS'`) and the new e2e total, the harness additions (`Home`, `WriteHomeFile`, `BinDir`, `RunStdin`), the `~`-expansion regression coverage, and any deviation or defect. Update the Phase 60 status row: "T345a, T345b, T357, T358 done; T345c, d open; T359, T360 open". `grep -n 'T345b5' docs/tasks/roadmap.md` must show the `[x]` bullet with no placeholder text.

- [ ] **Step 2: Verify and commit**

Run: `go test -count=1 ./... && hk check`

```bash
git add docs/tasks/roadmap.md
git commit -F - <<'EOF'
docs(roadmap): close T345b5 and T345b

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

## Self-review

**Spec coverage** (Lane A "CLI surface": `check config`/`check runtime` exit codes, `~` expansion in `--config` and `HERAUT_FILE`, `commit verify`, unknown config key reports a line number): all four, plus config discovery precedence, `commit check`, and the exit-code precedence of bare `check`.

**Type consistency:** `Home`, `WriteHomeFile`, `BinDir`, `RunStdin` (Task 1) are used in Task 2; `exitUsage` and `gitOnlyPath` are defined in Task 2 before use; `semverCfg`, `runOK`, `normalize`, `step`/`commit`/`tag` come from earlier parts.

**Placeholders:** the Task 3 note is written from measured counts at execution time.
