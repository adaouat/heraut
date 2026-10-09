# E2E release flow with fake gh/glab (T345b4c) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cover the full `heraut release` through the real binary with fake `gh` and `glab` executables that record their arguments: the GitHub and GitLab driver argv (including `--draft`, `--prerelease`, assets), multi-target ordering, the per-target `pre_release`/`post_release` hooks, a failing publish aborting the loop, `--dry-run` calling nothing, the missing-token preflight, and `release --force` on an unlisted branch (deferred from T345b3).

**Architecture:** The harness gains `Repo.FakeCLI(name)`: an executable that appends `name [arg] [arg]…` to a shared call log, and fails `release` subcommands when told to. `Repo.Run` puts the fake directory first on `PATH`. Scenarios are standalone tests on the T345b4a `flowRepo`. No production change.

**Tech Stack:** Go 1.27, testify, POSIX `sh` for the fakes (macOS/Linux).

**Spec:** `docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md` (Lane A "fake `gh`/`glab` recording argv"); behaviour source `docs/specs/02-configuration.md` § `release.targets`, `docs/specs/05-generators-and-platforms.md`, `docs/guides/release-pipeline-and-hooks.md`, ADR-0044/0053.

## Global Constraints

- TDD: failing test first, see it fail; characterising tests are proven able to fail by a deliberate mutation of one expectation, restored from a backup copy (never `sed 0,/x/` on macOS).
- Never `--no-verify`; fix lint with `hk fix`. Conventional commits, subject ≤ 72 characters (count it), trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, never `Claude-Session:`. Commits on `main`; do not push.
- `e2e/` imports no heraut `internal/` package. Error text is matched with `normalize`. Do not pin exit codes the spec leaves open: a failed publish exits 3 in the shipped code but the spec only says "runtime"; per Spec 01, "token env var unset" and "git operation failed" are Runtime (3), so those may be pinned, a failing `gh release create` (a failed external CLI) is covered by "binary missing / network failure" wording loosely, so assert `NotEqual(exitOK)` for it.
- The fakes never touch the network. `HERAUT_CHECK_UPDATE=false` is already set by the harness; tokens are passed explicitly per run.
- The notes file is a temp file whose path varies (`/tmp/heraut-notes-<n>`): normalise it to `<notes>` before comparing argv.
- Scope: `heraut release` against fakes. Real forge calls are Lane B (T345c-d).
- Every expected value below was produced by the real binary on 2026-10-09. If a test fails, investigate before touching the expectation.

## Review Focus

1. A failing publish must abort the loop (`glab` is not called after a failing `gh`), keep the already-pushed tag, and exit non-zero.
2. A failing `pre_release` hook must skip only its own target; the other target still publishes; the run still exits non-zero.
3. `--dry-run` must call neither CLI nor create a tag, commit or file.
4. A missing token must be refused in preflight, before any tag or commit.
5. `--prerelease` and `--draft` must appear exactly when the version/target calls for them.

---

### Task 1: `Repo.FakeCLI`

**Files:**
- Modify: `e2e/harness/repo.go` (field + `Run` PATH)
- Modify: `e2e/harness/run.go` (use the fake dirs)
- Create: `e2e/harness/fakecli.go`
- Modify: `e2e/harness/harness_test.go`

**Interfaces:**
- Consumes: `Repo`, `baseEnv`, `Repo.Run` (T345a).
- Produces: `(*Repo).FakeCLI(name string)` (installs the fake and records into the shared log), `(*Repo).FailReleases(name string)` (the fake's `release …` calls exit 1), `(*Repo).CLICalls() []string` (every recorded call, in order, as `name [arg] [arg]`).

- [ ] **Step 1: Write the failing test**

Append to `e2e/harness/harness_test.go`:

```go
func TestRepo_FakeCLIRecordsCallsInOrderAndCanFail(t *testing.T) {
	r := NewRepo(t)
	r.FakeCLI("gh")
	r.FakeCLI("glab")

	res := r.Run("/bin/sh", nil, "-c", "gh --version; glab release create v1 --repo acme/w; gh release create v1; echo rc=$?")
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, []string{
		"gh [--version]",
		"glab [release] [create] [v1] [--repo] [acme/w]",
		"gh [release] [create] [v1]",
	}, r.CLICalls())

	r.FailReleases("gh")
	res = r.Run("/bin/sh", nil, "-c", "gh release create v2; echo rc=$?")
	assert.Equal(t, "rc=1\n", res.Stdout)
	assert.Equal(t, "gh [release] [create] [v2]", r.CLICalls()[3], "a failing call is still recorded")
}
```

Run: `go test ./e2e/harness -run TestRepo_FakeCLI`
Expected: FAIL to compile, `r.FakeCLI undefined`.

- [ ] **Step 2: Implement**

In `e2e/harness/repo.go`, add two fields to `Repo`: `binDir string` (the directory holding the fakes and the shared log; empty until the first `FakeCLI`).

Create `e2e/harness/fakecli.go`:

```go
package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fakeScript = `#!/bin/sh
d=$(dirname "$0")
{ printf "%s" "%[1]s"; for a in "$@"; do printf " [%%s]" "$a"; done; echo; } >> "$d/calls.log"
if [ "$1" = release ] && [ -f "$d/fail-%[1]s" ]; then
  echo "boom" >&2
  exit 1
fi
exit 0
`

// FakeCLI installs an executable called name in a directory that Run puts first on PATH. It
// appends "name [arg] [arg]" to a call log shared by every fake of this repository and exits 0.
func (r *Repo) FakeCLI(name string) {
	r.t.Helper()
	if r.binDir == "" {
		r.binDir = r.t.TempDir()
	}
	script := fmt.Sprintf(fakeScript, name)
	if err := os.WriteFile(filepath.Join(r.binDir, name), []byte(script), 0o755); err != nil {
		r.t.Fatalf("installing fake %s: %v", name, err)
	}
}

// FailReleases makes the named fake exit 1 on any "release …" subcommand (calls are still recorded).
func (r *Repo) FailReleases(name string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.binDir, "fail-"+name), nil, 0o644); err != nil {
		r.t.Fatalf("marking %s as failing: %v", name, err)
	}
}

// CLICalls returns every call recorded by the fakes, oldest first.
func (r *Repo) CLICalls() []string {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.binDir, "calls.log"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		r.t.Fatalf("reading the call log: %v", err)
	}
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
```

In `e2e/harness/run.go`, change the environment line in `Run` to put `r.binDir` first on `PATH` when set:

```go
	cmd.Env = append(r.runEnv(), env...)
```

and add to `fakecli.go`:

```go
// runEnv is baseEnv with the fake directory (if any) prepended to PATH.
func (r *Repo) runEnv() []string {
	env := baseEnv(r.home)
	if r.binDir == "" {
		return env
	}
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + r.binDir + string(os.PathListSeparator) + strings.TrimPrefix(kv, "PATH=")
		}
	}
	return env
}
```

Run: `go test -count=1 ./e2e/harness`
Expected: PASS.

- [ ] **Step 3: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e/harness
git commit -F - <<'EOF'
test(e2e): add recording fake CLIs to the harness

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Release scenarios

**Files:**
- Create: `e2e/release_flow_test.go`

**Interfaces:**
- Consumes: Task 1 helpers; `flowRepo`, `runOK` (T345b4a), `normalize`, `exitOK`, `exitConfig`, `exitRuntime`, `harness.Binary/Run/Git/GitRemote/WriteFile`.
- Produces: `forgesCfg`, `releaseCfg(targets, extra string) string`, `releaseRepo(t, cfg) *harness.Repo`, `normCalls(calls []string) []string`.

- [ ] **Step 1: Write the first test (red: the file does not exist)**

Create `e2e/release_flow_test.go`:

```go
package e2e_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

const forgesCfg = `version: "1"
versioning:
  strategy: semver
changelog:
  output: CHANGELOG.md
commits:
  enrichment_forge: github
forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN
  - name: gitlab
    platform: gitlab
    project: acme/widget
    token_env: GITLAB_TOKEN
`

// releaseCfg appends a release block (targets) and any extra top-level config to forgesCfg.
func releaseCfg(targets, extra string) string {
	return forgesCfg + "release:\n  targets:\n" + targets + extra
}

const bothTargets = `    - forge: github
    - forge: gitlab
`

var notesPath = regexp.MustCompile(`\[[^\]]*heraut-notes-[0-9]+\]`)

// normCalls replaces the temp notes-file path, which differs per run, with <notes>.
func normCalls(calls []string) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = notesPath.ReplaceAllString(c, "[<notes>]")
	}
	return out
}

var tokens = []string{"GH_TOKEN=ghtok", "GITLAB_TOKEN=gltok"}

// releaseRepo is a flowRepo with both fake CLIs installed and one releasable commit.
func releaseRepo(t *testing.T, cfg string) *harness.Repo {
	t.Helper()
	repo := flowRepo(t, cfg)
	repo.FakeCLI("gh")
	repo.FakeCLI("glab")
	repo.Commit("feat: one")
	return repo
}

func TestRelease_GitHubArgv(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Equal(t, []string{
		"gh [--version]",
		"gh [api] [repos/acme/widget/releases?per_page=1]",
		"gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]",
	}, normCalls(repo.CLICalls()))
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag is pushed before the release is created")
	assert.Contains(t, normalize(res.Stdout), "https://github.com/acme/widget/releases/tag/v0.1.0")
}
```

Run: `go test -count=1 ./e2e -run TestRelease_GitHubArgv`
Expected: PASS once written. Prove it can fail: back up the file, exact-replace `"gh [--version]",` with `"gh [--versions]",`, run, confirm the failure shows `gh [--version]`, restore from the backup.

- [ ] **Step 2: Add the remaining tests**

Append to `e2e/release_flow_test.go`:

```go
func TestRelease_TwoTargetsPublishInDeclaredOrderWithDriverFlags(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(`    - forge: github
      draft: true
    - forge: gitlab
`, ""))

	res := repo.Run(bin, tokens, "release", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Equal(t, []string{
		"gh [--version]",
		"gh [api] [repos/acme/widget/releases?per_page=1]",
		"glab [--version]",
		"glab [api] [user]",
		"gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget] [--draft]",
		"glab [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]",
	}, normCalls(repo.CLICalls()), "every target is probed first, then published in declared order; draft is GitHub-only")
}

func TestRelease_PreReleaseIsMarkedFromTheVersion(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))

	res := repo.Run(bin, tokens, "release", "--offline", "--pre-release", "rc")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	calls := normCalls(repo.CLICalls())
	assert.Equal(t, "gh [release] [create] [v0.1.0-rc.1] [--notes-file] [<notes>] [--repo] [acme/widget] [--prerelease]", calls[len(calls)-1])
}

func TestRelease_AssetsAreAttachedAndAZeroMatchIsLenient(t *testing.T) {
	bin := harness.Binary(t)
	cfg := releaseCfg(`    - forge: github
      assets:
        - "dist/*.txt"
`, "")

	t.Run("matching files are passed to gh", func(t *testing.T) {
		repo := releaseRepo(t, cfg)
		repo.WriteFile("dist/a.txt", "a\n")
		repo.WriteFile("dist/b.txt", "b\n")

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		calls := normCalls(repo.CLICalls())
		assert.Equal(t, "gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget] [dist/a.txt] [dist/b.txt]", calls[len(calls)-1])
	})
	t.Run("a pattern matching nothing publishes without assets", func(t *testing.T) {
		repo := releaseRepo(t, cfg)

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		calls := normCalls(repo.CLICalls())
		assert.Equal(t, "gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]", calls[len(calls)-1])
	})
}

func TestRelease_AFailingPublishAbortsTheLoop(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(bothTargets, ""))
	repo.FailReleases("gh")

	res := repo.Run(bin, tokens, "release", "--offline")

	require.NotEqual(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "publish to github")
	for _, call := range repo.CLICalls() {
		assert.False(t, strings.HasPrefix(call, "glab [release]"), "the second target must not publish: %s", call)
	}
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag was already pushed and stays")
}

func TestRelease_DryRunCallsNothingAndCreatesNothing(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))
	before := repoState(t, repo)

	res := repo.Run(bin, tokens, "release", "--offline", "--dry-run")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout), "[dry-run] would create release")
	assert.Empty(t, repo.CLICalls(), "neither gh nor glab is invoked")
	assert.Equal(t, before, repoState(t, repo), "no tag, commit or file")
	assert.Empty(t, repo.GitRemote("tag", "-l"))
}

func TestRelease_ReleaseHooksRunPerTarget(t *testing.T) {
	bin := harness.Binary(t)

	t.Run("pre_release and post_release wrap each target in order", func(t *testing.T) {
		repo := releaseRepo(t, releaseCfg(bothTargets, `hooks:
  pre_release:
    - run: "echo pre {{ .Platform }} >> .hooklog"
  post_release:
    - run: "echo post {{ .Platform }} {{ .Tag }} >> .hooklog"
`))

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, []string{"pre github", "post github v0.1.0", "pre gitlab", "post gitlab v0.1.0"},
			logLines(repo.ReadFile(".hooklog")))
	})
	t.Run("a failing pre_release skips only its own target", func(t *testing.T) {
		repo := releaseRepo(t, releaseCfg(bothTargets, `hooks:
  pre_release:
    - run: "{{ if eq .Platform \"github\" }}exit 1{{ else }}echo ok >> .hooklog{{ end }}"
`))

		res := repo.Run(bin, tokens, "release", "--offline")

		require.NotEqual(t, exitOK, res.ExitCode, "the run still fails when a target was skipped")
		calls := repo.CLICalls()
		for _, call := range calls {
			assert.False(t, strings.HasPrefix(call, "gh [release]"), "github must be skipped: %s", call)
		}
		assert.Contains(t, normCalls(calls), "glab [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]")
		assert.Equal(t, []string{"ok"}, logLines(repo.ReadFile(".hooklog")))
	})
}

func TestRelease_AMissingTokenIsRefusedInPreflight(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))
	before := repoState(t, repo)

	res := repo.Run(bin, nil, "release", "--offline")

	require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "environment variable gh_token is not set")
	assert.Equal(t, []string{"gh [--version]"}, repo.CLICalls(), "nothing but the binary check ran")
	assert.Equal(t, before, repoState(t, repo), "no tag, commit or file")
}

func TestRelease_ForceLiftsTheUnlistedBranchRefusal(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver
  branches:
    - name: main
changelog:
  output: CHANGELOG.md
forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN
release:
  targets:
    - forge: github
`
	newRepo := func(t *testing.T) *harness.Repo {
		repo := releaseRepo(t, cfg)
		repo.Checkout("feature/x")
		return repo
	}

	t.Run("without --force nothing is published", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, tokens, "release", "--offline")

		require.Equal(t, exitConfig, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Empty(t, repo.CLICalls())
		assert.Empty(t, repo.GitRemote("tag", "-l"))
	})
	t.Run("with --force the release goes ahead", func(t *testing.T) {
		repo := newRepo(t)

		res := repo.Run(bin, tokens, "release", "--offline", "--force")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		calls := normCalls(repo.CLICalls())
		assert.Equal(t, "gh [release] [create] [v0.1.0] [--notes-file] [<notes>] [--repo] [acme/widget]", calls[len(calls)-1])
	})
}
```

Run: `go test -count=1 ./e2e -run TestRelease_`
Expected: PASS. If a test fails, copy the output and stop; investigate before touching the expectation. Then prove two tests can fail: back up the file, exact-replace `"glab [api] [user]",` with `"glab [api] [users]",` and `assert.Equal(t, []string{"ok"}, logLines(repo.ReadFile(".hooklog")))` with `assert.Equal(t, []string{"ok", "never"}, logLines(repo.ReadFile(".hooklog")))`, run `-run 'TwoTargets|ReleaseHooks'`, confirm both fail showing the real values, restore from the backup.

- [ ] **Step 3: Full suite, lint, commit**

Run: `go test -count=1 ./... && hk fix && hk check`

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): cover the full release flow against recording fake gh/glab

GitHub and GitLab driver argv (draft, pre-release, assets, probes),
declared-order publishing, a failing publish aborting the loop,
per-target release hooks, dry-run calling nothing, the missing-token
preflight, and --force lifting the unlisted-branch refusal.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Close T345b4c and T345b4

**Files:**
- Modify: `docs/tasks/roadmap.md`

- [ ] **Step 1: Flip, annotate, and close the parent**

Change `- \`[ ]\` **T345b4c**:` to `- \`[x]\` **T345b4c**:` and the parent `- \`[ ]\` **T345b4**:` to `- \`[x]\` **T345b4**:`. Add an indented completion note under T345b4c with real facts: the number of `TestRelease_` tests/subtests (`go test -count=1 -v ./e2e -run TestRelease_ 2>&1 | grep -c -- '--- PASS'`) and the new e2e total, the harness addition (`FakeCLI`, `FailReleases`, `CLICalls`, a `PATH` prefix in `Run`), that `release --force` on an unlisted branch (deferred from T345b3) is covered, and any deviation. Update the Phase 60 status row to include T345b4c. `grep -n 'T345b4c' docs/tasks/roadmap.md` must show the `[x]` bullet with no placeholder text.

- [ ] **Step 2: Verify and commit**

Run: `go test -count=1 ./... && hk check`

```bash
git add docs/tasks/roadmap.md
git commit -F - <<'EOF'
docs(roadmap): close T345b4c and T345b4

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

## Self-review

**Spec coverage** (Lane A: "`release` runs with fake `gh`/`glab` binaries that record argv, so a flag regression is caught without a forge"; "`--offline`"): argv for both drivers incl. flags, order, abort, per-target hooks, dry-run, preflight, `--force`, plus `--offline` on every run.

**Type consistency:** `FakeCLI`, `FailReleases`, `CLICalls`, `runEnv` are defined in Task 1 and used in Task 2; `repoState`, `logLines`, `flowRepo`, `runOK` come from earlier parts; `tokens`, `bothTargets`, `releaseRepo`, `normCalls` are defined in this file.

**Placeholders:** the Task 3 note is written from measured counts at execution time.
