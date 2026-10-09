# E2E maintenance branches (T345b3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cover ADR-0065 end to end through the real binary: branch-aware version resolution on declared maintenance lines, the hard errors (out of range, tag exists, no in-range base, ambiguous, underivable), unlisted-branch handling, CI-variable branch detection on a detached HEAD, `--set-version` collision rules, and history-aware changelog bounds (unmerged tags get no section, forward merges, same-commit tags).

**Architecture:** Harness gains three git helpers (`Switch`, `Detach`, `MergeNoFF`). The scenario runner's `step` gains branch operations so a layout (main plus a maintenance line) is declared as data. A small `changelogOutline` helper turns `CHANGELOG.md` into comparable `version: subject, subject` lines. No production code changes.

**Tech Stack:** Go 1.27, testify, the e2e harness.

**Spec:** `docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md` (Lane A "Maintenance branches (ADR-0065)"); behaviour source `docs/adr/0065-branch-aware-semver-resolution.md`, `docs/guides/maintenance-branches.md`.

## Global Constraints

- TDD: failing test first, see the failure. Rows that characterise shipped behaviour are first made to fail by a deliberate mutation of one expectation, restored with an exact-string replace (never `sed 0,/x/` on macOS).
- Never `--no-verify`; fix lint with `hk fix`. Conventional commits, subject ≤ 72 characters (count it), trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, never `Claude-Session:`. Commits on `main`; do not push.
- `e2e/` imports no heraut `internal/` package. Exit codes are literals: config 2, runtime 3, promotion 4. Error text is matched lower-cased and whitespace-collapsed.
- The CHANGELOG section date is the newest commit's date (real time), so outlines compare versions and subjects only, never dates or hashes.
- Scope: ADR-0065 behaviour reachable without a remote or a forge. `changelog --commit/--tag/--no-push`, hooks and fake `gh`/`glab` are T345b4. Do not assert exit codes that the spec does not pin (T359 covers the per-env branch guard, not this one: ADR-0065 pins Config for branch-rule errors and Runtime for resolve-time errors).
- Every expected value below was produced by the real binary on 2026-10-09. If a row fails, investigate before touching the expectation.

## Review Focus

1. A `feat:` on a patch-only line must fail with the range in the message, never release outside the line.
2. A tag cut on another branch (including a build-metadata tag of the same version) must refuse the next version; a build-metadata tag on the line itself must not.
3. Unlisted or unknown branches preview normally but are refused for publishing; `--dry-run` is not refused and creates nothing.
4. A tag not merged into HEAD must get no changelog section; after a forward merge it must.
5. `--set-version` must stay the escape hatch: it works on a branch whose range cannot be derived, and still refuses a taken version.

---

### Task 1: Harness git helpers

**Files:**
- Modify: `e2e/harness/repo.go`
- Modify: `e2e/harness/harness_test.go`

**Interfaces:**
- Consumes: `Repo.git` (T345a).
- Produces: `(*Repo).Switch(branch string)`, `(*Repo).Detach()`, `(*Repo).MergeNoFF(branch string)`.

- [ ] **Step 1: Write the failing test**

Append to `e2e/harness/harness_test.go`:

```go
func TestRepo_SwitchDetachAndMerge(t *testing.T) {
	r := NewRepo(t)
	r.Commit("feat: first")
	r.Checkout("release/1.3")
	r.Commit("fix: on the line")
	r.Switch("main")

	assert.Equal(t, "main", strings.TrimSpace(r.git("rev-parse", "--abbrev-ref", "HEAD")))

	r.MergeNoFF("release/1.3")
	assert.Equal(t, "chore: merge release/1.3", strings.TrimSpace(r.git("log", "-1", "--format=%s")))
	assert.Len(t, strings.Fields(r.git("log", "-1", "--format=%P")), 2, "a no-ff merge has two parents")

	r.Detach()
	assert.Equal(t, "HEAD", strings.TrimSpace(r.git("rev-parse", "--abbrev-ref", "HEAD")))
}
```

Run: `go test ./e2e/harness -run TestRepo_SwitchDetachAndMerge`
Expected: FAIL to compile, `r.Switch undefined`.

- [ ] **Step 2: Implement**

Append to `e2e/harness/repo.go`:

```go
// Switch moves to an existing branch.
func (r *Repo) Switch(branch string) {
	r.t.Helper()
	r.git("checkout", "-q", branch)
}

// Detach detaches HEAD at its current commit.
func (r *Repo) Detach() {
	r.t.Helper()
	r.git("checkout", "-q", "--detach")
}

// MergeNoFF merges branch into the current branch with a merge commit.
func (r *Repo) MergeNoFF(branch string) {
	r.t.Helper()
	r.git("merge", "-q", "--no-ff", "-m", "chore: merge "+branch, branch)
}
```

Run: `go test -count=1 ./e2e/harness`
Expected: PASS.

- [ ] **Step 3: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e/harness
git commit -F - <<'EOF'
test(e2e): add Switch, Detach and MergeNoFF to the harness

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Branch steps in the runner, and version-resolution scenarios

**Files:**
- Modify: `e2e/scenario_test.go`
- Create: `e2e/maintenance_test.go`

**Interfaces:**
- Consumes: Task 1 helpers; `scenario`, `runScenarios`, `commit`, `tag`, `versionNext`, `exitConfig/exitRuntime`, `exitAnyFailure` (T345b2).
- Produces: `step.checkout/switchTo/merge/detach` fields and constructors `checkout(name)`, `switchTo(name)`, `mergeNoFF(name)`, `detach()`; `applySteps(t, repo, steps)`; `lineHistory(line string, extra ...step) []step`; `branchesCfg`.

- [ ] **Step 1: Extend `step` and factor the replay**

In `e2e/scenario_test.go`:

1. Replace `type step struct{ commit, tag string }` with:

```go
type step struct{ commit, tag, checkout, switchTo, merge string; detach bool }
```

2. After `func tag(...)` add:

```go
func checkout(name string) step { return step{checkout: name} }
func switchTo(name string) step { return step{switchTo: name} }
func mergeNoFF(name string) step { return step{merge: name} }
func detach() step               { return step{detach: true} }

func applySteps(repo *harness.Repo, steps []step) {
	for _, s := range steps {
		switch {
		case s.tag != "":
			repo.Tag(s.tag)
		case s.checkout != "":
			repo.Checkout(s.checkout)
		case s.switchTo != "":
			repo.Switch(s.switchTo)
		case s.merge != "":
			repo.MergeNoFF(s.merge)
		case s.detach:
			repo.Detach()
		default:
			repo.Commit(s.commit)
		}
	}
}
```

3. In `runScenarios`, replace the `for _, s := range tc.history { ... }` loop with `applySteps(repo, tc.history)`.

Run: `go test -count=1 ./e2e` (then `hk fix` for alignment).
Expected: PASS, unchanged behaviour.

- [ ] **Step 2: Write the resolution scenarios**

Create `e2e/maintenance_test.go`:

```go
package e2e_test

import (
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

const branchesCfg = `version: "1"
versioning:
  strategy: semver
  branches:
    - name: main
    - name: release/*
`

// lineHistory builds: main has v1.3.0 then a line branch is cut there, then main moves on to
// v1.4.0, and the repo ends up checked out on the line with extra applied.
func lineHistory(line string, extra ...step) []step {
	base := []step{
		commit("feat: a"), tag("v1.3.0"),
		checkout(line),
		switchTo("main"),
		commit("feat: b"), tag("v1.4.0"),
		switchTo(line),
	}
	return append(base, extra...)
}

func TestMaintenance_Resolution(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "a fix on the line releases the next patch of the line", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")), args: versionNext, wantOut: "v1.3.1"},
		{name: "version current is the line's base, not main's latest", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")),
			args:    []string{"version", "current"}, wantOut: "v1.3.0"},
		{name: "a feat on a patch-only line is refused with the range", config: branchesCfg,
			history:  lineHistory("release/1.3", commit("feat: y")),
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"outside the maintenance range", "release/1.3 (>=1.3.0 <1.4.0)"}},
		{name: "a minor-capable line refuses a version cut on another branch", config: branchesCfg,
			history:  lineHistory("release/1.x", commit("feat: y")),
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"tag already exists: v1.4.0 (cut on another branch)"}},
		{name: "a line without a release in range cannot resolve", config: branchesCfg,
			history:  lineHistory("release/2.0", commit("fix: x")),
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"no release in range 2.0.x in the history of release/2.0"}},
		{name: "version current on a line without a release in range fails too", config: branchesCfg,
			history:  lineHistory("release/2.0", commit("fix: x")),
			args:     []string{"version", "current"},
			wantExit: exitRuntime, wantText: []string{"no tags found in range 2.0.x reachable from release/2.0"}},
		{name: "main keeps resolving from its own history", config: branchesCfg,
			history: lineHistory("release/1.3", switchTo("main"), commit("fix: m")),
			args:    versionNext, wantOut: "v1.4.1"},
		{name: "an unlisted branch previews like today", config: branchesCfg,
			history: lineHistory("feature/x", commit("fix: x")), args: versionNext, wantOut: "v1.4.1"},
		{name: "a pre-release on the line stays in range", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")),
			args:    []string{"version", "next", "--pre-release", "rc"}, wantOut: "v1.3.1-rc.1"},
		{name: "a build-metadata tag cut on the line itself is its release", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x"), tag("v1.3.1+7"), commit("fix: y")),
			args:    versionNext, wantOut: "v1.3.2"},
		{name: "a build-metadata tag of the same version cut elsewhere refuses", config: branchesCfg,
			history: lineHistory("release/1.3", switchTo("main"), commit("fix: m"), tag("v1.3.1+7"),
				switchTo("release/1.3"), commit("fix: x")),
			args:     versionNext,
			wantExit: exitRuntime, wantText: []string{"tag already exists: v1.3.1+7"}},
	})
}

func TestMaintenance_BranchRuleErrors(t *testing.T) {
	bin := harness.Binary(t)
	ambiguous := `version: "1"
versioning:
  strategy: semver
  branches:
    - name: release/*
    - name: release/1.3
      range: 1.3.x
`
	runScenarios(t, bin, nil, []scenario{
		{name: "two matching entries are a config error", config: ambiguous,
			history:  lineHistory("release/1.3", commit("fix: x")),
			args:     versionNext,
			wantExit: exitConfig, wantText: []string{"matches more than one versioning.branches entry"}},
		{name: "a glob match with no derivable range is a config error", config: branchesCfg,
			history:  lineHistory("release/foo", commit("fix: x")),
			args:     versionNext,
			wantExit: exitConfig, wantText: []string{"cannot derive a maintenance range from the branch name", "add range: to the entry"}},
		{name: "--set-version stays the escape hatch on such a branch", config: branchesCfg,
			history: lineHistory("release/foo", commit("fix: x")),
			args:    []string{"version", "next", "--set-version", "9.9.9"}, wantOut: "v9.9.9"},
		{name: "branches under calver is a config error", config: `version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  branches:
    - name: main
`,
			history:  []step{commit("feat: a")},
			args:     versionNext,
			wantExit: exitConfig, wantText: []string{"only valid with strategy: semver"}},
	})
}

func TestMaintenance_SetVersionCollisions(t *testing.T) {
	bin := harness.Binary(t)
	runScenarios(t, bin, nil, []scenario{
		{name: "a taken version is refused", config: branchesCfg,
			history:  lineHistory("release/1.3", commit("fix: x")),
			args:     []string{"version", "next", "--set-version", "1.4.0"},
			wantExit: exitRuntime, wantText: []string{"tag already exists: v1.4.0", "pick a free version for --set-version"}},
		{name: "a free version is accepted", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")),
			args:    []string{"version", "next", "--set-version", "1.3.1"}, wantOut: "v1.3.1"},
		{name: "a free version with a build id is accepted", config: branchesCfg,
			history: lineHistory("release/1.3", commit("fix: x")),
			args:    []string{"version", "next", "--set-version", "1.3.1", "--set-build-id", "5"}, wantOut: "v1.3.1+5"},
	})
}

func TestMaintenance_DetachedHeadUsesCIVariables(t *testing.T) {
	bin := harness.Binary(t)
	history := lineHistory("release/1.3", commit("fix: x"), detach())
	runScenarios(t, bin, nil, []scenario{
		{name: "no CI variable: the branch is unknown and previews like today", config: branchesCfg,
			history: history, args: versionNext, wantOut: "v1.4.1"},
		{name: "CI_COMMIT_BRANCH names the line", config: branchesCfg, history: history,
			args: versionNext, env: []string{"CI_COMMIT_BRANCH=release/1.3"}, wantOut: "v1.3.1"},
		{name: "BUILD_SOURCEBRANCH names the line", config: branchesCfg, history: history,
			args: versionNext, env: []string{"BUILD_SOURCEBRANCH=refs/heads/release/1.3"}, wantOut: "v1.3.1"},
		{name: "GITHUB_REF_NAME names the line when it is a branch ref", config: branchesCfg, history: history,
			args: versionNext, env: []string{"GITHUB_REF_NAME=release/1.3", "GITHUB_REF_TYPE=branch"}, wantOut: "v1.3.1"},
		{name: "GITHUB_REF_NAME is ignored for a tag ref", config: branchesCfg, history: history,
			args: versionNext, env: []string{"GITHUB_REF_NAME=release/1.3", "GITHUB_REF_TYPE=tag"}, wantOut: "v1.4.1"},
	})
}

func TestMaintenance_ReleaseRefusesUnlistedBranches(t *testing.T) {
	bin := harness.Binary(t)
	cfg := branchesCfg + `forges:
  - name: github
    platform: github
    repository: acme/widget
release:
  targets:
    - forge: github
`
	runScenarios(t, bin, nil, []scenario{
		{name: "release on an unlisted branch is refused before anything is written", config: cfg,
			history:  lineHistory("feature/x", commit("fix: x")),
			args:     []string{"release", "--set-version", "1.2.3"},
			wantExit: exitConfig, wantText: []string{`branch matches no versioning.branches entry: branch "feature/x"`, "pass --force to release anyway"}},
	})
}
```

The `--dry-run` preview is a standalone test (it prints several step lines, so it does not fit the single-output row shape). Add it right after `TestMaintenance_ReleaseRefusesUnlistedBranches` (and add `"github.com/stretchr/testify/assert"` and `"github.com/stretchr/testify/require"` to the file's imports now, they are used here and in Task 3):

```go
func TestMaintenance_DryRunIsNotRefused(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.WriteConfig(branchesCfg + `forges:
  - name: github
    platform: github
    repository: acme/widget
release:
  targets:
    - forge: github
`)
	applySteps(repo, lineHistory("feature/x", commit("fix: x")))

	res := repo.Run(bin, nil, "release", "--set-version", "1.2.3", "--dry-run")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout, "[dry-run] would tag")
	assert.NotContains(t, normalize(res.Stdout+" "+res.Stderr), "pass --force")
}
```

- [ ] **Step 3: Run; prove the table can fail**

Run: `go test -count=1 ./e2e -run 'TestMaintenance_'`
Expected: PASS (these characterise shipped behaviour). Then mutate: exact-replace `wantOut: "v1.3.1"},` (first occurrence only; verify `strings.Count == 1` in Python) with `wantOut: "v1.3.2"},`, run `-run TestMaintenance_Resolution`, confirm the failure shows `actual  : "v1.3.1"`, restore with an exact replace and `grep -c 'v1.3.2"},' e2e/maintenance_test.go` must be 1 (the legitimate build-metadata row).

- [ ] **Step 4: Lint and commit**

Run: `go test -count=1 ./... && hk fix && hk check`

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): cover maintenance-branch resolution and branch rules

Line resolution (patch on the line, out-of-range feat, collisions with a
tag cut elsewhere including build metadata, no in-range base), main and
unlisted branches, ambiguous/underivable/calver config errors, the
--set-version escape hatch and its collision probe, detached-HEAD
detection from CI variables, and the unlisted-branch release refusal.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: History-aware changelog bounds

**Files:**
- Modify: `e2e/maintenance_test.go`

**Interfaces:**
- Consumes: `applySteps`, `lineHistory`-style steps, `mergeNoFF`, `harness.NewRepo/ReadFile/Run`.
- Produces: `changelogOutline(content string) []string`.

- [ ] **Step 1: Write the failing test (outline helper does not exist yet)**

Append to `e2e/maintenance_test.go` (add `"strings"` and `"github.com/stretchr/testify/assert"` / `"github.com/stretchr/testify/require"` to its imports):

```go
const changelogCfg = `version: "1"
versioning:
  strategy: semver
  branches:
    - name: main
    - name: release/*
changelog:
  output: CHANGELOG.md
`

// changelogOutline reduces a CHANGELOG.md to "version: subject, subject" lines, in file order,
// dropping dates and hashes (both depend on when and where the test ran).
func changelogOutline(content string) []string {
	var outline []string
	var version string
	var subjects []string
	flush := func() {
		if version != "" {
			outline = append(outline, version+": "+strings.Join(subjects, ", "))
		}
	}
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "## ["):
			flush()
			version = strings.TrimPrefix(line, "## [")
			version = version[:strings.Index(version, "]")]
			subjects = nil
		case strings.HasPrefix(line, "- ") && version != "":
			subject := strings.TrimPrefix(line, "- ")
			if i := strings.LastIndex(subject, " - "); i >= 0 {
				subject = subject[:i]
			}
			subjects = append(subjects, subject)
		}
	}
	flush()
	return outline
}

func TestChangelogOutline(t *testing.T) {
	got := changelogOutline("# Changelog\n\n## [1.4.0] - 2026-10-09\n\n### Features\n\n- B1 - abc123\n- B2 - def456\n\n## [1.3.0] - 2026-10-09\n\n- A1 - 0a1b2c\n")

	assert.Equal(t, []string{"1.4.0: B1, B2", "1.3.0: A1"}, got)
}
```

Run: `go test ./e2e -run TestChangelogOutline`
Expected: FAIL to compile first if the helper is missing; once the code above is in the file it passes. Write the test block first, run it to see `undefined: changelogOutline`, then add the helper.

- [ ] **Step 2: Add the scenarios**

Append:

```go
func TestMaintenance_ChangelogBounds(t *testing.T) {
	bin := harness.Binary(t)
	regenerate := func(t *testing.T, steps []step, version string) []string {
		t.Helper()
		repo := harness.NewRepo(t)
		repo.WriteConfig(changelogCfg)
		applySteps(repo, steps)
		res := repo.Run(bin, nil, "changelog", "--regenerate", "--offline", "--set-version", version)
		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		return changelogOutline(repo.ReadFile("CHANGELOG.md"))
	}
	// main: a1 (v1.3.0), then the line release/1.3 gets two fixes tagged v1.3.1 and v1.3.2, and main
	// continues with two features tagged v1.4.0.
	layout := []step{
		commit("feat: a1"), tag("v1.3.0"),
		checkout("release/1.3"), commit("fix: f1"), tag("v1.3.1"), commit("fix: f2"), tag("v1.3.2"),
		switchTo("main"), commit("feat: b1"), commit("feat: b2"), tag("v1.4.0"),
	}

	t.Run("tags never merged into HEAD get no section", func(t *testing.T) {
		got := regenerate(t, layout, "1.5.0")

		assert.Equal(t, []string{"1.4.0: B1, B2", "1.3.0: A1"}, got)
	})

	t.Run("after a forward merge each tag gets its own section", func(t *testing.T) {
		got := regenerate(t, append(append([]step{}, layout...), mergeNoFF("release/1.3"), commit("fix: after")), "1.5.0")

		assert.Equal(t, []string{
			"1.5.0: F1, F2, After, Merge forward",
			"1.4.0: B1, B2",
			"1.3.2: F2",
			"1.3.1: F1",
			"1.3.0: A1",
		}, got)
	})

	t.Run("two tags on one commit bound each other, the empty section is dropped", func(t *testing.T) {
		got := regenerate(t, []step{
			commit("feat: a"), tag("v1.0.0"), commit("fix: b"), tag("v1.0.1"), tag("v1.0.2"), commit("fix: c"),
		}, "1.0.3")

		assert.Equal(t, []string{"1.0.3: C", "1.0.1: B", "1.0.0: A"}, got)
	})
}
```

Run: `go test -count=1 ./e2e -run 'TestMaintenance_ChangelogBounds|TestChangelogOutline'`
Expected: PASS. Prove it can fail: exact-replace `"1.3.2: F2",` with `"1.3.2: F1, F2",`, run, confirm the failure shows the actual `1.3.2: F2`, restore, `grep -c '"1.3.2: F1, F2"' e2e/maintenance_test.go` must be 0.

Note: the forward-merge row documents ADR-0065's "forward merges list the fix twice": `1.5.0` legitimately repeats `F1, F2` because they are in that section's history.

- [ ] **Step 3: Lint and commit**

Run: `go test -count=1 ./... && hk fix && hk check`

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): cover history-aware changelog bounds on maintenance lines

Tags never merged into HEAD get no section; after a forward merge each
tag gets its own, with the documented repeat of the merged-in fixes in
the next main section; two tags on one commit bound each other.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Close T345b3

**Files:**
- Modify: `docs/tasks/roadmap.md`

- [ ] **Step 1: Flip the bullet and write the note**

Change `- \`[ ]\` **T345b3**:` to `- \`[x]\` **T345b3**:` and add a completion note indented under it with real facts: new scenario count (`go test -count=1 -v ./e2e 2>&1 | grep -c -- '--- PASS: .*/'` minus 113), the harness additions (`Switch`, `Detach`, `MergeNoFF`), the new `step` branch operations, that the dry-run refusal check is a standalone test, and any deviation. Update the Phase 60 status row to include T345b3. `grep -n 'T345b3' docs/tasks/roadmap.md` must show the `[x]` bullet.

- [ ] **Step 2: Verify and commit**

Run: `go test -count=1 ./... && hk check`

```bash
git add docs/tasks/roadmap.md
git commit -F - <<'EOF'
docs(roadmap): close T345b3

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

## Self-review

**Spec coverage** (Lane A "Maintenance branches (ADR-0065)": `release/1.3` resolves inside its range; an unlisted branch refused except `--force`/`--dry-run`; collision guard on `--set-version`; same-commit tags bound each other in the changelog): range resolution (Task 2), unlisted refusal and dry-run (Task 2; `--force` is not run because it would proceed to push and publish, which is T345b4's fake-binary territory), collision guard (Task 2), same-commit tags and the rest of the bounds (Task 3).

**Type consistency:** `step` fields and constructors are defined in Task 2 and used in Tasks 2-3; `applySteps` replaces the loop in `runScenarios`; `changelogOutline` is defined before its tests; `Switch/Detach/MergeNoFF` (Task 1) are used only through `applySteps`.

**Placeholders:** the Task 4 note is written from measured counts at execution time. The Task 2 note about the dry-run row names the exact replacement (a standalone test), so the implementer deletes the second `ReleaseRefusesUnlistedBranches` row and writes `TestMaintenance_DryRunIsNotRefused` instead.
