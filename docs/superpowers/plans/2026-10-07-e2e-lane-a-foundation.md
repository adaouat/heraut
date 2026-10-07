# E2E Lane A Foundation (T345a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the hermetic end-to-end lane (Lane A) foundation: ADR-0066 and the testing-rule amendment, a build-tag test clock, a harness that builds and drives the real `heraut` binary against local git repos, and the first scenarios (SemVer resolution, `stay_at_v0`, pre-release lifecycle, overrides, CalVer clock smoke).

**Architecture:** A top-level `e2e/` tree. `e2e/harness` (package `harness`) builds the binary once per build-tag set, creates throwaway git repos and runs the binary with a scrubbed environment. `e2e/*_test.go` (package `e2e_test`) holds table-driven scenarios and runs inside plain `go test ./...`. CalVer needs a controllable clock: `internal/app` gains a `clock()` seam that is `time.Now` in shipped builds and reads `HERAUT_TEST_NOW` only when built with `-tags heraut_testclock`.

**Tech Stack:** Go 1.27, `os/exec`, `testify`, git, mise/hk.

**Spec:** `docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md` (T345a row of its Task breakdown).

## Global Constraints

- TDD: every behavioural change starts with a failing test, run and seen to fail before the implementation (`.claude/rules/testing.md`).
- Never `--no-verify`, never bypass hooks; fix lint through `hk fix` (not `gofmt`/`golangci-lint` directly).
- Commits are conventional, subject ≤ 72 chars, scope = package, e.g. `test(e2e): …`, `refactor(app): …`, `docs(adr): …`. Body explains the *why*.
- Commit trailer: `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`. Never add a `Claude-Session:` line.
- Commits land directly on `main` (pre-v1.0). Do not push.
- No real project paths, usernames or hosts in tests, docs or fixtures; synthetic placeholders only.
- Layer rules (`.claude/rules/coding.md`): `internal/app` may not gain new imports from heraut packages. `e2e/` imports no heraut `internal/` package.
- Comments: none by default; only the non-obvious *why*; never reference tasks or fix history in code comments.
- Scope: T345a only. Do not implement per-env, maintenance-branch, changelog, hook or forge scenarios (T345b-d), and do not touch the `native` generator clock (deferred to T345b, which is the first task that renders `GeneratedAt`).
- Exit codes (verified against the binary): usage/flag/config errors exit **2**, resolution runtime errors exit **3**, promotion guard exit **4**.
- The CLI error panel changes case and wraps lines (observed: `V1.4.0-Beta.1`, `--Pre-Release`, mid-sentence wraps). Scenario assertions on error text must lower-case and whitespace-collapse the output first and use short phrases. The mangling itself is filed as T357 (Task 5), not fixed here.

## Review Focus

Failure modes the spec implies but a happy-path scenario would not exercise:

1. `HERAUT_TEST_NOW` set to garbage in a `heraut_testclock` build must fail loudly naming the variable, not silently fall back to the real clock (Task 2 test `rejects garbage`).
2. The **shipped** binary must ignore `HERAUT_TEST_NOW` entirely (Task 4 test `shipped binary ignores HERAUT_TEST_NOW`).
3. Ambient CI variables (`GITHUB_ACTIONS`, `GITLAB_CI`, `CI_*`) on the developer's/CI machine must not leak into a scenario run and change forge or branch detection (Task 3 test `TestRun_ScrubsAmbientEnvAndReportsExit`).
4. A `go build` failure must surface the compiler output, not a confusing exec error later; a missing `git` must skip, not fail (Task 3 `Binary` and `NewRepo`).
5. A pre-release tag must never be the bump base of a final release, while a build-metadata-only tag counts as the release of its core (ADR-0064) (Task 4 rows `pre-release tag is not the bump base` and `build-metadata tag counts as its core release`).

---

### Task 1: ADR-0066 and the E2E testing layer

**Files:**
- Create: `docs/adr/0066-e2e-test-lanes.md`
- Modify: `docs/adr/README.md` (append index row)
- Modify: `.claude/rules/testing.md` (add an "E2E layers" section after "Four test layers")
- Modify: `docs/specs/06-dx-and-testing.md` (append a short pointer section)

**Interfaces:**
- Consumes: nothing.
- Produces: the build-tag names `heraut_testclock` and `e2e_forge`, the env var `HERAUT_TEST_NOW`, and the `e2e/` directory layout, which later tasks reference.

- [ ] **Step 1: Write the ADR**

Create `docs/adr/0066-e2e-test-lanes.md`:

```markdown
# ADR-0066: End-to-end test lanes — hermetic binary lane and opt-in forge lane

- **Status**: Accepted
- **Date**: 2026-10-07
- **Deciders**: bchatard
- **Design doc**: [`docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md`](../superpowers/specs/2026-10-07-e2e-smoke-suite-design.md)

---

## Context

The four test layers (unit, contract, integration, schema) cannot see two classes of defect: a
forge rejecting what heraut sends (T332's manual smoke run found T335 that way), and wiring bugs
between layers that each pass their own tests (ldflags, flag plumbing, exit-code mapping, real git
history). heraut is now used on many projects, so those escapes cost more.

`.claude/rules/testing.md` forbids network calls and requires determinism, so a forge e2e suite needs
an explicit, narrow exemption rather than an ad-hoc one.

## Decision

Add a fifth test layer, **E2E**, in a top-level `e2e/` tree that drives the **built `heraut` binary**,
split into two lanes:

- **Lane A, hermetic.** Real binary, real `git`, local repositories under `t.TempDir()`, no network.
  It lives inside the existing rules and runs in plain `go test ./...`, so it gates every PR. It covers
  version resolution and local release flows.
- **Lane B, forge sandbox.** Real binary against private GitHub/GitLab sandbox repositories. It is
  behind the `e2e_forge` build tag, runs only from its own workflow (`workflow_dispatch` plus
  nightly, advisory, never on pull requests), and takes coordinates and tokens from CI variables.
  Only this lane is exempt from "no network calls", and only for the sandbox hosts.

CalVer scenarios need a controllable clock. `internal/app` resolves its clock through a `clock()`
seam: `time.Now` in every shipped build, and a clock pinned by `HERAUT_TEST_NOW` (RFC 3339) only when
compiled with `-tags heraut_testclock`. Released binaries never contain the pinned-clock code.

## Alternatives considered

- **`libfaketime` / `LD_PRELOAD`:** unreliable, Go reads the clock through the vDSO and direct
  syscalls, not libc; blocked by SIP on macOS.
- **A production `HERAUT_NOW` variable or `--now` flag:** a user-visible knob that changes release
  versions is a footgun and a new CLI surface for a testing need.
- **Changing the machine clock:** needs root, breaks TLS and git, and cannot run on a developer
  machine.
- **Forge scenarios in `go test ./...`:** violates determinism and would need write tokens on pull
  requests.

## Consequences

- The e2e binary differs from the shipped one by one build-tagged file. Only CalVer scenarios use the
  tagged binary.
- `go test ./...` builds the binary once per tag set (about one second warm).
- Lane B needs sandbox repositories, scoped tokens and a cleanup sweeper; they are specified in the
  design doc and delivered in T345c/T345d.
- A failing Lane A scenario is a regression to fix, never a test to loosen.
```

- [ ] **Step 2: Add the index row**

Append to the table in `docs/adr/README.md` (after the ADR-0065 row):

```markdown
| [0066](0066-e2e-test-lanes.md) | End-to-end test lanes — hermetic binary lane and opt-in forge lane | Accepted |
```

- [ ] **Step 3: Amend the testing rules**

In `.claude/rules/testing.md`, insert this section immediately before `## MockRunner — the contract test workhorse`:

```markdown
## E2E layers (ADR-0066)

A fifth, separate layer drives the **built binary** from `e2e/`:

- **Lane A (hermetic)** — `e2e/*_test.go`, no build tag, runs in `go test ./...`. Real `git` and local
  repos under `t.TempDir()`, no network. Same determinism rules as every other layer.
- **Lane B (forge sandbox)** — `//go:build e2e_forge`, opt-in, never part of `go test ./...` or pull
  requests. The only layer allowed network calls, and only to the configured sandbox repos.

Scenarios assert on stdout, exit code (via `internal/exitcode` values) and repo state. CalVer scenarios
use a binary built with `-tags heraut_testclock` and set `HERAUT_TEST_NOW`; never add a production
clock override. A failing e2e scenario is fixed at the root cause like any other test.
```

- [ ] **Step 4: Add the Spec 06 pointer**

Append to `docs/specs/06-dx-and-testing.md`:

```markdown

## End-to-end tests

Beyond unit, contract, integration and schema tests, `e2e/` drives the built binary: a hermetic
lane that runs with `go test ./...` and an opt-in forge-sandbox lane. See
[ADR-0066](../adr/0066-e2e-test-lanes.md) and the
[design](../superpowers/specs/2026-10-07-e2e-smoke-suite-design.md).
```

- [ ] **Step 5: Lint and commit**

Run: `hk fix && hk check`
Expected: all linters pass (typos included).

```bash
git add docs/adr/0066-e2e-test-lanes.md docs/adr/README.md docs/specs/06-dx-and-testing.md .claude/rules/testing.md
git commit -F - <<'EOF'
docs(adr): add 0066 e2e test lanes and the testing-rule amendment

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Build-tagged test clock seam

**Files:**
- Create: `internal/app/clockenv.go`
- Create: `internal/app/clock.go`
- Create: `internal/app/clock_testclock.go`
- Create: `internal/app/clockenv_internal_test.go`
- Modify: `internal/app/resolver.go` (lines 208 and 214, plus the `time` import)

**Interfaces:**
- Consumes: nothing.
- Produces: unexported `clock() func() time.Time` (default build: `time.Now`; `-tags heraut_testclock`: pinned by `HERAUT_TEST_NOW`) and `clockFromEnv(getenv func(string) string) (func() time.Time, error)`. Task 4's CalVer scenarios rely on the behaviour, not the names.

- [ ] **Step 1: Write the failing test**

Create `internal/app/clockenv_internal_test.go`:

```go
package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClockFromEnv(t *testing.T) {
	t.Run("unset falls back to the real clock", func(t *testing.T) {
		now, err := clockFromEnv(func(string) string { return "" })
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), now(), time.Minute)
	})

	t.Run("pins the RFC 3339 instant", func(t *testing.T) {
		now, err := clockFromEnv(func(k string) string {
			if k == "HERAUT_TEST_NOW" {
				return "2026-12-31T23:59:00Z"
			}
			return ""
		})
		require.NoError(t, err)
		want := time.Date(2026, 12, 31, 23, 59, 0, 0, time.UTC)
		assert.True(t, now().Equal(want), "got %s", now())
		assert.True(t, now().Equal(want), "the pinned clock must not advance")
	})

	t.Run("rejects garbage naming the variable", func(t *testing.T) {
		_, err := clockFromEnv(func(string) string { return "yesterday" })
		require.Error(t, err)
		assert.ErrorContains(t, err, "HERAUT_TEST_NOW")
		assert.ErrorContains(t, err, "yesterday")
	})
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/app -run TestClockFromEnv`
Expected: FAIL to compile, `undefined: clockFromEnv`.

- [ ] **Step 3: Write the implementation**

Create `internal/app/clockenv.go`:

```go
package app

import (
	"fmt"
	"time"
)

const testNowEnv = "HERAUT_TEST_NOW"

// clockFromEnv returns a clock pinned to the RFC 3339 instant in HERAUT_TEST_NOW, or time.Now when
// the variable is unset. Only the heraut_testclock build wires it into clock(); it is kept out of
// the tagged file so its parsing is unit-tested in the default build.
func clockFromEnv(getenv func(string) string) (func() time.Time, error) {
	raw := getenv(testNowEnv)
	if raw == "" {
		return time.Now, nil
	}
	pinned, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s=%q is not an RFC 3339 instant: %w", testNowEnv, raw, err)
	}
	return func() time.Time { return pinned }, nil
}
```

Create `internal/app/clock.go`:

```go
//go:build !heraut_testclock

package app

import "time"

func clock() func() time.Time { return time.Now }
```

Create `internal/app/clock_testclock.go`:

```go
//go:build heraut_testclock

package app

import (
	"os"
	"time"
)

func clock() func() time.Time {
	now, err := clockFromEnv(os.Getenv)
	if err != nil {
		panic(err)
	}
	return now
}
```

In `internal/app/resolver.go` replace both `time.Now` arguments:

```go
		return calver.New(runner, cfg, clock()), nil
```
```go
		calc := calver.New(nil, cfg, clock())
```

and delete `"time"` from that file's import block (it has no other use; `grep -n "time\." internal/app/resolver.go` must print nothing).

- [ ] **Step 4: Run tests, build both variants**

Run: `go test ./internal/app && go build ./... && go build -tags heraut_testclock ./cmd/heraut`
Expected: PASS and both builds succeed.

- [ ] **Step 5: Lint and commit**

Run: `hk fix && hk check`
Expected: pass. If `unused` flags `clockFromEnv`/`testNowEnv` in the default build, keep them: the unit test uses them; if the linter still objects, report it instead of deleting the test.

```bash
git add internal/app/clockenv.go internal/app/clock.go internal/app/clock_testclock.go internal/app/clockenv_internal_test.go internal/app/resolver.go
git commit -F - <<'EOF'
refactor(app): route the CalVer clock through a build-tagged seam

Shipped builds keep time.Now. A heraut_testclock build reads
HERAUT_TEST_NOW so the e2e suite can hit period boundaries through the
real binary without a production clock override (ADR-0066).

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: e2e harness (`Binary`, `NewRepo`, `Run`)

**Files:**
- Create: `e2e/harness/build.go`
- Create: `e2e/harness/repo.go`
- Create: `e2e/harness/run.go`
- Create: `e2e/harness/harness_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces (used by Task 4):
  - `harness.Binary(t testing.TB, tags ...string) string`: path to a built `heraut`, cached per tag set.
  - `harness.Cleanup()`: removes the build directory; called from `TestMain`.
  - `harness.NewRepo(t testing.TB) *harness.Repo` with `Dir string`.
  - `(*Repo).Commit(msg string)`, `(*Repo).Tag(name string)` (annotated), `(*Repo).WriteConfig(yaml string)`.
  - `(*Repo).Run(bin string, env []string, args ...string) harness.Result`.
  - `harness.Result{Stdout, Stderr string; ExitCode int}`.

- [ ] **Step 1: Write the failing tests**

Create `e2e/harness/harness_test.go`:

```go
package harness

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo_CommitTagAndConfig(t *testing.T) {
	r := NewRepo(t)
	r.Commit("feat: first")
	r.Tag("v1.0.0")
	r.WriteConfig("version: \"1\"\n")

	assert.Equal(t, "v1.0.0", strings.TrimSpace(r.git("tag", "-l")))
	assert.Equal(t, "tag", strings.TrimSpace(r.git("cat-file", "-t", "v1.0.0")), "tags must be annotated")
	assert.Equal(t, "feat: first", strings.TrimSpace(r.git("log", "-1", "--format=%s")))
	assert.FileExists(t, r.Dir+"/.heraut.yml")
}

func TestRun_ScrubsAmbientEnvAndReportsExit(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("CI_PROJECT_PATH", "acme/widget")
	r := NewRepo(t)

	res := r.Run("/usr/bin/env", []string{"E2E_EXTRA=1"})
	require.Equal(t, 0, res.ExitCode)
	assert.NotContains(t, res.Stdout, "GITHUB_ACTIONS")
	assert.NotContains(t, res.Stdout, "CI_PROJECT_PATH")
	assert.Contains(t, res.Stdout, "E2E_EXTRA=1")
	assert.Contains(t, res.Stdout, "NO_COLOR=1")

	res = r.Run("/bin/sh", nil, "-c", "echo out; echo err >&2; exit 7")
	assert.Equal(t, 7, res.ExitCode)
	assert.Equal(t, "out\n", res.Stdout)
	assert.Equal(t, "err\n", res.Stderr)
}

func TestBinary_BuildsOncePerTagSet(t *testing.T) {
	t.Cleanup(Cleanup)
	plain := Binary(t)
	assert.Equal(t, plain, Binary(t), "same tag set must reuse the build")
	tagged := Binary(t, "heraut_testclock")
	assert.NotEqual(t, plain, tagged)
	assert.FileExists(t, plain)
	assert.FileExists(t, tagged)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./e2e/harness`
Expected: FAIL to compile, `undefined: NewRepo`, `Binary`, `Cleanup`.

- [ ] **Step 3: Write the implementation**

Create `e2e/harness/build.go`:

```go
// Package harness builds and drives the real heraut binary for the e2e suite (ADR-0066).
package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Same shape as the ldflags in .goreleaser.yml, so the version-injection path is exercised.
const ldflags = "-s -w -X main.Version=v0.0.0-e2e"

var (
	mu     sync.Mutex
	binDir string
	built  = map[string]string{}
)

// Binary builds cmd/heraut once per distinct tag set and returns the executable path.
func Binary(t testing.TB, tags ...string) string {
	t.Helper()
	key := strings.Join(tags, ",")

	mu.Lock()
	defer mu.Unlock()
	if path, ok := built[key]; ok {
		return path
	}
	if binDir == "" {
		dir, err := os.MkdirTemp("", "heraut-e2e-")
		if err != nil {
			t.Fatalf("creating build dir: %v", err)
		}
		binDir = dir
	}

	out := filepath.Join(binDir, fmt.Sprintf("heraut-%d", len(built)))
	args := []string{"build", "-ldflags", ldflags, "-o", out}
	if key != "" {
		args = append(args, "-tags", key)
	}
	args = append(args, "./cmd/heraut")

	cmd := exec.Command("go", args...)
	cmd.Dir = moduleRoot(t)
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, combined)
	}
	built[key] = out
	return out
}

// Cleanup removes the build directory; call it from TestMain after m.Run.
func Cleanup() {
	mu.Lock()
	defer mu.Unlock()
	if binDir != "" {
		_ = os.RemoveAll(binDir)
	}
	binDir = ""
	built = map[string]string{}
}

func moduleRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test's working directory")
		}
		dir = parent
	}
}
```

Create `e2e/harness/repo.go`:

```go
package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Repo is a throwaway git repository with deterministic identity and no signing.
type Repo struct {
	t    testing.TB
	Dir  string
	home string
}

// NewRepo initialises an empty repository on branch main; it skips when git is not on PATH.
func NewRepo(t testing.TB) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	r := &Repo{t: t, Dir: t.TempDir(), home: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.git("config", "user.email", "e2e@example.com")
	r.git("config", "user.name", "e2e")
	r.git("config", "commit.gpgsign", "false")
	r.git("config", "tag.gpgsign", "false")
	return r
}

// Commit records an empty commit with the given message.
func (r *Repo) Commit(msg string) {
	r.t.Helper()
	r.git("commit", "-q", "--allow-empty", "-m", msg)
}

// Tag creates an annotated tag on HEAD.
func (r *Repo) Tag(name string) {
	r.t.Helper()
	r.git("tag", "-a", name, "-m", name)
}

// WriteConfig writes .heraut.yml at the repository root.
func (r *Repo) WriteConfig(yaml string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.Dir, ".heraut.yml"), []byte(yaml), 0o644); err != nil {
		r.t.Fatalf("writing .heraut.yml: %v", err)
	}
}

func (r *Repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = baseEnv(r.home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func baseEnv(home string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1",
		"NO_COLOR=1",
		"LC_ALL=C",
	}
}
```

Create `e2e/harness/run.go`:

```go
package harness

import (
	"bytes"
	"errors"
	"os/exec"
)

// Result is the observable outcome of one binary invocation.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes bin with args inside the repository. The environment is built from scratch
// (PATH, HOME, NO_COLOR, ...) plus env, so ambient CI variables never reach the binary.
func (r *Repo) Run(bin string, env []string, args ...string) Result {
	r.t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = r.Dir
	cmd.Env = append(baseEnv(r.home), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		r.t.Fatalf("running %s %v: %v", bin, args, err)
	}
	return res
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./e2e/harness`
Expected: PASS (the build test takes a few seconds the first time).

- [ ] **Step 5: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e/harness
git commit -F - <<'EOF'
test(e2e): add the harness that builds and drives the heraut binary

Builds once per build-tag set with the goreleaser ldflags shape, creates
throwaway git repos, and runs the binary with an environment built from
scratch so ambient CI variables cannot change detection (ADR-0066).

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Lane A scenarios (SemVer, `stay_at_v0`, pre-release, overrides, CalVer clock)

**Files:**
- Create: `e2e/main_test.go`
- Create: `e2e/scenario_test.go` (shared table runner)
- Create: `e2e/semver_test.go`
- Create: `e2e/calver_clock_test.go`
- Create: `e2e/cli_test.go`

**Interfaces:**
- Consumes: `harness.Binary`, `harness.Cleanup`, `harness.NewRepo`, `(*Repo).Commit/Tag/WriteConfig/Run`, `harness.Result` (Task 3); the `heraut_testclock` build behaviour (Task 2).
- Produces: the `scenario` table runner (`runScenarios`) that T345b extends.

Because these scenarios characterise behaviour that already ships, the TDD "red" is the missing runner/files. Write the runner and one row first, confirm it fails for the right reason, then add the rows. Every expected value below was checked against the real binary on 2026-10-07; if a row fails, treat it as a possible regression, investigate before touching the expectation.

- [ ] **Step 1: Write the runner and `TestMain`**

Create `e2e/main_test.go`:

```go
package e2e_test

import (
	"os"
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestMain(m *testing.M) {
	code := m.Run()
	harness.Cleanup()
	os.Exit(code)
}
```

Create `e2e/scenario_test.go`:

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

type step struct{ commit, tag string }

func commit(msg string) step { return step{commit: msg} }
func tag(name string) step   { return step{tag: name} }

const (
	exitOK      = 0
	exitConfig  = 2
	exitRuntime = 3
)

type scenario struct {
	name     string
	config   string
	history  []step
	args     []string
	wantExit int
	wantOut  string   // exact trimmed stdout; checked only when wantExit == exitOK
	wantText []string // lower-cased, whitespace-collapsed substrings of stdout+stderr
	notText  []string // substrings that must be absent
}

// normalize lower-cases and collapses whitespace: the CLI error panel re-cases and wraps text.
func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func runScenarios(t *testing.T, bin string, env []string, tests []scenario) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := harness.NewRepo(t)
			repo.WriteConfig(tc.config)
			for _, s := range tc.history {
				if s.tag != "" {
					repo.Tag(s.tag)
				} else {
					repo.Commit(s.commit)
				}
			}

			res := repo.Run(bin, env, tc.args...)

			require.Equal(t, tc.wantExit, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			if tc.wantExit == exitOK {
				assert.Equal(t, tc.wantOut, strings.TrimSpace(res.Stdout))
			}
			all := normalize(res.Stdout + " " + res.Stderr)
			for _, want := range tc.wantText {
				assert.Contains(t, all, want)
			}
			for _, not := range tc.notText {
				assert.NotContains(t, all, not)
			}
		})
	}
}
```

- [ ] **Step 2: Write one row and see it fail**

Create `e2e/semver_test.go` with the config helpers and a single row:

```go
package e2e_test

import "testing"

func semverCfg(bumpExtra string) string {
	return `version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  initial_version: "0.1.0"
  bump:
    mode: auto
` + bumpExtra
}

const stayAtV0 = "    stay_at_v0: true\n"

var versionNext = []string{"version", "next"}

func TestSemVer_Resolution(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "feat bumps minor and 1.9.0 goes to 1.10.0", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("feat: b")},
			args:    versionNext, wantOut: "v1.10.0"},
	})
}
```

(add `"github.com/adaouat/heraut/e2e/harness"` to the imports of this file.)

Run: `go test ./e2e -run TestSemVer_Resolution`
Expected: PASS for the one row once the import is present. To observe a real red first, temporarily change `wantOut` to `"v1.100.0"`, run, confirm the failure message shows `v1.10.0`, then restore `v1.10.0`.

- [ ] **Step 3: Add the remaining SemVer rows**

Replace the body of `TestSemVer_Resolution` with the full table (keep the rest of the file):

```go
func TestSemVer_Resolution(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "untagged repo uses initial_version", config: semverCfg(""),
			args: versionNext, wantOut: "v0.1.0"},
		{name: "feat bumps minor and 1.9.0 goes to 1.10.0", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("feat: b")},
			args:    versionNext, wantOut: "v1.10.0"},
		{name: "fix bumps patch", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("fix: b")},
			args:    versionNext, wantOut: "v1.9.1"},
		{name: "chore is a patch per the default table", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("chore: b")},
			args:    versionNext, wantOut: "v1.9.1"},
		{name: "bang prefix bumps major", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("feat!: b")},
			args:    versionNext, wantOut: "v2.0.0"},
		{name: "BREAKING CHANGE footer bumps major", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.9.0"), commit("fix: b\n\nBREAKING CHANGE: x")},
			args:    versionNext, wantOut: "v2.0.0"},
		{name: "non-conventional commits give no bump", config: semverCfg(""),
			history:  []step{commit("feat: a"), tag("v1.0.0"), commit("update stuff")},
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"no releasable commits since v1.0.0"}},
		{name: "custom tag prefix round-trips", config: `version: "1"
versioning:
  strategy: semver
  tag_prefix: "rel-"
  initial_version: "0.1.0"
  bump:
    mode: auto
`,
			history: []step{commit("feat: a"), tag("rel-1.0.0"), commit("fix: b")},
			args:    versionNext, wantOut: "rel-1.0.1"},
		{name: "pre-release tag is not the bump base", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.1")},
			args:    versionNext, wantOut: "v1.4.0"},
		{name: "build-metadata tag counts as its core release", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.4.0"), commit("fix: b"), tag("v1.4.1+158404"), commit("fix: c")},
			args:    versionNext, wantOut: "v1.4.2"},
		{name: "manual mode refuses to compute", config: `version: "1"
versioning:
  strategy: semver
  bump:
    mode: manual
`,
			history:  []step{commit("feat: a")},
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"manual bump mode requires --set-version"}},
	})
}

func TestSemVer_StayAtV0(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "without the setting a breaking commit reaches 1.0.0", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    versionNext, wantOut: "v1.0.0"},
		{name: "setting holds the major back to a minor with a warning", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    versionNext, wantOut: "v0.5.0",
			wantText: []string{"major bump held back by versioning.bump.stay_at_v0", "feat!: b"}},
		{name: "--allow-major lifts it for the run", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    []string{"version", "next", "--allow-major"}, wantOut: "v1.0.0",
			notText: []string{"held back"}},
		{name: "no effect once the major is 1 or higher", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v1.2.0"), commit("feat!: b")},
			args:    versionNext, wantOut: "v2.0.0", notText: []string{"held back"}},
		{name: "--set-version ignores the setting", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    []string{"version", "next", "--set-version", "1.0.0"}, wantOut: "v1.0.0",
			notText: []string{"held back"}},
		{name: "a pre-release is held back too, naming the candidates", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    []string{"version", "next", "--pre-release", "rc"}, wantOut: "v0.5.0-rc.1",
			wantText: []string{"major bump held back", "v1.0.0-rc.1"}},
		{name: "a pre-release with --allow-major opens the major", config: semverCfg(stayAtV0),
			history: []step{commit("feat: a"), tag("v0.4.0"), commit("feat!: b")},
			args:    []string{"version", "next", "--pre-release", "rc", "--allow-major"}, wantOut: "v1.0.0-rc.1"},
	})
}

func TestSemVer_PreReleaseLifecycle(t *testing.T) {
	bin := harness.Binary(t)
	rc := []string{"version", "next", "--pre-release", "rc"}
	runScenarios(t, bin, nil, []scenario{
		{name: "first rc of the next minor", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b")},
			args:    rc, wantOut: "v1.4.0-rc.1"},
		{name: "second rc increments the counter", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.1"), commit("fix: c")},
			args:    rc, wantOut: "v1.4.0-rc.2"},
		{name: "re-cutting a label needs a new commit", config: semverCfg(""),
			history:  []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.1")},
			args:     rc,
			wantExit: exitRuntime, wantText: []string{"no commits since v1.4.0-rc.1"}},
		{name: "a higher label needs no new commit", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-beta.2")},
			args:    rc, wantOut: "v1.4.0-rc.1"},
		{name: "a lower label than an existing one is a regression", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.2"), commit("fix: c")},
			args:    []string{"version", "next", "--pre-release", "beta"},
			wantExit: exitRuntime, wantText: []string{"would sort below existing v1.4.0-rc.2"}},
		{name: "a breaking commit escalating the series is refused", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.2"), commit("feat!: d")},
			args:    rc,
			wantExit: exitRuntime, wantText: []string{"would escalate to a new major 2.0.0"}},
		{name: "--allow-major opens the new series with a warning", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.3.0"), commit("feat: b"), tag("v1.4.0-rc.2"), commit("feat!: d")},
			args:    []string{"version", "next", "--pre-release", "rc", "--allow-major"}, wantOut: "v2.0.0-rc.1",
			wantText: []string{"pre-release core escalated 1.4.0 → 2.0.0"}},
		{name: "--pre-release cannot be combined with --set-version", config: semverCfg(""),
			history:  []step{commit("feat: a")},
			args:     []string{"version", "next", "--pre-release", "rc", "--set-version", "1.0.0"},
			wantExit: exitConfig, wantText: []string{"cannot be combined with --set-version"}},
		{name: "a dotted label is rejected", config: semverCfg(""),
			history:  []step{commit("feat: a")},
			args:     []string{"version", "next", "--pre-release", "bad.label"},
			wantExit: exitConfig, wantText: []string{"contains '.'"}},
		{name: "calver cannot mint pre-releases", config: `version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  tag_prefix: ""
`,
			history:  []step{commit("feat: a")},
			args:     rc,
			wantExit: exitConfig, wantText: []string{"requires versioning.strategy: semver"}},
	})
}

func TestSemVer_Overrides(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "set-version renders through the prefix", config: semverCfg(""),
			args: []string{"version", "next", "--set-version", "1.2.3"}, wantOut: "v1.2.3"},
		{name: "an already prefixed value round-trips", config: semverCfg(""),
			args: []string{"version", "next", "--set-version", "v1.2.3"}, wantOut: "v1.2.3"},
		{name: "set-build-id appends build metadata", config: semverCfg(""),
			args: []string{"version", "next", "--set-version", "1.2.3", "--set-build-id", "99"}, wantOut: "v1.2.3+99"},
		{name: "build metadata inside the version is rejected", config: semverCfg(""),
			args:     []string{"version", "next", "--set-version", "1.2.3+5"},
			wantExit: exitConfig, wantText: []string{"must not carry build metadata"}},
		{name: "a non-SemVer value is rejected", config: semverCfg(""),
			args:     []string{"version", "next", "--set-version", "nope"},
			wantExit: exitConfig, wantText: []string{"is not a valid semver version"}},
	})
}
```

Run: `go test ./e2e -run 'TestSemVer_'`
Expected: PASS. If a row fails, copy the failure output into the report and stop: do not edit the expectation until it is clear whether heraut regressed.

- [ ] **Step 4: Add the `version current` and `--version` scenarios**

Create `e2e/cli_test.go`:

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestCLI_VersionCurrentIgnoresPreReleases(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "latest final wins over a newer pre-release", config: semverCfg(""),
			history: []step{commit("feat: a"), tag("v1.0.0"), commit("feat: b"), tag("v1.1.0-rc.1")},
			args:    []string{"version", "current"}, wantOut: "v1.0.0"},
	})
}

func TestCLI_VersionFlagReportsTheInjectedVersion(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)

	res := repo.Run(bin, nil, "--version")

	assert.Equal(t, exitOK, res.ExitCode)
	assert.Contains(t, strings.Join(strings.Fields(res.Stdout), " "), "0.0.0-e2e")
}
```

Run: `go test ./e2e -run TestCLI_`
Expected: PASS.

- [ ] **Step 5: Add the CalVer clock scenarios**

Create `e2e/calver_clock_test.go`:

```go
package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/e2e/harness"
)

const calverCfg = `version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  tag_prefix: ""
`

func TestCalVer_SimulatedClock(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	history := []step{commit("feat: a"), tag("2026.10.3"), commit("feat: b")}
	at := func(instant string) []string { return []string{"HERAUT_TEST_NOW=" + instant} }

	runScenarios(t, bin, at("2026-10-20T12:00:00Z"), []scenario{
		{name: "same month increments PATCH", config: calverCfg, history: history,
			args: versionNext, wantOut: "2026.10.4"},
	})
	runScenarios(t, bin, at("2026-11-01T00:00:00Z"), []scenario{
		{name: "next month resets PATCH to 0", config: calverCfg, history: history,
			args: versionNext, wantOut: "2026.11.0"},
	})
}

func TestCalVer_InvalidTestClockFailsLoudly(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	repo := harness.NewRepo(t)
	repo.WriteConfig(calverCfg)
	repo.Commit("feat: a")

	res := repo.Run(bin, []string{"HERAUT_TEST_NOW=yesterday"}, "version", "next")

	assert.NotEqual(t, exitOK, res.ExitCode)
	assert.Contains(t, res.Stdout+res.Stderr, "HERAUT_TEST_NOW")
}

func TestCalVer_ShippedBinaryIgnoresTestClock(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.WriteConfig(calverCfg)
	repo.Commit("feat: a")

	res := repo.Run(bin, []string{"HERAUT_TEST_NOW=2000-01-01T00:00:00Z"}, "version", "next")

	assert.Equal(t, exitOK, res.ExitCode)
	assert.False(t, strings.HasPrefix(strings.TrimSpace(res.Stdout), "2000."),
		"the shipped binary must use the real clock, got %q", res.Stdout)
}
```

Run: `go test ./e2e -run TestCalVer_`
Expected: PASS. The panic in the tagged build exits non-zero and prints the variable name on stderr.

- [ ] **Step 6: Run the whole suite and time it**

Run: `go test ./... 2>&1 | tail -5`
Expected: all packages pass. Record the wall-clock time of `go test ./e2e/...` for Task 5's note: `time go test -count=1 ./e2e/...`.

- [ ] **Step 7: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): cover SemVer resolution, stay_at_v0 and pre-releases end to end

Table-driven scenarios run the built binary against throwaway repos:
bump matrix, v1.9.0 -> v1.10.0, stay_at_v0 and --allow-major, the
pre-release lifecycle and its guards, --set-version/--set-build-id, and a
CalVer clock smoke through the heraut_testclock build.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Roadmap, findings and closing note

**Files:**
- Modify: `docs/tasks/roadmap.md` (Phase 60 table row near line 231, the T345 entry near line 2599, and a new T357 entry)

**Interfaces:**
- Consumes: the timing recorded in Task 4 step 6, and the list of defects (if any) that scenarios exposed.
- Produces: the roadmap state later sessions read: T345a `[x]`, T345b-d `[ ]`, T357 `[ ]`.

- [ ] **Step 1: Replace the T345 entry with the four sub-tasks**

In `docs/tasks/roadmap.md`, replace the whole `#### ✦ \`[ ]\` T345: …` block (heading, paragraph and bullet sketch, up to but not including `### Phase 61`) with:

```markdown
Design: [`docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md`](../superpowers/specs/2026-10-07-e2e-smoke-suite-design.md)
(two lanes: a hermetic binary lane that gates every PR, and an opt-in forge-sandbox lane;
[ADR-0066](../adr/0066-e2e-test-lanes.md)). T345 is split into four tasks, one per session:

#### `[x]` T345a: ADR-0066, harness, test clock, SemVer / `stay_at_v0` / pre-release scenarios (Lane A)

**Completion note:** <write one paragraph: what landed (plan `docs/superpowers/plans/2026-10-07-e2e-lane-a-foundation.md`), the measured `go test ./e2e/...` time and the `go test ./...` before/after, any scenario that exposed a real defect and how it was handled, and the deferral of the `native` generator clock to T345b.>

#### `[ ]` T345b: remaining Lane A scenarios

CalVer period boundaries with the simulated clock (month/year/ISO-week/quarter, `PATCH` reset, sprint),
per-env promotion (E001/E002/E003, `--force`, `tag_format`) including `stay_at_v0` under
`semver-per-env`, maintenance branches, changelog and release local flow against a bare remote with
fake `gh`/`glab`, hooks, `--offline`, CLI surface (`check`, `~` expansion, `commit verify`). Adds
`native.WithClock` (the generator's `GeneratedAt`) fed from `app`'s clock seam.

#### `[ ]` T345c: forge harness and scenarios B1-B4 (Lane B)

`e2e_forge` build tag, sandbox configuration and safety guards, per-run branch and tag namespace,
cleanup plus sweeper; final release, pre-release, build metadata and per-env-with-asset scenarios on
GitHub and GitLab (the per-env one is T335's regression test).

#### `[ ]` T345d: scenarios B5-B10, workflow, guide

CalVer, cross-forge GitLab-to-GitHub, draft, maintenance branch, PR/MR enrichment, dry-run;
`.github/workflows/e2e.yml` (`workflow_dispatch` + nightly, advisory), `mise run test:e2e`,
`docs/guides/e2e-tests.md`.

#### `[ ]` T357: error panel mangles identifiers in messages

Surfaced while writing T345a: the CLI error display re-cases and wraps message text, e.g. `V1.4.0-Beta.1
would sort below …`, `--Pre-Release cannot be combined with --set-version`, `--Set-Version "nope" …`.
Flags, tags and config keys must render verbatim. Find the owner first (heraut's `internal/ui` or
`forge/cli`/fang styling), then fix at the root with a test that asserts an identifier survives
rendering; the e2e scenarios already normalise case, so they will not catch a regression.
```

- [ ] **Step 2: Update the Phase 60 status row**

Change the table row `| 60 | End-to-end smoke tests … | Not started — see T345 (needs design) |` to:

```markdown
| 60 | End-to-end tests: hermetic binary lane + opt-in forge sandboxes | In progress — T345a done; T345b-d, T357 open |
```

- [ ] **Step 3: Fill in the T345a completion note**

Replace the angle-bracket placeholder in T345a's note with the actual paragraph, using the numbers recorded in Task 4 step 6. It must state real facts only (timing, any defect found and its disposition, the native-clock deferral). Do not leave angle brackets in the file: `grep -n '<write one paragraph' docs/tasks/roadmap.md` must print nothing.

- [ ] **Step 4: Verify and commit**

Run: `go test ./... && hk check`
Expected: all pass.

```bash
git add docs/tasks/roadmap.md
git commit -F - <<'EOF'
docs(roadmap): split T345 into T345a-d, close T345a, file T357

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

## Self-review

**Spec coverage** (T345a row: "ADR-0066, testing-rule amendment, harness, `heraut_testclock` clock, first scenarios: SemVer resolution + `stay_at_v0` + pre-release lifecycle"): ADR and rule amendment (Task 1); clock (Task 2); harness `Build`/`NewRepo`/`Run` (Task 3, `Build` is named `Binary` here); SemVer resolution, `stay_at_v0`, pre-release lifecycle, overrides, CalVer clock smoke and the CLI `--version`/`version current` rows (Task 4); roadmap breakdown (Task 5). Deliberately not in this plan: the harness's bare-remote helper and `native` clock (T345b), per-env `stay_at_v0` (T345b), `Lane B` (T345c/d), the `mise run test:e2e` task (T345d, it is Lane B's entry point).

**Type consistency:** `harness.Binary(t, tags...)`, `Cleanup()`, `NewRepo(t)`, `Repo.Commit/Tag/WriteConfig/Run(bin, env, args...)`, `Result{Stdout, Stderr, ExitCode}` are defined in Task 3 and used with the same signatures in Task 4. `clockFromEnv(getenv)` and `clock()` match between Task 2's test, implementation and `resolver.go` edit.

**Placeholders:** the only deferred content is the T345a completion note, which is filled in Task 5 step 3 from measured numbers and checked by grep.
