package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
)

// historyRepo starts a real git repo in a fresh temp dir (the test's working directory) and
// returns its git helper. Global/system git config and CI branch variables are neutralised so
// the host can't influence the result.
func historyRepo(t *testing.T) func(args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	testutil.ClearCIEnv(t)
	for _, k := range []string{"CI_COMMIT_BRANCH", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "BUILD_SOURCEBRANCH", "BUILD_SOURCEBRANCHNAME"} {
		t.Setenv(k, "")
	}
	t.Chdir(t.TempDir())

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("checkout", "-b", "main")
	return git
}

// buildMaintenanceRepo builds main (v1.3.0, v1.3.1, v1.4.0, then an unreleased feature) and a
// release/1.3 maintenance line forked at v1.3.1 carrying v1.3.2, leaving main checked out.
func buildMaintenanceRepo(t *testing.T) func(args ...string) {
	t.Helper()
	git := historyRepo(t)
	commit := func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag := func(name string) { git("tag", "-a", "-m", name, name) }

	commit("feat: initial")
	tag("v1.3.0")
	commit("fix: first patch")
	tag("v1.3.1")
	commit("feat: minor feature")
	tag("v1.4.0")
	git("checkout", "-b", "release/1.3", "v1.3.1")
	commit("fix: maintenance patch")
	tag("v1.3.2")
	git("checkout", "main")
	commit("feat: next feature")
	return git
}

var historyLC = &port.LinkContext{Platform: "github", BaseURL: "https://github.com", Owner: "acme", Repo: "widget"}

// sectionOf returns the changelog section anchored at tag (up to the next anchor), or "".
func sectionOf(body, tag string) string {
	anchor := "<!-- heraut-release: " + tag + " -->"
	start := strings.Index(body, anchor)
	if start == -1 {
		return ""
	}
	rest := body[start+len(anchor):]
	if end := strings.Index(rest, "<!-- heraut-release: "); end != -1 {
		rest = rest[:end]
	}
	return rest
}

func regenerateSemver(t *testing.T, cfg *config.Config, env string, driver config.ContentDriver, tag string) string {
	t.Helper()
	driver.Output = filepath.Join(t.TempDir(), "CHANGELOG.md")
	gen := buildGenerator(execadapter.New(false, false), &driver, native.ModeChangelog, "", true, false, nil, "", tagOrderFor(cfg, env))
	body, err := gen.Generate(tag, historyLC)
	require.NoError(t, err)
	return body
}

// TestHistoryBounds_RealRepo_UnmergedMaintenanceTagGetsNoSection (T352, ADR-0065): a maintenance
// tag never merged into main is outside main's history, so main's changelog has no section for it,
// and v1.4.0 stays bounded at v1.3.1.
func TestHistoryBounds_RealRepo_UnmergedMaintenanceTagGetsNoSection(t *testing.T) {
	buildMaintenanceRepo(t)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}

	body := regenerateSemver(t, cfg, "", config.ContentDriver{}, "v1.5.0")

	assert.NotContains(t, body, "<!-- heraut-release: v1.3.2 -->", "an unmerged maintenance tag gets no section")
	assert.NotContains(t, body, "Maintenance patch")
	v140 := sectionOf(body, "v1.4.0")
	require.NotEmpty(t, v140)
	assert.Contains(t, v140, "Minor feature")
	assert.NotContains(t, v140, "First patch")
	assert.Contains(t, v140, "compare/v1.3.1..v1.4.0")
}

// TestHistoryBounds_RealRepo_MergedForwardMaintenanceTag (T352, ADR-0065): once release/1.3 is
// merged forward, v1.3.2 gets its own section (v1.3.1..v1.3.2) and v1.4.0 is still bounded at its
// own ancestor v1.3.1 — not at v1.3.2, which precedes it by version but not in its history.
func TestHistoryBounds_RealRepo_MergedForwardMaintenanceTag(t *testing.T) {
	git := buildMaintenanceRepo(t)
	git("merge", "--no-ff", "-m", "chore: merge release/1.3", "release/1.3")
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}

	body := regenerateSemver(t, cfg, "", config.ContentDriver{}, "v1.5.0")

	v132 := sectionOf(body, "v1.3.2")
	require.NotEmpty(t, v132, "a merged-forward maintenance tag gets its own section")
	assert.Contains(t, v132, "Maintenance patch")
	assert.NotContains(t, v132, "First patch")
	assert.NotContains(t, v132, "Minor feature")
	assert.Contains(t, v132, "compare/v1.3.1..v1.3.2")

	v140 := sectionOf(body, "v1.4.0")
	require.NotEmpty(t, v140)
	assert.Contains(t, v140, "Minor feature")
	assert.NotContains(t, v140, "Maintenance patch")
	assert.Contains(t, v140, "compare/v1.3.1..v1.4.0", "v1.4.0 is bounded by its own ancestor, not by v1.3.2")
}

// TestHistoryBounds_RealRepo_PerEnvAncestryIsScopePreserving (T352, ADR-0065): under
// semver-per-env, uat/1.3.0 is prod/1.3.0's highest-precedence ancestor overall, but the prod
// changelog's section is bounded by the highest ancestor in its own scope, prod/1.2.0, so it lists
// both commits B and C.
func TestHistoryBounds_RealRepo_PerEnvAncestryIsScopePreserving(t *testing.T) {
	git := historyRepo(t)
	commit := func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag := func(name string) { git("tag", "-a", "-m", name, name) }

	commit("feat: commit a")
	tag("prod/1.2.0")
	commit("feat: commit b")
	tag("uat/1.3.0")
	commit("feat: commit c")
	tag("prod/1.3.0")

	cfg := &config.Config{
		Versioning:   config.Versioning{Strategy: "semver-per-env", TagFormat: "{env}/{version}"},
		Environments: map[string]config.Environment{"prod": {}, "uat": {}},
	}

	body := regenerateSemver(t, cfg, "prod", config.ContentDriver{TagGlob: "prod/*"}, "prod/1.4.0")

	prod130 := sectionOf(body, "prod/1.3.0")
	require.NotEmpty(t, prod130)
	assert.Contains(t, prod130, "Commit b")
	assert.Contains(t, prod130, "Commit c")
	assert.NotContains(t, prod130, "Commit a")
	assert.NotContains(t, body, "<!-- heraut-release: uat/1.3.0 -->")
}

// TestHistoryBounds_RealRepo_SameCommitTagsBoundEachOther (ADR-0065): two releases tagged on one
// commit bound each other as they did before ancestry bounds — v1.4.1's section is v1.4.0..v1.4.1,
// empty — so v1.4.0's entries are never repeated under v1.4.1, keeping linear-history output
// unchanged.
func TestHistoryBounds_RealRepo_SameCommitTagsBoundEachOther(t *testing.T) {
	git := historyRepo(t)
	commit := func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag := func(name string) { git("tag", "-a", "-m", name, name) }

	commit("feat: initial")
	tag("v1.3.0")
	commit("feat: minor feature")
	tag("v1.4.0")
	tag("v1.4.1")
	commit("feat: next feature")
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}

	body := regenerateSemver(t, cfg, "", config.ContentDriver{}, "v1.5.0")

	assert.Equal(t, 1, strings.Count(body, "Minor feature"), "v1.4.0's entries appear once:\n%s", body)
	assert.NotContains(t, sectionOf(body, "v1.4.1"), "Minor feature")
	v140 := sectionOf(body, "v1.4.0")
	require.NotEmpty(t, v140)
	assert.Contains(t, v140, "Minor feature")
	assert.Contains(t, v140, "compare/v1.3.0..v1.4.0")
}

// TestHistoryBounds_RealRepo_PromotionTagOnSameCommitNeverBoundsFallback (ADR-0065): a promotion
// tags one commit for several envs. prod/1.2.0, the oldest prod release, shares its commit with
// uat/1.2.0; the oldest-in-scope fallback must skip that same-commit tag and bound at uat/1.1.0,
// not produce an empty section.
func TestHistoryBounds_RealRepo_PromotionTagOnSameCommitNeverBoundsFallback(t *testing.T) {
	git := historyRepo(t)
	commit := func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag := func(name string) { git("tag", "-a", "-m", name, name) }

	commit("feat: commit a")
	tag("uat/1.1.0")
	commit("feat: commit b")
	tag("uat/1.2.0")
	tag("prod/1.2.0")
	commit("feat: commit c")

	cfg := &config.Config{
		Versioning:   config.Versioning{Strategy: "semver-per-env", TagFormat: "{env}/{version}"},
		Environments: map[string]config.Environment{"prod": {}, "uat": {}},
	}

	body := regenerateSemver(t, cfg, "prod", config.ContentDriver{TagGlob: "prod/*"}, "prod/1.3.0")

	prod120 := sectionOf(body, "prod/1.2.0")
	require.NotEmpty(t, prod120)
	assert.Contains(t, prod120, "Commit b")
	assert.NotContains(t, prod120, "Commit a")
}
