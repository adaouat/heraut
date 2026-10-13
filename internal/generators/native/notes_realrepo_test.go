package native_test

import (
	"os"
	"os/exec"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
)

func notesGenerator(t *testing.T, drv *config.ContentDriver, opts ...native.Option) *native.Generator {
	t.Helper()
	gitFixture(t)
	tag := exec.Command("git", "tag", "-a", "-m", "v0.2.0", "v0.2.0")
	tag.Env = append(os.Environ(), "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := tag.CombinedOutput()
	require.NoError(t, err, string(out))
	return native.New(execadapter.New(false, false), drv, native.ModeNotes, opts...)
}

func TestNotes_RealRepo_DefaultLayout(t *testing.T) {
	g := notesGenerator(t, &config.ContentDriver{})

	out, err := g.Generate("v0.1.0", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "### ")
	assert.Contains(t, out, "- *(auth)* Add login")
	assert.Contains(t, out, "- *(auth)* Add logout")
	assert.Contains(t, out, "- Correct null check")
	assert.NotContains(t, out, "## [", "no release header")
	assert.NotContains(t, out, "by @", "no author")
	assert.NotContains(t, out, "](", "no hash or PR link")
	assert.NotContains(t, out, "bump some library", "rendering.excludes still apply")
}

func TestNotes_RealRepo_RangeIsThePreviousTagToTheTag(t *testing.T) {
	g := notesGenerator(t, &config.ContentDriver{})

	out, err := g.Generate("v0.2.0", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "Redesign the page")
	assert.NotContains(t, out, "Add login", "commits before v0.1.0 belong to v0.1.0")
}

func TestNotes_RealRepo_FromOverridesTheLowerBound(t *testing.T) {
	g := notesGenerator(t, &config.ContentDriver{}, native.WithFromTag("v0.1.0"))
	out, err := g.Generate("v0.2.0", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "Redesign the page")
	assert.NotContains(t, out, "Add login")

	g = notesGenerator(t, &config.ContentDriver{}, native.WithFromTag("v0.2.0"))
	out, err = g.Generate("v0.2.0", nil)
	require.NoError(t, err)
	assert.Empty(t, out, "an empty range prints nothing")
}

func TestNotes_RealRepo_NoSurvivingCommitIsEmpty(t *testing.T) {
	drv := &config.ContentDriver{Excludes: []config.Exclude{{Type: "feat"}, {Type: "revert"}}}
	g := notesGenerator(t, drv)

	out, err := g.Generate("v0.2.0", nil)
	require.NoError(t, err)
	assert.Empty(t, out)
}
