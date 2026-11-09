//go:build unix

package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/e2e/harness"
)

// Ctrl-C while an asset upload hangs stops heraut instead of being ignored, and the draft it created
// is deleted on the way out, so no half-published release is left behind.
func TestRelease_InterruptDuringAnUploadDeletesTheDraftAndExits(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg(`    - forge: github
      assets:
        - "dist/*.txt"
`, ""))
	repo.WriteFile("dist/a.txt", "a\n")
	arrived := repo.FakeGitHub().Hang(harness.HangUploads)

	p := repo.Start(bin, tokens, "release", "--offline")
	select {
	case <-arrived:
	case <-time.After(30 * time.Second):
		res := p.Wait(t)
		t.Fatalf("the upload never started; stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	}
	p.Interrupt(t)
	res := p.Wait(t)

	assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout, "context canceled")
	assert.Equal(t, []string{githubList, githubRepo, githubRelease, githubUpload, githubDelete}, repo.APICalls())
}

// Ctrl-C while the token check stalls stops heraut check instead of being ignored, whichever form
// runs the check.
func TestCheck_InterruptDuringAStalledTokenCheckExits(t *testing.T) {
	bin := harness.Binary(t)
	for _, args := range [][]string{
		{"check", "runtime"},
		{"check"},
		{"check", "runtime", "--json"},
		{"check", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			repo := harness.NewRepo(t)
			repo.WriteConfig(withStandIns(repo, githubCfg))
			repo.Commit("chore: init")
			arrived := repo.FakeGitHub().Hang(harness.HangReleaseLists)

			p := repo.Start(bin, tokens, args...)
			interruptOnArrival(t, p, arrived, "the token check")
			res := p.Wait(t)

			assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			assert.Contains(t, res.Stdout+res.Stderr, "context canceled")
		})
	}
}

// Ctrl-C while a pull-request lookup stalls stops heraut changelog. Under the default optional
// policy a failed lookup only drops the PR links; an interrupt writes no changelog at all.
func TestChangelog_InterruptDuringAStalledLookupWritesNothing(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.WriteConfig(withStandIns(repo, githubCfg+"changelog:\n  output: CHANGELOG.md\n"))
	repo.Commit("chore: init")
	repo.Commit("feat: one")
	arrived := repo.FakeGitHub().Hang(harness.HangGraphQL)

	p := repo.Start(bin, tokens, "changelog")
	interruptOnArrival(t, p, arrived, "the pull-request lookup")
	res := p.Wait(t)

	assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout+res.Stderr, "interrupt signal received")
	assert.NotContains(t, res.Stdout+res.Stderr, "(required)", "the default policy is optional")
	assert.NoFileExists(t, filepath.Join(repo.Dir, "CHANGELOG.md"))
}

// interruptOnArrival sends Ctrl-C to p once the stand-in has received the request it was told to
// leave unanswered.
func interruptOnArrival(t *testing.T, p *harness.Process, arrived <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-arrived:
	case <-time.After(30 * time.Second):
		res := p.Wait(t)
		t.Fatalf("%s never started; stdout:\n%s\nstderr:\n%s", what, res.Stdout, res.Stderr)
	}
	p.Interrupt(t)
}

// The same holds for heraut notes: an interrupted lookup prints no notes.
func TestNotes_InterruptDuringAStalledLookupExits(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.WriteConfig(withStandIns(repo, githubCfg))
	repo.Commit("feat: one")
	repo.Git("tag", "-a", "-m", "v1.0.0", "v1.0.0")
	arrived := repo.FakeGitHub().Hang(harness.HangGraphQL)

	p := repo.Start(bin, tokens, "notes", "v1.0.0")
	interruptOnArrival(t, p, arrived, "the pull-request lookup")
	res := p.Wait(t)

	assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout+res.Stderr, "interrupt signal received")
	assert.NotContains(t, res.Stdout, "One")
}

// Ctrl-C during heraut release's preflight token check stops it before anything is tagged.
func TestRelease_InterruptDuringTheTokenCheckExitsBeforeTagging(t *testing.T) {
	bin := harness.Binary(t)
	repo := releaseRepo(t, releaseCfg("    - forge: github\n", ""))
	arrived := repo.FakeGitHub().Hang(harness.HangReleaseLists)

	p := repo.Start(bin, tokens, "release", "--offline")
	interruptOnArrival(t, p, arrived, "the token check")
	res := p.Wait(t)

	assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout+res.Stderr, "context canceled")
	assert.Empty(t, repo.Git("tag", "--list"))
}
