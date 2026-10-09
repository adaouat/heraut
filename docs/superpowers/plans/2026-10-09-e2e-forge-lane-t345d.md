# E2E forge lane: scenarios B5-B10, workflow, guide (T345d) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish Lane B: scenarios B5 (CalVer), B6 (GitLab source mirrored to GitHub), B7 (draft), B8 (maintenance line with real tags), B9 (PR/MR enrichment on a dedicated sandbox pair) and B10 (dry-run), the nightly/dispatch workflow `.github/workflows/e2e.yml`, and the setup guide `docs/guides/e2e-tests.md`.

**Architecture:** Extends `e2e/forgeharness` (T345c) with `OpenAndMerge` on both forges, a second sandbox pair for enrichment (`HERAUT_E2E_GITHUB_ENRICH_REPO` / `HERAUT_E2E_GITLAB_ENRICH_PROJECT`), `Workspace.AlsoClean` for the cross-forge mirror and `WaitForRelease` for eventually consistent listings. Scenarios go in `e2e/forge_flows_test.go` (same `e2e_forge` tag). No production code change.

**Tech Stack:** Go 1.27, testify, real `gh`/`glab`, GitHub Actions, mise.

**Spec:** `docs/superpowers/specs/2026-10-07-e2e-smoke-suite-design.md` (Lane B scenarios B5-B10, "CI and rules"); ADR-0066; T345c plan and completion note.

## Global Constraints

- **No real data in the repository** (names, hosts, tokens): placeholders such as `<owner>/<name>-testing` in code comments, docs and workflow; real values only in environment variables, repository variables and secrets.
- **Opt-in:** every Lane B Go file has `//go:build e2e_forge`; missing configuration skips, a guard failure fails. The B9 pair is independent: with its variables unset B9 skips and the rest still runs.
- Guards and cleanup rules from T345c apply unchanged: only names carrying the run id are deleted; cleanup failures fail the test; releases are deleted before tags before branches.
- Live runs against the sandboxes are allowed (user-approved): only `e2e-` prefixed resources; verify the sandboxes are empty afterwards. Merged PRs/MRs of B9 cannot be deleted and remain as closed history on the enrichment pair (documented in the guide).
- GitHub's release list and GitHub's commit-to-PR association are eventually consistent (seconds): poll with a bounded timeout, never `sleep` blindly.
- The workflow is a NEW file; do not touch `ci.yml`, `release.yml` or any secret. Pin actions by commit SHA with a version comment, as `ci.yml` does; install `gh`/`glab` versions read from the `Dockerfile` ARGs so they cannot drift.
- TDD: failing test first (offline harness tests under `-tags e2e_forge`); live scenarios are verified by running them.
- Never `--no-verify`; fix lint with `hk fix`. Conventional commits, subject ≤ 72 characters (count it), trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, never `Claude-Session:`. Commits on `main`; do not push.

## Review Focus

1. B6: the mirror's resources (a release and a tag created by `gh release create` on the second forge) must be cleaned up, and only when they carry the run id.
2. B7/B9: eventual consistency must be handled by bounded polling, with a clear failure message on timeout, not by flaky single reads.
3. The workflow must not run on pull requests, must not expose secrets to forks, and must be advisory (no `needs:` from other workflows, no required status).
4. No token or real name in the workflow, guide, tests or logs.
5. B8 must prove the line's range and the collision guard on real clones; B10 must prove nothing was created remotely.

---

### Task 1: Harness extensions

**Files:**
- Modify: `e2e/forgeharness/config.go` (enrich fields), `forge.go` (`OpenAndMerge`), `github.go`, `gitlab.go`, `workspace.go` (`AlsoClean`, `RequireEnrich`, `WaitForRelease`)
- Modify: `e2e/forgeharness/config_test.go`, `forge_test.go`, `workspace_test.go`

**Interfaces:**
- Produces:
  - `Config.GitHubEnrichRepo`, `Config.GitLabEnrichProject` (env `HERAUT_E2E_GITHUB_ENRICH_REPO`, `HERAUT_E2E_GITLAB_ENRICH_PROJECT`); `(Config) ForEnrich(name string) Config` returns a copy whose `GitHubRepo`/`GitLabProject` point at the enrichment pair.
  - `Forge.OpenAndMerge(base, head, title string) (number int, err error)`: opens a pull/merge request and merges it with a merge commit.
  - `RequireEnrich(t, name) Forge` (skips when the enrichment variable is unset, guards like `Require`).
  - `(*Workspace).AlsoClean(f Forge)`: at cleanup also deletes `f`'s releases and tags carrying the run id (no branches).
  - `WaitForRelease(t, f Forge, tag string, timeout time.Duration) Release`: polls `f.Release` every 2 s, fails with a message naming the tag on timeout.

- [ ] **Step 1: Write the failing tests**

Add to `e2e/forgeharness/config_test.go`:

```go
func TestEnrichConfig(t *testing.T) {
	t.Setenv("HERAUT_E2E_GITHUB_REPO", "acme/widget-testing")
	t.Setenv("HERAUT_E2E_GITHUB_ENRICH_REPO", "acme/widget-testing-enrich")
	t.Setenv("HERAUT_E2E_GITLAB_ENRICH_PROJECT", "group/widget-testing-enrich")

	c := LoadConfig()
	e := c.ForEnrich("github")

	assert.Equal(t, "acme/widget-testing-enrich", c.GitHubEnrichRepo)
	assert.Equal(t, "group/widget-testing-enrich", c.GitLabEnrichProject)
	assert.Equal(t, "acme/widget-testing-enrich", e.GitHubRepo, "the enrichment copy targets the second pair")
	assert.Equal(t, "acme/widget-testing", c.GitHubRepo, "the original is untouched")
	assert.Equal(t, "group/widget-testing-enrich", c.ForEnrich("gitlab").GitLabProject)
}
```

Add to `e2e/forgeharness/forge_test.go`:

```go
func TestOpenAndMerge(t *testing.T) {
	gh := fakeAPI(t, "gh", [][2]string{
		{"pulls/7/merge", `{"merged":true}`},
		{"repos/acme/widget-testing/pulls", `{"number":7}`},
	})
	f := NewGitHub(Config{GitHubRepo: "acme/widget-testing", Pattern: "*testing*"}, "tok")

	n, err := f.OpenAndMerge("e2e/x-base", "e2e/x-feat", "feat: add widget (via PR)")

	require.NoError(t, err)
	assert.Equal(t, 7, n)
	all := strings.Join(gh(), "\n")
	assert.Contains(t, all, "api -X POST repos/acme/widget-testing/pulls")
	assert.Contains(t, all, "head=e2e/x-feat")
	assert.Contains(t, all, "base=e2e/x-base")
	assert.Contains(t, all, "api -X PUT repos/acme/widget-testing/pulls/7/merge")
}

func TestGitLabOpenAndMerge(t *testing.T) {
	gl := fakeAPI(t, "glab", [][2]string{
		{"merge_requests/3/merge", `{"state":"merged"}`},
		{"projects/group%2Fwidget-testing/merge_requests?", `{"iid":3,"merge_status":"can_be_merged"}`},
		{"merge_requests/3", `{"iid":3,"merge_status":"can_be_merged"}`},
	})
	f := NewGitLab(Config{GitLabProject: "group/widget-testing", Pattern: "*testing*"}, "tok")

	n, err := f.OpenAndMerge("e2e/x-base", "e2e/x-feat", "feat: add widget (via MR)")

	require.NoError(t, err)
	assert.Equal(t, 3, n)
	all := strings.Join(gl(), "\n")
	assert.Contains(t, all, "api -X POST projects/group%2Fwidget-testing/merge_requests")
	assert.Contains(t, all, "source_branch=e2e/x-feat")
	assert.Contains(t, all, "target_branch=e2e/x-base")
	assert.Contains(t, all, "api -X PUT projects/group%2Fwidget-testing/merge_requests/3/merge")
}

func TestWaitForReleasePollsUntilItAppears(t *testing.T) {
	f := &flakyForge{appearsAfter: 3}

	rel := WaitForRelease(t, f, "tag-1", 30*time.Second)

	assert.Equal(t, "tag-1", rel.Tag)
	assert.Equal(t, 3, f.calls, "it polled until the release showed up")
}
```

Add `"time"` to the imports of `forge_test.go` and define in that file:

```go
type flakyForge struct {
	Forge
	appearsAfter, calls int
}

func (f *flakyForge) Release(tag string) (Release, bool, error) {
	f.calls++
	if f.calls < f.appearsAfter {
		return Release{}, false, nil
	}
	return Release{Tag: tag}, true, nil
}
```

Note: `WaitForRelease` takes a `pollEvery` package variable (default 2 s) that this test sets to a few milliseconds: add `pollEvery = 5 * time.Millisecond` at the top of `TestWaitForReleasePollsUntilItAppears` and restore it with `t.Cleanup`.

Add to `e2e/forgeharness/workspace_test.go`:

```go
func TestAlsoCleanDeletesTheMirrorsRunResourcesOnly(t *testing.T) {
	f := &localForge{url: newBare(t)}
	mirror := &localForge{}
	var runID string

	t.Run("run", func(t *testing.T) {
		w := NewWorkspace(t, f)
		runID = w.RunID
		w.AlsoClean(mirror)
		mirror.tags = []string{w.TagPrefix + "0.1.0", "e2e-1-ffff-v9.9.9", "v1.0.0"}
		mirror.releases = []string{w.TagPrefix + "0.1.0", "e2e-1-ffff-v9.9.9"}
		mirror.branches = []string{w.Branch}
	})

	assert.Equal(t, []string{runID + "-v0.1.0"}, mirror.deletedReleases)
	assert.Equal(t, []string{runID + "-v0.1.0"}, mirror.deletedTags)
	assert.Empty(t, mirror.deletedBranches, "a mirror's branches are never touched")
}
```

Run: `go test -tags e2e_forge ./e2e/forgeharness`
Expected: FAIL to compile (`ForEnrich`, `OpenAndMerge`, `WaitForRelease`, `AlsoClean` undefined).

- [ ] **Step 2: Implement**

`config.go`: add fields `GitHubEnrichRepo`, `GitLabEnrichProject` read from the two new variables in `LoadConfig`, and:

```go
// ForEnrich returns a copy of c whose repository for the named forge is the enrichment pair.
func (c Config) ForEnrich(name string) Config {
	switch name {
	case "github":
		c.GitHubRepo = c.GitHubEnrichRepo
	case "gitlab":
		c.GitLabProject = c.GitLabEnrichProject
	}
	return c
}
```

`forge.go`: add `OpenAndMerge(base, head, title string) (int, error)` to the `Forge` interface.

`github.go`:

```go
func (g *github) OpenAndMerge(base, head, title string) (int, error) {
	out, err := g.call("-X", "POST", "repos/"+g.repo+"/pulls", "-f", "title="+title, "-f", "head="+head, "-f", "base="+base)
	if err != nil {
		return 0, err
	}
	var pr struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return 0, err
	}
	if _, err := g.call("-X", "PUT", fmt.Sprintf("repos/%s/pulls/%d/merge", g.repo, pr.Number), "-f", "merge_method=merge"); err != nil {
		return 0, err
	}
	return pr.Number, nil
}
```

`gitlab.go` (merge requests are mergeable only once GitLab computed `merge_status`; poll briefly):

```go
func (g *gitlab) OpenAndMerge(base, head, title string) (int, error) {
	out, err := g.call("-X", "POST", g.base()+"/merge_requests", "-f", "title="+title, "-f", "source_branch="+head, "-f", "target_branch="+base)
	if err != nil {
		return 0, err
	}
	var mr struct {
		IID int `json:"iid"`
	}
	if err := json.Unmarshal(out, &mr); err != nil {
		return 0, err
	}
	path := fmt.Sprintf("%s/merge_requests/%d", g.base(), mr.IID)
	deadline := time.Now().Add(60 * time.Second)
	for {
		out, err := g.call(path)
		if err != nil {
			return 0, err
		}
		var st struct {
			MergeStatus string `json:"merge_status"`
		}
		_ = json.Unmarshal(out, &st)
		if st.MergeStatus == "can_be_merged" {
			break
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("merge request !%d never became mergeable (status %q)", mr.IID, st.MergeStatus)
		}
		time.Sleep(pollEvery)
	}
	if _, err := g.call("-X", "PUT", path+"/merge"); err != nil {
		return 0, err
	}
	return mr.IID, nil
}
```

(import `time` in `gitlab.go`; `fmt` already.)

`workspace.go`:

```go
var pollEvery = 2 * time.Second

// RequireEnrich is Require for the dedicated enrichment sandbox pair.
func RequireEnrich(t *testing.T, name string) Forge {
	t.Helper()
	c := LoadConfig()
	if (name == "github" && c.GitHubEnrichRepo == "") || (name == "gitlab" && c.GitLabEnrichProject == "") {
		t.Skipf("the enrichment sandbox for %s is not configured (HERAUT_E2E_%s_ENRICH_*): skipping", name, strings.ToUpper(name))
	}
	return requireWith(t, name, c.ForEnrich(name))
}

// WaitForRelease polls f.Release until the release exists (the forges' listings lag behind
// creation) and fails the test naming the tag if it never appears.
func WaitForRelease(t *testing.T, f Forge, tag string, timeout time.Duration) Release {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		rel, ok, err := f.Release(tag)
		if err != nil {
			t.Fatalf("looking up release %q: %v", tag, err)
		}
		if ok {
			return rel
		}
		if time.Now().After(deadline) {
			t.Fatalf("release %q did not appear within %s", tag, timeout)
		}
		time.Sleep(pollEvery)
	}
}

// AlsoClean registers f (a mirror target) for cleanup of this run's releases and tags.
func (w *Workspace) AlsoClean(f Forge) { w.mirrors = append(w.mirrors, f) }
```

Refactor `Require` into `Require(t, name)` → `requireWith(t, name, LoadConfig())` (same body, parameterised by the config), add a `mirrors []Forge` field to `Workspace`, and make `cleanup` run the release-then-tag deletion for each mirror (through the same `w.delete` guard) after the main forge's branch deletion. Extract the release/tag part of `cleanup` into `func (w *Workspace) cleanupReleasesAndTags(t *testing.T, f Forge) []string` so mirrors reuse it.

Run: `go test -count=1 -tags e2e_forge ./e2e/forgeharness`
Expected: PASS.

- [ ] **Step 3: Lint and commit**

Run: `hk fix && hk check`

```bash
git add e2e/forgeharness
git commit -F - <<'EOF'
test(e2e): extend the forge harness for mirrors, merges and polling

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Scenarios B5-B8 and B10

**Files:**
- Create: `e2e/forge_flows_test.go`

**Interfaces:**
- Consumes: Task 1; T345c's `eachForge`, `commitConfigAndSubjects`, `release`, `semverBlock`, `forgeNames`; `harness.Binary` (including the `heraut_testclock` build), `exitOK`, `exitRuntime`.

- [ ] **Step 1: Write the scenarios**

Create `e2e/forge_flows_test.go`:

```go
//go:build e2e_forge

package e2e_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/forgeharness"
	"github.com/adaouat/heraut/e2e/harness"
)

func TestForge_B5_CalVerRelease(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		block := "versioning:\n  strategy: calver\n  format: \"YYYY.MM.PATCH\"\n  tag_prefix: \"" + ws.TagPrefix + "\"\n"
		commitConfigAndSubjects(ws, ws.Config(block, ""), "feat: one")

		res := ws.Repo.Run(bin, append(ws.Env(), "HERAUT_TEST_NOW=2031-03-04T00:00:00Z"), "release", "--offline")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		tag := ws.TagPrefix + "2031.03.0"
		rel, ok, err := ws.Forge.Release(tag)
		require.NoError(t, err)
		require.True(t, ok, "a CalVer tag shape is accepted by the forge")
		assert.Contains(t, rel.Body, "One")
	})
}

func TestForge_B6_GitLabSourceMirroredToGitHub(t *testing.T) {
	bin := harness.Binary(t)
	gl := forgeharness.Require(t, "gitlab")
	gh := forgeharness.Require(t, "github")
	ws := forgeharness.NewWorkspace(t, gl)
	ws.AlsoClean(gh)

	cfg := `version: "1"
` + semverBlock(ws) + `changelog:
  output: CHANGELOG.md
commits:
  enrichment_forge: gitlab
forges:
  - name: gitlab
    platform: gitlab
    project: ` + gl.Coordinates() + `
    token_env: GITLAB_TOKEN
  - name: github
    platform: github
    repository: ` + gh.Coordinates() + `
    token_env: GH_TOKEN
release:
  targets:
    - forge: gitlab
    - forge: github
`
	commitConfigAndSubjects(ws, cfg, "feat: one", "fix: two")

	res := ws.Repo.Run(bin, []string{"GITLAB_TOKEN=" + gl.Token(), "GH_TOKEN=" + gh.Token()}, "release", "--offline")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	tag := ws.TagPrefix + "0.1.0"
	glRel, ok, err := gl.Release(tag)
	require.NoError(t, err)
	require.True(t, ok, "the source forge has the release")
	ghRel := forgeharness.WaitForRelease(t, gh, tag, 60*time.Second)
	assert.Equal(t, trimmed(glRel.Body), trimmed(ghRel.Body), "both forges carry the same notes")
}

func TestForge_B7_DraftRelease(t *testing.T) {
	bin := harness.Binary(t)
	f := forgeharness.Require(t, "github")
	ws := forgeharness.NewWorkspace(t, f)
	cfg := ws.Config(semverBlock(ws), "")
	cfg = replaceOnce(cfg, "    - forge: github\n", "    - forge: github\n      draft: true\n")
	commitConfigAndSubjects(ws, cfg, "feat: one")

	release(t, ws, bin)

	tag := ws.TagPrefix + "0.1.0"
	rel := forgeharness.WaitForRelease(t, f, tag, 90*time.Second)
	assert.True(t, rel.Draft, "the release is a draft, not published")
}

func TestForge_B8_MaintenanceLine(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		block := "versioning:\n  strategy: semver\n  tag_prefix: \"" + ws.TagPrefix + "\"\n  branches:\n    - name: main\n    - name: \"e2e/*\"\n      range: 1.3.x\n"
		commitConfigAndSubjects(ws, ws.Config(block, ""), "feat: base")
		ws.Repo.Git("tag", "-a", ws.TagPrefix+"1.3.0", "-m", "base")
		ws.Repo.Commit("fix: on the line")

		release(t, ws, bin)

		rel, ok, err := ws.Forge.Release(ws.TagPrefix + "1.3.1")
		require.NoError(t, err)
		require.True(t, ok, "the line released the next patch of its range")
		assert.Contains(t, rel.Body, "On the line")

		// a tag cut elsewhere (a commit that is not in this branch's history) makes the next version taken
		ws.Repo.Git("checkout", "-q", "--detach", "HEAD~1")
		ws.Repo.Commit("chore: elsewhere")
		ws.Repo.Git("tag", ws.TagPrefix+"1.3.2")
		ws.Repo.Git("checkout", "-q", ws.Branch)
		ws.Repo.Commit("fix: another fix")

		res := ws.Repo.Run(bin, ws.Env(), "release", "--offline")

		require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, res.Stdout+res.Stderr, "ag already exists")
	})
}

func TestForge_B10_DryRunCreatesNothing(t *testing.T) {
	bin := harness.Binary(t)
	eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
		commitConfigAndSubjects(ws, ws.Config(semverBlock(ws), ""), "feat: one")
		before := ws.Repo.Git("rev-parse", "HEAD")

		res := ws.Repo.Run(bin, ws.Env(), "release", "--offline", "--dry-run")

		require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Equal(t, before, ws.Repo.Git("rev-parse", "HEAD"), "no local commit")
		tags, err := ws.Forge.Tags(ws.RunID)
		require.NoError(t, err)
		assert.Empty(t, tags, "no tag on the forge")
		_, ok, err := ws.Forge.Release(ws.TagPrefix + "0.1.0")
		require.NoError(t, err)
		assert.False(t, ok, "no release on the forge")
		branches, err := ws.Forge.Branches(ws.Branch)
		require.NoError(t, err)
		assert.Empty(t, branches, "the run's branch was never pushed")
	})
}
```

Add the two helpers to `e2e/forge_release_test.go` (same package, same tag):

```go
func trimmed(s string) string { return strings.TrimSpace(s) }

func replaceOnce(s, old, new string) string { return strings.Replace(s, old, new, 1) }
```

and re-add `"strings"` to its imports. The collision assertion's text is `"ag already exists"` on purpose (matches both `Tag already exists` and `tag already exists` regardless of the panel's capitalisation); keep it.

Run offline first (gating): `go vet -tags e2e_forge ./e2e/ && go test -count=1 -tags e2e_forge -run 'TestForge_' ./e2e/` with the variables unset → all skip.

- [ ] **Step 2: Run live**

```bash
HERAUT_E2E_GITHUB_REPO=<owner>/<name>-testing HERAUT_E2E_GITLAB_PROJECT=<owner>/<name>-testing \
  go test -count=1 -tags e2e_forge -run 'TestForge_B(5|6|7|8|10)' ./e2e/ -v
```

Run it in the background (it takes several minutes) and read the result. Expected: B5, B8, B10 pass once per forge, B6 and B7 pass; afterwards both sandboxes hold no `e2e-*` branch, tag or release (verify with `gh api`/`glab api`). If a scenario fails, read the failure and the forge state, fix the cause, and repeat; do not loosen an assertion. A B6 failure about the GitHub tag/release leaking means `AlsoClean` is wrong.

- [ ] **Step 3: Lint and commit**

Run: `go test -count=1 ./... && hk fix && hk check`

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): cover CalVer, cross-forge, draft, maintenance and dry-run live

Scenarios B5, B6, B7, B8 and B10 against the private sandboxes: a CalVer
tag, one run publishing the same notes to GitLab and GitHub, a GitHub
draft, a maintenance line with a taken version, and a dry run that
creates nothing on the forge.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: B9 enrichment

**Files:**
- Modify: `e2e/forge_flows_test.go`

- [ ] **Step 1: Write the scenario**

Append to `e2e/forge_flows_test.go` (add `"regexp"` to the imports):

```go
var prRef = regexp.MustCompile(`in \[[#!][0-9]+\]\(`)

func TestForge_B9_PullRequestEnrichment(t *testing.T) {
	bin := harness.Binary(t)
	for _, name := range forgeNames {
		t.Run(name, func(t *testing.T) {
			f := forgeharness.RequireEnrich(t, name)
			ws := forgeharness.NewWorkspace(t, f)
			base, feat := ws.Branch+"-base", ws.Branch+"-feat"

			// base and feature branches start at the sandbox's main; the PR merges into the base,
			// so the sandbox's default branch is never written
			ws.Repo.Git("push", "-q", "origin", "HEAD:refs/heads/"+base)
			ws.Repo.Git("checkout", "-q", "-b", feat)
			ws.Repo.WriteFile("widget.txt", "widget\n")
			ws.Repo.Git("add", "widget.txt")
			ws.Repo.Git("commit", "-q", "-m", "feat: add widget")
			ws.Repo.Git("push", "-q", "origin", "HEAD:refs/heads/"+feat)
			number, err := f.OpenAndMerge(base, feat, "feat: add widget (via request)")
			require.NoError(t, err)
			t.Logf("merged request %d", number)

			ws.Repo.Git("fetch", "-q", "origin", base)
			ws.Repo.Git("checkout", "-q", "-B", ws.Branch, "FETCH_HEAD")
			cfg := ws.Config("versioning:\n  strategy: semver\n", "")
			cfg = replaceOnce(cfg, "changelog:\n", "commits:\n  enrichment_policy: required\nchangelog:\n")
			ws.Repo.WriteConfig(cfg)

			// the commit-to-request association shows up a few seconds after the merge: poll
			deadline := time.Now().Add(120 * time.Second)
			for {
				res := ws.Repo.Run(bin, ws.Env(), "changelog", "--set-version", "0.1.0", "--regenerate")
				require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
				if prRef.MatchString(ws.Repo.ReadFile("CHANGELOG.md")) {
					return
				}
				if time.Now().After(deadline) {
					t.Fatalf("the changelog never linked request %d:\n%s", number, ws.Repo.ReadFile("CHANGELOG.md"))
				}
				time.Sleep(10 * time.Second)
			}
		})
	}
}
```

Run: `go vet -tags e2e_forge ./e2e/`, then the gated skip check, then live:

```bash
HERAUT_E2E_GITHUB_ENRICH_REPO=<owner>/<name>-testing-enrich HERAUT_E2E_GITLAB_ENRICH_PROJECT=<owner>/<name>-testing-enrich \
  go test -count=1 -tags e2e_forge -run 'TestForge_B9' ./e2e/ -v
```

Expected: both forges pass; the enrichment sandboxes hold no `e2e/` branch afterwards (merged PRs/MRs remain as history). If GitLab's MR enrichment needs different config (project variable, `api_mode`), fix the scenario's config for that forge and record it in the roadmap note; if the MR link never appears for a reason in heraut, stop and report it as a finding.

- [ ] **Step 2: Lint and commit**

Run: `go test -count=1 ./... && hk fix && hk check`

```bash
git add e2e
git commit -F - <<'EOF'
test(e2e): cover PR/MR enrichment against the dedicated sandbox pair

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Workflow, guide, close T345d

**Files:**
- Create: `.github/workflows/e2e.yml`
- Create: `docs/guides/e2e-tests.md`
- Modify: `docs/tasks/roadmap.md`, `docs/specs/06-dx-and-testing.md` (guide pointer)

- [ ] **Step 1: The workflow**

Create `.github/workflows/e2e.yml`:

```yaml
name: E2E forge lane

# Advisory: runs the opt-in forge-sandbox scenarios (ADR-0066, Lane B). Never on pull requests —
# the sandbox tokens are secrets. A failure sends GitHub's built-in email for failed scheduled
# workflows; it gates nothing. Setup (variables, secrets, token scopes): docs/guides/e2e-tests.md.
on:
  workflow_dispatch:
  schedule:
    - cron: "17 3 * * *"

permissions:
  contents: read

concurrency:
  group: e2e-forge
  cancel-in-progress: false

jobs:
  forge:
    runs-on: ubuntu-latest
    timeout-minutes: 45
    env:
      HERAUT_E2E_GITHUB_REPO: ${{ vars.HERAUT_E2E_GITHUB_REPO }}
      HERAUT_E2E_GITLAB_PROJECT: ${{ vars.HERAUT_E2E_GITLAB_PROJECT }}
      HERAUT_E2E_GITHUB_ENRICH_REPO: ${{ vars.HERAUT_E2E_GITHUB_ENRICH_REPO }}
      HERAUT_E2E_GITLAB_ENRICH_PROJECT: ${{ vars.HERAUT_E2E_GITLAB_ENRICH_PROJECT }}
      HERAUT_E2E_GITHUB_TOKEN: ${{ secrets.HERAUT_E2E_GITHUB_TOKEN }}
      HERAUT_E2E_GITLAB_TOKEN: ${{ secrets.HERAUT_E2E_GITLAB_TOKEN }}
    steps:
      - name: Checkout code
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6
      - name: Setup Mise
        uses: jdx/mise-action@1648a7812b9aeae629881980618f079932869151 # v4
      - name: Install gh and glab (versions from the Dockerfile)
        run: |
          GH_VERSION=$(sed -n 's/^ARG GH_VERSION=//p' Dockerfile | head -1)
          GLAB_VERSION=$(sed -n 's/^ARG GLAB_VERSION=//p' Dockerfile | head -1)
          mise use -g "gh@${GH_VERSION}" "glab@${GLAB_VERSION}"
      - name: Harness unit tests (offline)
        run: go test -count=1 -tags e2e_forge ./e2e/forgeharness
      - name: Sweep leftovers of crashed runs
        run: go run -tags e2e_forge ./e2e/cmd/sweep
      - name: Forge scenarios
        run: go test -count=1 -tags e2e_forge -run 'TestForge_' ./e2e/ -v
```

Run: `hk check` (actionlint and yamlfmt cover workflows). Do not run the workflow.

- [ ] **Step 2: The guide**

Create `docs/guides/e2e-tests.md` documenting, with placeholders only: the two lanes and what each covers (link ADR-0066 and the design); running Lane A (`go test ./...`); creating the sandbox repositories (private, base name matching `*testing*`, one per forge, plus an `-enrich` pair for B9 with no branch protection on the base branches); the environment variables (`HERAUT_E2E_GITHUB_REPO`, `HERAUT_E2E_GITLAB_PROJECT`, `HERAUT_E2E_GITHUB_ENRICH_REPO`, `HERAUT_E2E_GITLAB_ENRICH_PROJECT`, `HERAUT_E2E_REPO_PATTERN`, `HERAUT_E2E_GITHUB_TOKEN`, `HERAUT_E2E_GITLAB_TOKEN`); local runs with `mise run test:e2e` using the developer's `gh`/`glab` login; CI setup (repository variables for the coordinates, secrets for the two tokens, token scopes: GitHub fine-grained token with Contents and Pull requests read/write on the sandbox repos only; GitLab project or personal token with `api` scope limited to the sandboxes, role Maintainer); the safety guards and cleanup (run id, per-run branch, sweeper command, what the sweeper cannot delete: merged PRs/MRs of B9); the eventual-consistency notes; and troubleshooting (a leaked resource: run the sweeper). No real names. Add a pointer line to `docs/specs/06-dx-and-testing.md`'s "End-to-end tests" section.

- [ ] **Step 3: Close T345d and verify**

In `docs/tasks/roadmap.md` flip the T345d heading to `[x]`, add a completion note with real facts (scenarios and live results per forge, the enrichment pair created for B9, any finding or deviation, the workflow and guide), and set the Phase 60 row to done except T359/T360. `grep -rn` for the real sandbox names in `docs/`, `e2e/` and `.github/` must print nothing. Run `go test -count=1 ./... && hk check`.

```bash
git add .github/workflows/e2e.yml docs
git commit -F - <<'EOF'
ci(e2e): add the advisory forge-lane workflow and the setup guide

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
EOF
```

(If the roadmap/spec edits are large, make them a separate `docs(roadmap): close T345d` commit.)

---

## Self-review

**Spec coverage** (B5-B10, workflow, guide, notification via GitHub's email): B5 Task 2, B6 Task 2, B7 Task 2, B8 Task 2, B9 Task 3, B10 Task 2, workflow and guide Task 4. The spec's `mise run test:e2e` landed in T345c.

**Type consistency:** `OpenAndMerge`, `RequireEnrich`, `AlsoClean`, `WaitForRelease`, `ForEnrich`, `pollEvery` are defined in Task 1 and used in Tasks 2-3; `trimmed` and `replaceOnce` are defined in Task 2 and used in Tasks 2-3.

**Placeholders:** all coordinates are `<owner>/<name>-testing`; the Task 4 notes are written from live results.
