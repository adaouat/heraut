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

// TestTagOrderFor_RealRepo_FallbackNeverBoundsByNonAncestorTag (T334): two envs on diverging
// branches under semver-per-env (tag_format "{env}/{version}", whose {env}
// token is a wildcard in tagfmt.ParseVersion — any env's tag parses through it identically). envb
// branches off before enva's own first release exists, so envb's first release has no ancestor
// release at all — but enva/0.9.0 sorts adjacent to envb/1.0.0 in a naive unscoped, §11-ordered
// pool (0.9.0 immediately below 1.0.0), so an ordering-only fallback would pick it as "previous"
// despite it sitting on an unrelated branch. The ancestor-only listing (`git tag -l --merged <t>
// --no-contains <t>`) must exclude it, since enva/0.9.0 is never an ancestor of envb/1.0.0.
func TestTagOrderFor_RealRepo_FallbackNeverBoundsByNonAncestorTag(t *testing.T) {
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
	git("checkout", "-b", "trunk")
	commit("feat: root")
	git("checkout", "-b", "envb")
	commit("feat: envb release")
	git("tag", "envb/1.0.0")
	git("checkout", "trunk")
	commit("feat: enva release")
	git("tag", "enva/0.9.0")
	git("checkout", "envb")

	runner := execadapter.New(false, false)
	cfg := &config.Config{
		Versioning:   config.Versioning{Strategy: "semver-per-env", TagFormat: "{env}/{version}"},
		Environments: map[string]config.Environment{"envb": {}},
	}
	order := tagOrderFor(cfg, "envb")

	changelogDir := t.TempDir()
	driver := &config.ContentDriver{Output: filepath.Join(changelogDir, "CHANGELOG.md"), TagGlob: "envb/*"}
	gen := buildGenerator(runner, driver, native.ModeChangelog, "", false, false, nil, "", order)

	body, err := gen.Generate("envb/1.1.0", nil)
	require.NoError(t, err)

	assert.NotContains(t, body, "Enva release", "enva's commit must never leak into envb's changelog")
	assert.Contains(t, body, "Envb release")
	assert.Contains(t, body, "Root",
		"envb/1.0.0's true (ancestor-bound) previous tag is none — its full history, including the "+
			"root commit, belongs to its own section. A non-ancestor prev (enva/0.9.0) would "+
			"have silently excluded the root commit from the range instead, since enva/0.9.0 already "+
			"covers it — reproducing T334's missing-entries bug class on a different topology.")
}
