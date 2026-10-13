package cmd_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/testutil"
)

func TestNewNotesCmd(t *testing.T) {
	c := cmd.NewNotesCmd()
	assert.Equal(t, "notes <tag>", c.Use)
	for _, name := range []string{"from", "format", "template", "output", "env", "force", "offline"} {
		assert.NotNil(t, c.Flags().Lookup(name), "flag %q not registered", name)
	}
	assert.Equal(t, "markdown", c.Flags().Lookup("format").DefValue)
}

// notesRepo: v0.1.0 (the testutil fixture), then a fix commit tagged v0.2.0, with a config that
// keeps the run offline.
func notesRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	testutil.RealGitRepo(t, "v0.1.0")
	for _, args := range [][]string{
		{"commit", "--allow-empty", "-m", "fix(ui): stop the crash"},
		{"tag", "-a", "-m", "v0.2.0", "v0.2.0"},
	} {
		out, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	require.NoError(t, os.WriteFile(".heraut.yml", []byte("version: \"1\"\nversioning:\n  strategy: semver\n"), 0o644))
}

func runNotes(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := cmd.NewRootCmd("dev")
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"notes", "--offline"}, args...))
	_, err = root.ExecuteC()
	return out.String(), errb.String(), err
}

func TestNotes_PrintsTheRangeToStdoutOnly(t *testing.T) {
	notesRepo(t)

	stdout, stderr, err := runNotes(t, "v0.2.0")

	require.NoError(t, err)
	assert.Contains(t, stdout, "- *(ui)* Stop the crash")
	assert.NotContains(t, stdout, "## [", "no release heading")
	assert.Empty(t, stderr)
}

func TestNotes_TextFormat(t *testing.T) {
	notesRepo(t)

	stdout, _, err := runNotes(t, "v0.2.0", "--format", "text")

	require.NoError(t, err)
	assert.Contains(t, stdout, "- (ui) Stop the crash")
	assert.NotContains(t, stdout, "*")
	assert.NotContains(t, stdout, "#")
}

func TestNotes_OutputWritesTheFileAndPrintsNothing(t *testing.T) {
	notesRepo(t)
	file := filepath.Join(t.TempDir(), "store.txt")

	stdout, _, err := runNotes(t, "v0.2.0", "--format", "text", "--output", file)

	require.NoError(t, err)
	assert.Empty(t, stdout)
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(b), "- (ui) Stop the crash")
}

func TestNotes_OutputToAMissingDirectoryIsARuntimeError(t *testing.T) {
	notesRepo(t)
	file := filepath.Join(t.TempDir(), "missing", "store.txt")

	_, _, err := runNotes(t, "v0.2.0", "--output", file)

	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, exitcode.Resolve(err))
	assert.NoFileExists(t, file)
}

func TestNotes_TemplateFlagReplacesTheDefault(t *testing.T) {
	notesRepo(t)
	tmpl := filepath.Join(t.TempDir(), "notes.tmpl")
	require.NoError(t, os.WriteFile(tmpl, []byte(`{{ define "commit.message" }}* {{ .Description }}{{ end }}`), 0o644))

	stdout, _, err := runNotes(t, "v0.2.0", "--template", tmpl)

	require.NoError(t, err)
	assert.Contains(t, stdout, "* Stop the crash")
}

func TestNotes_ExitCodes(t *testing.T) {
	notesRepo(t)

	_, _, err := runNotes(t, "v9.9.9")
	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, exitcode.Resolve(err))
	assert.Contains(t, err.Error(), "v9.9.9")

	_, _, err = runNotes(t, "v0.2.0", "--format", "html")
	require.Error(t, err)
	assert.Equal(t, exitcode.Usage, exitcode.Resolve(err))
}

func TestNotes_AFailureShowsItsCause(t *testing.T) {
	notesRepo(t)

	_, _, err := runNotes(t, "v0.2.0", "--template", "/nonexistent/notes.tmpl")

	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, exitcode.Resolve(err))
	assert.Contains(t, err.Error(), "nonexistent/notes.tmpl", "the cause is not reduced to a summary")
}

func TestNotes_EmptyRangePrintsNothingAndNotices(t *testing.T) {
	notesRepo(t)

	stdout, stderr, err := runNotes(t, "v0.2.0", "--from", "v0.2.0")

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "no notable changes")
}

func TestNotes_EmptyRangeWithOutputWritesAnEmptyFile(t *testing.T) {
	notesRepo(t)
	file := filepath.Join(t.TempDir(), "store.txt")

	_, _, err := runNotes(t, "v0.2.0", "--from", "v0.2.0", "--output", file)

	require.NoError(t, err)
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Empty(t, b)
}
