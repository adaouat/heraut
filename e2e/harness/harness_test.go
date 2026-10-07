package harness

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo_CommitTagAndConfig(t *testing.T) {
	r := NewRepo(t)
	r.Commit("feat: first")
	r.Tag("v1.0.0")
	r.WriteConfig("version: \"1\"\n")

	assert.Equal(t, "v1.0.0", strings.TrimSpace(r.git("tag", "-l")))
	assert.Equal(t, "tag", strings.TrimSpace(r.git("cat-file", "-t", "v1.0.0")), "tags must be annotated")
	assert.Equal(t, "feat: first", strings.TrimSpace(r.git("log", "-1", "--format=%s")))
	assert.FileExists(t, r.Dir+"/.heraut.yml")
}

func TestRun_ScrubsAmbientEnvAndReportsExit(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("CI_PROJECT_PATH", "acme/widget")
	r := NewRepo(t)

	res := r.Run("/usr/bin/env", []string{"E2E_EXTRA=1"})
	require.Equal(t, 0, res.ExitCode)
	assert.NotContains(t, res.Stdout, "GITHUB_ACTIONS")
	assert.NotContains(t, res.Stdout, "CI_PROJECT_PATH")
	assert.Contains(t, res.Stdout, "E2E_EXTRA=1")
	assert.Contains(t, res.Stdout, "NO_COLOR=1")
	assert.Contains(t, res.Stdout, "HERAUT_CHECK_UPDATE=false", "the hermetic lane must never reach the update-check endpoint")

	res = r.Run("/bin/sh", nil, "-c", "echo out; echo err >&2; exit 7")
	assert.Equal(t, 7, res.ExitCode)
	assert.Equal(t, "out\n", res.Stdout)
	assert.Equal(t, "err\n", res.Stderr)
}

func TestBinary_BuildsOncePerTagSet(t *testing.T) {
	t.Cleanup(Cleanup)
	plain := Binary(t)
	assert.Equal(t, plain, Binary(t), "same tag set must reuse the build")
	tagged := Binary(t, "heraut_testclock")
	assert.NotEqual(t, plain, tagged)
	assert.FileExists(t, plain)
	assert.FileExists(t, tagged)
}

func TestRepo_ReadFile(t *testing.T) {
	r := NewRepo(t)
	r.WriteConfig("version: \"1\"\n")

	assert.Equal(t, "version: \"1\"\n", r.ReadFile(".heraut.yml"))
}
