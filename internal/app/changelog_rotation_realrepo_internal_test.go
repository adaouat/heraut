package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
)

// TestChangelogRotation_RealRepo_PeriodBoundary is T248: a real-git-repo proof that a CalVer
// rotation pattern produces one file per period, each containing only its own period's entries —
// with no duplication across the boundary. It exercises the full wiring
// (wrapWithRotation → buildGenerator → native.Generate) against the real git CLI, the integration
// counterpart to changelog_rotation_internal_test.go's MockRunner-based unit tests.
//
// This test is what surfaced T247's PreviousTagOverride gap in the first place: without it, the
// first release of the 2026 bucket would have no in-scope previous tag to bound its range by, and
// would silently absorb every 2025 commit too.
func TestChangelogRotation_RealRepo_PeriodBoundary(t *testing.T) {
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
	commit("feat: first feature")

	changelogDir := t.TempDir()
	driver := &config.ContentDriver{Output: filepath.Join(changelogDir, "CHANGELOG_{YYYY}.md")}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	runner := execadapter.New(false, false)

	build := func() *rotatingGenerator {
		gen := buildGenerator(runner, driver, native.ModeChangelog, "", false, false, nil, "")
		wrapped := wrapWithRotation(gen, runner, cfg, driver, "", false, false, nil, "")
		rg, ok := wrapped.(*rotatingGenerator)
		require.True(t, ok, "driver.Output has rotation tokens, wrapWithRotation must wrap")
		return rg
	}

	// Release 1: 2025.12.0 — generated before the tag exists, matching the real pipeline's
	// generate-then-tag sequencing.
	_, err := build().Generate("2025.12.0", nil)
	require.NoError(t, err)
	git("tag", "2025.12.0")

	// Time passes into the new year.
	commit("feat: second feature")

	// Release 2: 2026.01.0 — a new rotation bucket. 2025.12.0 exists as a real tag but falls
	// outside the "^2026\." scope.
	_, err = build().Generate("2026.01.0", nil)
	require.NoError(t, err)

	changelog2025, err := os.ReadFile(filepath.Join(changelogDir, "CHANGELOG_2025.md"))
	require.NoError(t, err, "CHANGELOG_2025.md must exist")
	assert.Contains(t, string(changelog2025), "First feature")
	assert.NotContains(t, string(changelog2025), "Second feature")

	changelog2026, err := os.ReadFile(filepath.Join(changelogDir, "CHANGELOG_2026.md"))
	require.NoError(t, err, "CHANGELOG_2026.md must exist")
	assert.Contains(t, string(changelog2026), "Second feature")
	assert.NotContains(t, string(changelog2026), "First feature",
		"the new bucket's first release must not absorb the prior bucket's entries")

	_, statErr := os.Stat(filepath.Join(changelogDir, "CHANGELOG_{YYYY}.md"))
	assert.True(t, os.IsNotExist(statErr), "the literal, unsubstituted pattern must never be written to disk")
}
