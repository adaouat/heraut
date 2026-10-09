//go:build e2e_forge

package e2e_test

import (
	"regexp"
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
	assert.Equal(t, withoutLinks(glRel.Body), withoutLinks(ghRel.Body), "both forges carry the same notes")
	assert.Contains(t, glRel.Body, "gitlab.com", "the GitLab notes link commits on GitLab")
	assert.Contains(t, ghRel.Body, "github.com", "and the GitHub notes link commits on GitHub")
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

var mdLink = regexp.MustCompile(`\]\([^)]*\)`)

// withoutLinks drops the target URLs of markdown links: each target's notes link commits on its
// own forge, so only the text around the links is comparable.
func withoutLinks(s string) string { return trimmed(mdLink.ReplaceAllString(s, "]()")) }

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
