package commitwizard

import (
	"os"
	"strings"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/adaouat/heraut/internal/testutil"
)

// TestCommit_RealGit_RoundTrip is the one place the assembled message is verified to reach
// git intact: it runs the production exec.Runner against a real temporary repo, commits a
// staged change via commit() (git commit -F <tmpfile>), and reads the message back. The
// contract tests only assert the git arg shape; this closes the "bytes the user confirmed
// == bytes git recorded" gap. Skips when git is absent (RealGitRepo handles the skip).
func TestCommit_RealGit_RoundTrip(t *testing.T) {
	testutil.RealGitRepo(t, "v0.0.0") // chdirs into a fresh repo (initial commit + tag, no hooks)
	r := execadapter.New(false, false)

	require.NoError(t, os.WriteFile("file.txt", []byte("hi"), 0o644))
	_, _, err := r.Run("git", "add", "file.txt")
	require.NoError(t, err)

	msg := Assemble(Answers{
		Type:    "feat",
		Scope:   "cmd",
		Subject: "add wizard",
		Body:    "Guided prompts build the message.",
		Footers: []conventionalcommit.Footer{{Token: "Closes", Value: "#42"}},
	}).Format()

	require.NoError(t, commit(r, msg, false))

	out, _, err := r.Run("git", "log", "-1", "--format=%B")
	require.NoError(t, err)
	assert.Equal(t, msg, strings.TrimRight(out, "\n"))
}
