package native_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
)

// gitFixture builds a real temporary git repository with a known set of conventional commits
// across two releases and chdirs into it (restored on cleanup). It exercises the native
// generator against the real git CLI and the real exec runner — the integration counterpart
// to the MockRunner contract tests. Skips when git is unavailable.
//
// Release v0.1.0: feat ×2 (same scope) / fix / docs / chore(deps) (excluded) / non-conventional.
// Unreleased:     feat / revert.
func gitFixture(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	t.Chdir(dir)

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
			// Hermetic: ignore the developer's global/system git config (gpg signing,
			// forced-annotated tags, etc.) so the fixture is deterministic on any machine.
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	commit := func(msg string) { git("commit", "--allow-empty", "-m", msg) }

	git("init")
	commit("feat(auth): add login")  // oldest in the Features group
	commit("feat(auth): add logout") // same scope → oldest-first is the within-group tiebreak
	commit("fix: correct null check")
	commit("docs: update readme")
	commit("chore(deps): bump some library") // excluded by the default rendering.excludes
	commit("random non-conventional commit") // → catch-all Other
	git("tag", "v0.1.0")
	commit("feat(ui): redesign the page")
	commit("revert: revert the redesign")
}

func TestNativeGenerator_RealRepo_Changelog(t *testing.T) {
	gitFixture(t)
	g := native.New(execadapter.New(false, false), &config.ContentDriver{Generator: "native"}, native.ModeChangelog)

	out, err := g.Generate("v0.2.0", nil)
	require.NoError(t, err)

	// Full file: header + a section for the unreleased tag and one per existing tag.
	assert.Contains(t, out, "# Changelog")
	assert.Contains(t, out, "## [0.2.0]", "the unreleased section uses the new tag")
	assert.Contains(t, out, "## [0.1.0]", "plus a section per existing tag")

	// Type grouping from the default commits.types taxonomy.
	assert.Contains(t, out, "### 🚀 Features")
	assert.Contains(t, out, "Add login")
	assert.Contains(t, out, "Redesign the page")
	// Within a group, commits render oldest-first (collectCommits passes --reverse).
	assert.Less(t, strings.Index(out, "Add login"), strings.Index(out, "Add logout"),
		"oldest-first within the Features group")
	assert.Contains(t, out, "### 🐛 Bug Fixes")
	assert.Contains(t, out, "Correct null check")
	assert.Contains(t, out, "### 📚 Documentation")

	// Built-in fallbacks: catch-all Other (non-conventional) and Revert (T132/T134).
	assert.Contains(t, out, "### 💼 Other")
	assert.Contains(t, out, "Random non-conventional commit")
	assert.Contains(t, out, "### ◀️ Revert")

	// Default exclusion: chore(deps) never renders.
	assert.NotContains(t, out, "bump some library")
}

func TestNativeGenerator_RealRepo_ReleaseNotes(t *testing.T) {
	gitFixture(t)
	g := native.New(execadapter.New(false, false), &config.ContentDriver{Generator: "native"}, native.ModeReleaseNotes)

	// Release notes for the first tag — its commit range is the full history up to v0.1.0.
	out, err := g.Generate("v0.1.0", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "### 🚀 Features")
	assert.Contains(t, out, "Add login")
	assert.NotContains(t, out, "Redesign the page", "feat(ui) belongs to the unreleased range, not v0.1.0")
	assert.Contains(t, out, "Commit Statistics")
	assert.Contains(t, out, "commit(s) contributed to the release.")
}
