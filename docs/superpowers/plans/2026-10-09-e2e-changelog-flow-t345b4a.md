# E2E changelog flow with a bare remote (T345b4a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cover `heraut changelog` end to end against a real (local, bare) remote: the file written, `--commit`, `--tag`, `--no-push`, branch and tag actually arriving on the remote, annotated vs lightweight tags, incremental splice vs `--regenerate`, a re-run that stages nothing, pre-releases never writing the changelog, a tag-only run when an environment disables the changelog, a failing push, and `--offline` with a required enrichment policy.

**Architecture:** Harness gains a bare-remote helper and a few git conveniences. Scenarios are standalone tests (each needs post-run repo and remote assertions that the table runner does not express), sharing a `flowRepo` constructor. No production code changes.

**Tech Stack:** Go 1.27, testify, the e2e harness.

**Spec:** `docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md` (Lane A "Changelog and release flow, local publish"); behaviour source `docs/specs/03-commands.md` § `heraut changelog`, ADR-0012, ADR-0038.

## Global Constraints

- TDD: failing test first, see it fail. Characterising rows are first proven able to fail by a deliberate mutation of one expectation, restored from a backup copy or an exact-string replace (never `sed 0,/x/` on macOS).
- Never `--no-verify`; fix lint with `hk fix`. Conventional commits, subject ≤ 72 characters (count it), trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, never `Claude-Session:`. Commits on `main`; do not push.
- `e2e/` imports no heraut `internal/` package. Exit codes are literals: config 2, runtime 3. Error text is matched lower-cased and whitespace-collapsed (`normalize`).
- The remote is a bare repository under `t.TempDir()`; nothing touches the network (the harness already sets `HERAUT_CHECK_UPDATE=false`). Every test passes `--offline` unless it is about `--offline`.
- Section dates and commit hashes vary per run: compare `changelogOutline` lines (version and subjects), never dates or hashes. The outline helper already exists in `e2e/maintenance_test.go`.
- Scope: `changelog` flows only. Hooks are T345b4b; the full `release` with fake `gh`/`glab` is T345b4c. Do not run `release` here.
- Every expected value below was produced by the real binary on 2026-10-09. If a row fails, investigate before touching the expectation.

## Review Focus

1. `--no-push` must leave the remote untouched (no commit, no tag), while the default must deliver both.
2. The tag must point at the changelog commit, not the commit before it.
3. A re-run whose changelog is identical must skip the commit with a warning and still exit 0, not fail on git's "nothing to commit".
4. A hand edit inside an old section must survive an incremental release and be dropped by `--regenerate`.
5. A pre-release must not write or commit `CHANGELOG.md` yet must still tag and push.

---

### Task 1: Harness remote and git helpers

**Files:**
- Modify: `e2e/harness/repo.go`
- Modify: `e2e/harness/harness_test.go`

**Interfaces:**
- Consumes: `Repo.git`, `Repo.t`, `Repo.Dir`, `baseEnv` (earlier tasks).
- Produces: `(*Repo).AddRemote()` (creates a bare repo, adds it as `origin`, pushes the current branch with upstream), `(*Repo).RemoveRemote()`, `(*Repo).Git(args ...string) string` (exported `git` run in the repo, trimmed output), `(*Repo).GitRemote(args ...string) string` (git run against the bare remote, trimmed output), `(*Repo).WriteFile(rel, content string)`.

- [ ] **Step 1: Write the failing test**

Append to `e2e/harness/harness_test.go`:

```go
func TestRepo_RemoteAndFileHelpers(t *testing.T) {
	r := NewRepo(t)
	r.Commit("feat: first")
	r.AddRemote()

	assert.Equal(t, r.Git("rev-parse", "HEAD"), r.GitRemote("rev-parse", "main"), "AddRemote pushes the branch")
	assert.Equal(t, "origin/main", r.Git("rev-parse", "--abbrev-ref", "main@{upstream}"), "the branch tracks origin")

	r.Tag("v1.0.0")
	r.Git("push", "origin", "v1.0.0")
	assert.Equal(t, "v1.0.0", r.GitRemote("tag", "-l"))

	r.WriteFile("docs/NOTES.md", "hello\n")
	assert.Equal(t, "hello\n", r.ReadFile("docs/NOTES.md"))

	r.RemoveRemote()
	_, err := os.Stat(r.remote)
	assert.True(t, os.IsNotExist(err), "the bare remote is gone")
}
```

Add `"os"` to that file's imports. Run: `go test ./e2e/harness -run TestRepo_RemoteAndFileHelpers`
Expected: FAIL to compile, `r.AddRemote undefined`.

- [ ] **Step 2: Implement**

In `e2e/harness/repo.go`, add a `remote string` field to `Repo` and append:

```go
// AddRemote creates a bare repository, wires it as origin and pushes the current branch with
// upstream tracking. The repository needs at least one commit.
func (r *Repo) AddRemote() {
	r.t.Helper()
	r.remote = filepath.Join(r.t.TempDir(), "origin.git")
	cmd := exec.Command("git", "init", "-q", "--bare", r.remote)
	cmd.Env = baseEnv(r.home)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	r.git("remote", "add", "origin", r.remote)
	r.git("push", "-q", "-u", "origin", "main")
}

// RemoveRemote deletes the bare remote so a later push fails.
func (r *Repo) RemoveRemote() {
	r.t.Helper()
	if err := os.RemoveAll(r.remote); err != nil {
		r.t.Fatalf("removing the remote: %v", err)
	}
}

// Git runs git in the repository and returns its trimmed output.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return strings.TrimSpace(r.git(args...))
}

// GitRemote runs git against the bare remote and returns its trimmed output.
func (r *Repo) GitRemote(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"--git-dir", r.remote}, args...)...)
	cmd.Env = baseEnv(r.home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git (remote) %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// WriteFile writes content to a path relative to the repository root, creating directories.
func (r *Repo) WriteFile(rel, content string) {
	r.t.Helper()
	path := filepath.Join(r.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatalf("creating directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatalf("writing %s: %v", rel, err)
	}
}
```

Add `"strings"` to `repo.go`'s imports. Run: `go test -count=1 ./e2e/harness`
Expected: PASS.

- [ ] **Step 3: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e/harness
git commit -F - <<'EOF'
test(e2e): add a bare remote and git helpers to the harness

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Changelog flow scenarios

**Files:**
- Create: `e2e/changelog_flow_test.go`

**Interfaces:**
- Consumes: Task 1 helpers; `harness.Binary/NewRepo/Run/WriteConfig/ReadFile`, `commit/tag`-free helpers (`Repo.Commit`, `Repo.Tag`), `normalize`, `changelogOutline`, `exitOK/exitRuntime`.
- Produces: `flowRepo(t, cfg string) *harness.Repo`, `flowCfg`.

- [ ] **Step 1: Write the first scenario (red: the file does not exist)**

Create `e2e/changelog_flow_test.go`:

```go
package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

const flowCfg = `version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
changelog:
  output: CHANGELOG.md
`

// flowRepo is a repository with one commit, the given config (untracked) and a bare origin that
// already has main.
func flowRepo(t *testing.T, cfg string) *harness.Repo {
	t.Helper()
	repo := harness.NewRepo(t)
	repo.WriteConfig(cfg)
	repo.Commit("chore: init")
	repo.AddRemote()
	return repo
}

func runOK(t *testing.T, repo *harness.Repo, bin string, args ...string) harness.Result {
	t.Helper()
	res := repo.Run(bin, nil, args...)
	require.Equal(t, exitOK, res.ExitCode, "heraut %v\nstdout:\n%s\nstderr:\n%s", args, res.Stdout, res.Stderr)
	return res
}

func TestChangelogFlow_CommitTagAndPush(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, "chore(release): 0.1.0", repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, "tag", repo.Git("cat-file", "-t", "v0.1.0"), "tags are annotated by default")
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.Git("rev-list", "-n", "1", "v0.1.0"),
		"the tag points at the changelog commit")
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "the tag reached the remote")
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.GitRemote("rev-parse", "main"), "the branch reached the remote")
	assert.Equal(t, []string{"0.1.0: One, Init"}, changelogOutline(repo.ReadFile("CHANGELOG.md")))
}
```

Run: `go test -count=1 ./e2e -run TestChangelogFlow_CommitTagAndPush`
Expected: PASS once the file exists. To see it can fail, temporarily replace the exact string `"chore(release): 0.1.0"` with `"chore(release): 9.9.9"`, run, confirm the failure shows `chore(release): 0.1.0`, and restore from a backup copy made before the mutation.

- [ ] **Step 2: Add the remaining flows**

Append to `e2e/changelog_flow_test.go`:

```go
func TestChangelogFlow_NoPushKeepsTheRemoteUntouched(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	remoteHead := repo.GitRemote("rev-parse", "main")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--no-push", "--offline")

	assert.Equal(t, "chore(release): 0.1.0", repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, "v0.1.0", repo.Git("tag", "-l"))
	assert.Equal(t, remoteHead, repo.GitRemote("rev-parse", "main"), "the remote branch is untouched")
	assert.Empty(t, repo.GitRemote("tag", "-l"), "the remote has no tag")
}

func TestChangelogFlow_WithoutFlagsOnlyWritesTheFile(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	head := repo.Git("rev-parse", "HEAD")

	runOK(t, repo, bin, "changelog", "--offline")

	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "no commit")
	assert.Empty(t, repo.Git("tag", "-l"), "no tag")
	assert.Contains(t, repo.Git("status", "--porcelain"), "?? CHANGELOG.md")
	assert.Equal(t, []string{"0.1.0: One, Init"}, changelogOutline(repo.ReadFile("CHANGELOG.md")))
}

func TestChangelogFlow_TagImpliesCommitAndPush(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--tag", "--offline")

	assert.Equal(t, "chore(release): 0.1.0", repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"))
}

func TestChangelogFlow_CommitWithoutTag(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--offline", "--set-version", "0.1.0")

	assert.Empty(t, repo.Git("tag", "-l"))
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), repo.GitRemote("rev-parse", "main"), "the commit was pushed")
}

func TestChangelogFlow_AnIdenticalRerunSkipsTheCommit(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	runOK(t, repo, bin, "changelog", "--commit", "--offline", "--set-version", "0.1.0")
	head := repo.Git("rev-parse", "HEAD")

	res := runOK(t, repo, bin, "changelog", "--commit", "--offline", "--set-version", "0.1.0")

	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "nothing new is committed")
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "nothing to commit")
}

func TestChangelogFlow_IncrementalSpliceKeepsHandEditsRegenerateDropsThem(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	edited := strings.Replace(repo.ReadFile("CHANGELOG.md"), "- One - ", "- One (edited by hand) - ", 1)
	require.NotEqual(t, repo.ReadFile("CHANGELOG.md"), edited, "the hand edit must change the file")
	repo.WriteFile("CHANGELOG.md", edited)
	repo.Git("commit", "-q", "-am", "docs: hand edit")
	repo.Commit("fix: two")
	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, []string{"0.1.1: Two, Hand edit", "0.1.0: One (edited by hand), Init"},
		changelogOutline(repo.ReadFile("CHANGELOG.md")), "an incremental release splices only the new section")

	repo.Commit("fix: three")
	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline", "--regenerate")

	assert.Equal(t, []string{"0.1.2: Three", "0.1.1: Two, Hand edit", "0.1.0: One, Init"},
		changelogOutline(repo.ReadFile("CHANGELOG.md")), "--regenerate rebuilds every section")
}

func TestChangelogFlow_LightweightTags(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, `version: "1"
versioning:
  strategy: semver
  tag_type: lightweight
changelog:
  output: CHANGELOG.md
`)
	repo.Commit("feat: one")

	runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline")

	assert.Equal(t, "commit", repo.Git("cat-file", "-t", "v0.1.0"), "a lightweight tag is a bare ref")
	assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"))
}

func TestChangelogFlow_PreReleaseTagsWithoutTouchingTheChangelog(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	head := repo.Git("rev-parse", "HEAD")

	res := runOK(t, repo, bin, "changelog", "--commit", "--tag", "--offline", "--set-version", "1.0.0-rc.1")

	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "pre-release: changelog.md not updated")
	assert.NoFileExists(t, filepath.Join(repo.Dir, "CHANGELOG.md"))
	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "no changelog commit")
	assert.Equal(t, "v1.0.0-rc.1", repo.GitRemote("tag", "-l"), "the tag is still pushed")
}

func TestChangelogFlow_DisabledChangelogStillTags(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, `version: "1"
versioning:
  strategy: semver-per-env
changelog:
  output: CHANGELOG.md
environments:
  dev:
    tag_format: "dev/{version}"
    bump: auto
    disable_changelog: true
`)
	repo.Commit("feat: one")
	head := repo.Git("rev-parse", "HEAD")

	res := runOK(t, repo, bin, "changelog", "--tag", "--env", "dev", "--offline")

	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "changelog disabled")
	assert.NoFileExists(t, filepath.Join(repo.Dir, "CHANGELOG.md"))
	assert.Equal(t, head, repo.Git("rev-parse", "HEAD"))
	assert.Equal(t, "dev/0.1.0", repo.GitRemote("tag", "-l"))
}

func TestChangelogFlow_AFailingPushIsARuntimeError(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	repo.RemoveRemote()

	res := repo.Run(bin, nil, "changelog", "--commit", "--tag", "--offline")

	require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "does not appear to be a git repository")
	assert.Empty(t, repo.Git("tag", "-l"), "the tag is only created after the commit was pushed")
}

func TestChangelogFlow_OfflineLiftsARequiredEnrichmentPolicy(t *testing.T) {
	bin := harness.Binary(t)
	cfg := `version: "1"
versioning:
  strategy: semver
changelog:
  output: CHANGELOG.md
commits:
  enrichment_policy: required
`
	t.Run("without --offline a required policy with no forge fails", func(t *testing.T) {
		repo := flowRepo(t, cfg)
		repo.Commit("feat: one")

		res := repo.Run(bin, nil, "changelog")

		require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "remote enrichment (required): no forge resolved")
	})
	t.Run("--offline forces the policy to disabled", func(t *testing.T) {
		repo := flowRepo(t, cfg)
		repo.Commit("feat: one")

		runOK(t, repo, bin, "changelog", "--offline")

		assert.FileExists(t, filepath.Join(repo.Dir, "CHANGELOG.md"))
	})
}
```

Remove the unused `"os"` import if `hk` flags it (it is only needed if a test reads a file by path).

Run: `go test -count=1 ./e2e -run TestChangelogFlow_`
Expected: PASS. If a row fails, copy the failure output and stop: do not edit the expectation before understanding whether heraut regressed. Then prove the tests can fail: back up the file, exact-replace `"0.1.1: Two, Hand edit", "0.1.0: One (edited by hand), Init"` with `"0.1.1: Two, Hand edit", "0.1.0: One, Init"`, run `-run IncrementalSplice`, confirm the failure shows the `(edited by hand)` text, restore from the backup, and confirm `git diff` shows no mutation left.

- [ ] **Step 3: Full suite, lint, commit**

Run: `go test -count=1 ./... && hk fix && hk check`

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): cover the changelog commit, tag and push flow on a bare remote

Commit and tag delivery to the remote, --no-push, --tag implying commit,
the nothing-to-commit skip, incremental splice vs --regenerate,
lightweight tags, pre-releases and disabled changelogs that still tag,
a failing push, and --offline with a required enrichment policy.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Close T345b4a and split T345b4 in the roadmap

**Files:**
- Modify: `docs/tasks/roadmap.md`
- Modify: `.claude/rules/coding.md`

- [ ] **Step 1: Replace the T345b4 bullet with its three parts**

In the T345b list, replace the `- \`[ ]\` **T345b4**:` bullet (all its lines up to the T345b5 bullet) with:

```markdown
- `[ ]` **T345b4**: changelog and release local flow, split into three parts:
  - `[x]` **T345b4a**: `changelog` flows against a bare remote (commit, tag, push, `--no-push`,
    splice vs `--regenerate`, pre-release and disabled-changelog tag-only runs, failing push,
    `--offline`).
  - `[ ]` **T345b4b**: hooks: the six points and their order, `--no-hooks`, `--skip-hook`,
    `HERAUT_SKIP_HOOKS`, a failing hook aborting the run, hook file staging.
  - `[ ]` **T345b4c**: the full `release` with fake `gh`/`glab` recording argv: notes file, release
    flags per forge, asset upload, `--dry-run` calling nothing, `release --force` on an unlisted
    branch.
```

Then add, indented under the T345b4a line, a completion note with real facts: how many `TestChangelogFlow_` tests and subtests ran (`go test -count=1 -v ./e2e -run TestChangelogFlow_ 2>&1 | grep -c -- '--- PASS'`), the harness additions (`AddRemote`, `RemoveRemote`, `Git`, `GitRemote`, `WriteFile`), and any deviation or defect found. Update the Phase 60 status row to include T345b4a. `grep -n 'T345b4a' docs/tasks/roadmap.md` must show the `[x]` bullet and no placeholder text.

- [ ] **Step 2: Add the `e2e/` row to the layer table**

In `.claude/rules/coding.md`, in the "Layer rules" table, add after the `internal/testutil/` row:

```markdown
| `e2e/`               | `e2e/harness` and testify only, never an `internal/` package (it drives the built binary, ADR-0066) |
```

Keep the table's column alignment.

- [ ] **Step 3: Verify and commit**

Run: `go test -count=1 ./... && hk check`

```bash
git add docs/tasks/roadmap.md .claude/rules/coding.md
git commit -F - <<'EOF'
docs(roadmap): close T345b4a and split the rest of T345b4

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

## Self-review

**Spec coverage** (Lane A "Changelog and release flow, local publish": `changelog --commit --tag --no-push` against the bare remote: changelog file content, commit message, annotated tag, history-aware bounds, `--regenerate`, `disable_changelog` per env, hooks and `--skip-hook`, fake `gh`/`glab`, `--offline`): file content, commit message, annotated and lightweight tag, push and `--no-push`, regenerate, disabled changelog, `--offline` (this plan); history-aware bounds (T345b3); hooks (T345b4b); fake `gh`/`glab` (T345b4c).

**Type consistency:** Task 1 helpers (`AddRemote`, `RemoveRemote`, `Git`, `GitRemote`, `WriteFile`) are the only new API and are used only in Task 2 and the harness test; `changelogOutline`, `normalize`, `exitOK`, `exitRuntime` already exist.

**Placeholders:** the Task 3 note is written from measured counts at execution time.
