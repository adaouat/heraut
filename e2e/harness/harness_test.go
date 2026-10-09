package harness

import (
	"os"
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

func TestRepo_Checkout(t *testing.T) {
	r := NewRepo(t)
	r.Commit("feat: first")

	r.Checkout("develop")

	assert.Equal(t, "develop", strings.TrimSpace(r.git("rev-parse", "--abbrev-ref", "HEAD")))
}

func TestRepo_SwitchDetachAndMerge(t *testing.T) {
	r := NewRepo(t)
	r.Commit("feat: first")
	r.Checkout("release/1.3")
	r.Commit("fix: on the line")
	r.Switch("main")

	assert.Equal(t, "main", strings.TrimSpace(r.git("rev-parse", "--abbrev-ref", "HEAD")))

	r.MergeNoFF("release/1.3")
	assert.Equal(t, "chore: merge release/1.3", strings.TrimSpace(r.git("log", "-1", "--format=%s")))
	assert.Len(t, strings.Fields(r.git("log", "-1", "--format=%P")), 2, "a no-ff merge has two parents")

	r.Detach()
	assert.Equal(t, "HEAD", strings.TrimSpace(r.git("rev-parse", "--abbrev-ref", "HEAD")))
}

func TestRepo_RemoteAndFileHelpers(t *testing.T) {
	r := NewRepo(t)
	r.Commit("feat: first")
	r.AddRemote()

	assert.Equal(t, r.Git("rev-parse", "HEAD"), r.GitRemote("rev-parse", "main"), "AddRemote pushes the branch")
	assert.Equal(t, "origin/main", r.Git("rev-parse", "--abbrev-ref", "main@{upstream}"), "the branch tracks origin")

	r.Tag("v1.0.0")
	r.Git("push", "origin", "v1.0.0")
	assert.Equal(t, "v1.0.0", r.GitRemote("tag", "-l"))

	r.WriteFile("docs/NOTES.md", "hello\n")
	assert.Equal(t, "hello\n", r.ReadFile("docs/NOTES.md"))

	r.RemoveRemote()
	_, err := os.Stat(r.remote)
	assert.True(t, os.IsNotExist(err), "the bare remote is gone")
}

func TestRepo_FakeCLIRecordsCallsInOrderAndCanFail(t *testing.T) {
	r := NewRepo(t)
	r.FakeCLI("gh")
	r.FakeCLI("glab")

	res := r.Run("/bin/sh", nil, "-c", "gh --version; glab release create v1 --repo acme/w; gh release create v1; echo rc=$?")
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, []string{
		"gh [--version]",
		"glab [release] [create] [v1] [--repo] [acme/w]",
		"gh [release] [create] [v1]",
	}, r.CLICalls())

	r.FailReleases("gh")
	res = r.Run("/bin/sh", nil, "-c", "gh release create v2; echo rc=$?")
	assert.Equal(t, "rc=1\n", res.Stdout)
	assert.Equal(t, "gh [release] [create] [v2]", r.CLICalls()[3], "a failing call is still recorded")
}

func TestRepo_HomeBinDirAndStdin(t *testing.T) {
	r := NewRepo(t)

	r.WriteHomeFile("cfg/x.yml", "hello\n")
	res := r.Run("/bin/sh", nil, "-c", "cat ~/cfg/x.yml")
	assert.Equal(t, "hello\n", res.Stdout, "the child sees the same HOME")
	assert.Equal(t, r.Home()+"/cfg/x.yml", strings.TrimSpace(r.Run("/bin/sh", nil, "-c", "echo $HOME/cfg/x.yml").Stdout))

	assert.Empty(t, r.BinDir())
	r.FakeCLI("gh")
	assert.NotEmpty(t, r.BinDir())
	assert.FileExists(t, r.BinDir()+"/gh")

	res = r.RunStdin("/bin/cat", nil, "from stdin\n")
	assert.Equal(t, "from stdin\n", res.Stdout)
}
