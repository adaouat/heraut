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
)

// TestTagOrderFor_RealRepo_SemverPrecedenceBoundsChangelogAndNotes is T334's real-git regression,
// covering the SemVer v2 roadmap's "Review Focus" 1 and 2:
//
//  1. A repo with v1.3.0, v1.4.0-rc.1 (commit A), v1.4.0+158404 (commit B) and a new commit C: the
//     incremental CHANGELOG section for the next release must list commit C only — not re-list
//     commit B, which git's version:refname sort would do (it puts the pre-release tag above its
//     own release, ASCII '+' < '-').
//  2. Release notes for a final (v1.5.0) cut after a pre-release of the same core (v1.5.0-rc.1)
//     must span back to the last final (v1.4.0+158404), not to the pre-release tag `git describe`
//     topology would pick.
func TestTagOrderFor_RealRepo_SemverPrecedenceBoundsChangelogAndNotes(t *testing.T) {
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
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	commit := func(msg string) { git("commit", "--allow-empty", "-m", msg) }

	git("init")
	commit("feat: initial")
	git("tag", "v1.3.0")
	commit("feat: commit a")
	git("tag", "v1.4.0-rc.1")
	commit("feat: commit b")
	git("tag", "v1.4.0+158404")
	commit("feat: commit c")

	runner := execadapter.New(false, false)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}
	order := tagOrderFor(cfg, "")

	// Review Focus 1: the incremental changelog's new section must list commit C only.
	changelogDir := t.TempDir()
	driver := &config.ContentDriver{Output: filepath.Join(changelogDir, "CHANGELOG.md")}
	changelogGen := buildGenerator(runner, driver, native.ModeChangelog, "", false, false, nil, "", order)

	body, err := changelogGen.Generate("v1.5.0", nil)
	require.NoError(t, err)

	unreleasedAnchor := "<!-- heraut-release: v1.5.0 -->"
	releasedAnchor := "<!-- heraut-release: v1.4.0+158404 -->"
	unreleasedStart := strings.Index(body, unreleasedAnchor)
	releasedStart := strings.Index(body, releasedAnchor)
	require.NotEqual(t, -1, unreleasedStart, "the new v1.5.0 section must exist")
	require.NotEqual(t, -1, releasedStart, "v1.4.0+158404 must have its own section")
	assert.NotContains(t, body, "1.4.0-rc.1]", "the pre-release tag must get no section of its own (ADR-0064)")

	unreleasedSection := body[unreleasedStart:releasedStart]
	assert.Contains(t, unreleasedSection, "Commit c")
	assert.NotContains(t, unreleasedSection, "Commit b",
		"commit B is already released under v1.4.0+158404 and must not be re-listed in the new section")

	assert.Equal(t, 1, strings.Count(body, "Commit b"),
		"commit B must appear exactly once, in its own already-released section")

	// Review Focus 2: a final cut after a pre-release of the same core spans back to the last
	// final, not to the pre-release `git describe` topology would pick.
	git("tag", "v1.5.0-rc.1")
	git("tag", "v1.5.0")

	notesGen := buildGenerator(runner, &config.ContentDriver{}, native.ModeReleaseNotes, "", false, false, nil, "", order)
	notes, err := notesGen.Generate("v1.5.0", nil)
	require.NoError(t, err)

	assert.Contains(t, notes, "Commit c")
	assert.NotContains(t, notes, "Commit b",
		"release notes for the final must span back to v1.4.0+158404 (the last final), not v1.4.0-rc.1")
	assert.NotContains(t, notes, "Commit a")
}
