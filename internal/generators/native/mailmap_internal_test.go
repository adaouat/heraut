package native

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The commit walk reads the author through git's mailmap: %aN and %aE, never the raw %an / %ae.
func TestLogFormat_ReadsTheMailmappedAuthor(t *testing.T) {
	assert.True(t, strings.HasPrefix(logFormat, "%H%x01%aN%x01%aE%x01"), "logFormat: %q", logFormat)
	assert.NotContains(t, logFormat, "%an")
	assert.NotContains(t, logFormat, "%ae")
}

// mailmapRepo builds a repository where Ada released v1.0.0 with her work email, then committed for
// the next release with her personal one. With mailmap set, a .mailmap maps both to ada@example.com,
// so both the commit walk and the authors before v1.0.0 must read through it.
func mailmapRepo(t *testing.T, mailmap bool) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Chdir(t.TempDir())
	git := func(email string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Ada", "GIT_AUTHOR_EMAIL="+email,
			"GIT_COMMITTER_NAME=Ada", "GIT_COMMITTER_EMAIL="+email,
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v\n%s", args, out)
	}
	git("ada@example.com", "init", "-q")
	if mailmap {
		require.NoError(t, os.WriteFile(".mailmap", []byte(
			"Ada Lovelace <ada@example.com> <ada.lovelace@corp.example.com>\n"+
				"Ada Lovelace <ada@example.com> <ada@home.example.com>\n"), 0o644))
		git("ada.lovelace@corp.example.com", "add", ".mailmap")
	}
	git("ada.lovelace@corp.example.com", "commit", "-q", "--allow-empty", "-m", "feat: first")
	git("ada.lovelace@corp.example.com", "tag", "v1.0.0")
	git("ada@home.example.com", "commit", "-q", "--allow-empty", "-m", "fix: second")
}

// With a .mailmap, a returning contributor's second email is the same person: one identity in the
// commits, and not a new contributor.
func TestNewContributors_FollowTheMailmap(t *testing.T) {
	mailmapRepo(t, true)
	runner := execadapter.New(false, false)

	commits, err := collectCommits(runner, "v1.0.0..HEAD")
	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, "Ada Lovelace", commits[0].Author)
	assert.Equal(t, "ada@example.com", commits[0].Email)

	before, err := authorsBefore(runner, "v1.0.0")
	require.NoError(t, err)
	assert.True(t, before["ada@example.com"])

	assert.Empty(t, collectContributors(toParsedCommits(commits), before, nil), "Ada is not new in the second release")
}

// Without a .mailmap nothing changes: the second email is a new author.
func TestNewContributors_WithoutAMailmap(t *testing.T) {
	mailmapRepo(t, false)
	runner := execadapter.New(false, false)

	commits, err := collectCommits(runner, "v1.0.0..HEAD")
	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, "Ada", commits[0].Author)
	assert.Equal(t, "ada@home.example.com", commits[0].Email)

	before, err := authorsBefore(runner, "v1.0.0")
	require.NoError(t, err)
	got := collectContributors(toParsedCommits(commits), before, nil)
	require.Len(t, got, 1)
	assert.Equal(t, "ada@home.example.com", got[0].Author.Email)
}
