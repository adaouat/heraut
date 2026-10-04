package app

import (
	"os"
	"os/exec"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
)

func notesRepo(t *testing.T) (git func(args ...string), commit func(msg string)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Chdir(t.TempDir())
	git = func(args ...string) {
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
	commit = func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	git("init", "-b", "main")
	return git, commit
}

// notesFor generates release notes through buildReleasePipelineConfig, so the pre-release wiring
// under test is the production one.
func notesFor(t *testing.T, tag string, preRelease bool) string {
	t.Helper()
	testutil.ClearCIEnv(t)
	cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"},
		Release: &config.Release{Notes: &config.ContentDriver{}}}
	runner := execadapter.New(false, false)
	pCfg, err := buildReleasePipelineConfig(runner, runner, cfg, "", "", false, false, preRelease)
	require.NoError(t, err)
	require.NotNil(t, pCfg.Notes)
	out, err := pCfg.Notes.Generate(tag, nil)
	require.NoError(t, err)
	return out
}

// TestNotesRange_RealRepo_PreReleaseSpansPreviousTagAny (T341, ADR-0064): an rc's notes span back
// to the previous tag of any kind; the final spans back to the last final.
func TestNotesRange_RealRepo_PreReleaseSpansPreviousTagAny(t *testing.T) {
	git, commit := notesRepo(t)
	commit("feat: initial")
	git("tag", "-a", "-m", "v1.3.0", "v1.3.0")
	commit("feat: commit a")
	git("tag", "-a", "-m", "rc1", "v1.4.0-rc.1")
	commit("feat: commit b")
	git("tag", "-a", "-m", "rc2", "v1.4.0-rc.2")

	rc2 := notesFor(t, "v1.4.0-rc.2", true)
	assert.Contains(t, rc2, "Commit b")
	assert.NotContains(t, rc2, "Commit a", "rc.2 notes start at rc.1")

	git("tag", "-a", "-m", "final", "v1.4.0")
	final := notesFor(t, "v1.4.0", false)
	assert.Contains(t, final, "Commit a")
	assert.Contains(t, final, "Commit b")
}

// TestNotesRange_RealRepo_PreReleaseIgnoresSideBranchTag (T341): a tag that exists only on a
// branch not merged into HEAD never bounds a pre-release's notes.
func TestNotesRange_RealRepo_PreReleaseIgnoresSideBranchTag(t *testing.T) {
	git, commit := notesRepo(t)
	commit("feat: initial")
	git("tag", "-a", "-m", "v1.3.0", "v1.3.0")
	commit("feat: between")
	git("checkout", "-b", "side")
	commit("fix: side fix")
	git("tag", "-a", "-m", "v1.3.2", "v1.3.2")
	git("checkout", "main")
	commit("feat: main work")
	git("tag", "-a", "-m", "rc1", "v1.4.0-rc.1")

	got := notesFor(t, "v1.4.0-rc.1", true)
	assert.Contains(t, got, "Main work")
	assert.Contains(t, got, "Between", "a range bounded by the unmerged v1.3.2 would drop this commit")
	assert.NotContains(t, got, "Side fix")
	assert.NotContains(t, got, "Initial", "range starts at v1.3.0, not before it")
}
