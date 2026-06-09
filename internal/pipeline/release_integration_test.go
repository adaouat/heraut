package pipeline_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/gitcliff"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/platforms/github"
	"github.com/adaouat/heraut/internal/platforms/gitlab"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_Integration_MultiPlatform_DistinctlyFlavoredNotes is the end-to-end closure of
// Phase 14 (T72). Unlike the MockRunner contract tests, it drives the pipeline through the
// *real* exec runner, so it proves the one path MockRunner cannot: heraut's per-platform
// HERAUT_REMOTE_URL actually propagates through exec.Runner.RunEnv into the git-cliff
// subprocess, and each platform's release receives notes rendered with *its own* host.
//
// The external tools are FakeBins: a stand-in git-cliff that echoes the injected
// $HERAUT_REMOTE_URL (standing in for "git-cliff rendered links against this host" —
// actual Tera rendering is covered by the T71 manual PoC), and gh/glab that capture the
// --notes they receive. git is faked to a no-op so `git tag`/`git push` never touch the
// real repository this test runs in.
func TestRun_Integration_MultiPlatform_DistinctlyFlavoredNotes(t *testing.T) {
	capture := t.TempDir()
	ghNotes := filepath.Join(capture, "gh-notes.txt")
	glNotes := filepath.Join(capture, "gl-notes.txt")
	t.Setenv("GH_CAPTURE", ghNotes)
	t.Setenv("GLAB_CAPTURE", glNotes)

	// git: no-op so tag/push never mutate the real repo this test runs in.
	exectest.FakeBin(t, "git", "#!/bin/sh\nexit 0\n")
	// git-cliff: echo the heraut-injected per-platform remote URL as the "notes".
	exectest.FakeBin(t, "git-cliff", "#!/bin/sh\nprintf '%s' \"$HERAUT_REMOTE_URL\"\n")
	// gh / glab: capture the --notes argument each was handed, to its own file.
	exectest.FakeBin(t, "gh", captureNotesScript("GH_CAPTURE"))
	exectest.FakeBin(t, "glab", captureNotesScript("GLAB_CAPTURE"))

	runner := execadapter.New(false, false)

	cfg := &pipeline.Config{
		Notes: gitcliff.New(runner, &config.ContentDriver{Generator: "git-cliff"}, gitcliff.ModeReleaseNotes),
		Platforms: []port.Platform{
			github.New(runner, &config.Platform{Type: "github", Repository: "test/gh-repo"}),
			gitlab.New(runner, &config.Platform{Type: "gitlab", Project: "test/gl-proj"}),
		},
	}

	p := pipeline.New(runner, &fakeResolver{result: resolvedResult("v1.1.0")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	gh, err := os.ReadFile(ghNotes)
	require.NoError(t, err, "gh fake should have captured --notes")
	gl, err := os.ReadFile(glNotes)
	require.NoError(t, err, "glab fake should have captured --notes")

	// Each platform's notes carry that platform's own host (proving per-platform
	// generation through the real exec env path) and not the other's.
	assert.Equal(t, "https://github.com/test/gh-repo", string(gh))
	assert.Equal(t, "https://gitlab.com/test/gl-proj", string(gl))
	assert.NotContains(t, string(gh), "gitlab.com")
	assert.NotContains(t, string(gl), "github.com")
}

// captureNotesScript returns a /bin/sh FakeBin that writes the value following the first
// --notes flag to the file named by the given environment variable.
func captureNotesScript(captureEnvVar string) string {
	return `#!/bin/sh
prev=""
for a in "$@"; do
  if [ "$prev" = "--notes" ]; then printf '%s' "$a" > "$` + captureEnvVar + `"; fi
  prev="$a"
done
exit 0
`
}
