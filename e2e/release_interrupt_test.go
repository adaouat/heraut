//go:build unix

package e2e_test

import (
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
	arrived := repo.FakeGitHub().HangUploads()

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
	assert.Equal(t, []string{githubList, githubRelease, githubUpload, githubDelete}, repo.APICalls())
}
