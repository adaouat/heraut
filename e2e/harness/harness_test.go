package harness

import (
	"encoding/json"
	"io"
	"net/http"
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

	r.RestoreRemote("HEAD")
	assert.Equal(t, r.Git("rev-parse", "HEAD"), r.GitRemote("rev-parse", "main"), "RestoreRemote recreates origin at the given rev")
	assert.Empty(t, r.GitRemote("tag", "-l"), "a restored remote starts without tags")
}

func TestRepo_FakeCLIRecordsCallsInOrderAndCanFail(t *testing.T) {
	r := NewRepo(t)
	r.FakeCLI("gh")
	r.FakeCLI("tea")

	res := r.Run("/bin/sh", nil, "-c", "gh --version; tea release create v1 --repo acme/w; gh release create v1; echo rc=$?")
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, []string{
		"gh [--version]",
		"tea [release] [create] [v1] [--repo] [acme/w]",
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

func TestRepo_FakeCLIWorksWithOnlyItsOwnDirOnPath(t *testing.T) {
	r := NewRepo(t)
	r.FakeCLI("gh")

	res := r.Run("/bin/sh", []string{"PATH=" + r.BinDir()}, "-c", "gh --version")

	assert.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Equal(t, []string{"gh [--version]"}, r.CLICalls(), "the call is logged next to the fake, not at /")
}

func TestRepo_FakeGitLabAnswersAndLogsNextToTheFakeCLIs(t *testing.T) {
	r := NewRepo(t)
	r.FakeCLI("gh")
	gl := r.FakeGitLab()
	assert.Same(t, gl, r.FakeGitLab(), "a repository has one stand-in")

	res := r.Run("/bin/sh", nil, "-c", "gh --version")
	require.Equal(t, 0, res.ExitCode)
	do := func(method, path, body string) (int, string) {
		req, err := http.NewRequest(method, gl.URL+path, strings.NewReader(body))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	code, _ := do(http.MethodGet, "/api/v4/user", "")
	assert.Equal(t, 200, code)
	code, body := do(http.MethodGet, "/api/v4/projects/42/releases?per_page=1", "")
	assert.Equal(t, 200, code)
	assert.Equal(t, "[]", body, "the releases list is an array")
	code, _ = do(http.MethodPut, "/api/v4/projects/acme%2Fwidget/packages/generic/release-assets/v1/a.txt", "bin")
	assert.Equal(t, 201, code)
	code, _ = do(http.MethodPost, "/api/v4/projects/acme%2Fwidget/releases", `{"tag_name":"v1"}`)
	assert.Equal(t, 201, code)

	assert.Equal(t, []string{
		"gh [--version]",
		"gitlab [GET] [/api/v4/user]",
		"gitlab [GET] [/api/v4/projects/42/releases]",
		"gitlab [PUT] [/api/v4/projects/acme/widget/packages/generic/release-assets/v1/a.txt]",
		"gitlab [POST] [/api/v4/projects/acme/widget/releases]",
	}, r.CLICalls(), "requests are logged in order, with decoded paths and no query")
	assert.Equal(t, "v1", gl.Release()["tag_name"])

	gl.FailReleases()
	code, _ = do(http.MethodPost, "/api/v4/projects/acme%2Fwidget/releases", `{"tag_name":"v2"}`)
	assert.Equal(t, 403, code)
	assert.Equal(t, "gitlab [POST] [/api/v4/projects/acme/widget/releases]", r.CLICalls()[5], "a failing call is still recorded")
	assert.Equal(t, "v1", gl.Release()["tag_name"], "a refused release is not kept")
}

func TestRepo_FakeGitHubAnswersLikeAnEnterpriseServerAndLogs(t *testing.T) {
	r := NewRepo(t)
	gh := r.FakeGitHub()
	assert.Same(t, gh, r.FakeGitHub(), "a repository has one stand-in")
	do := func(method, url, body string) (int, map[string]any) {
		req, err := http.NewRequest(method, url, strings.NewReader(body))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		var m map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&m)
		return resp.StatusCode, m
	}

	code, _ := do(http.MethodGet, gh.URL+"/api/v3/repos/acme/widget/releases?per_page=1", "")
	assert.Equal(t, 200, code)
	code, rel := do(http.MethodPost, gh.URL+"/api/v3/repos/acme/widget/releases", `{"tag_name":"v1","draft":true}`)
	assert.Equal(t, 201, code)
	assert.Equal(t, gh.URL+"/api/uploads/repos/acme/widget/releases/1/assets{?name,label}", rel["upload_url"], "uploads go to the same origin")
	code, asset := do(http.MethodPost, gh.URL+"/api/uploads/repos/acme/widget/releases/1/assets?name=meta%231.json", "{}")
	assert.Equal(t, 201, code)
	assert.Equal(t, "meta.1.json", asset["name"], "GitHub rewrites special characters")
	code, _ = do(http.MethodPatch, gh.URL+"/api/v3/repos/acme/widget/releases/1", `{"draft":false}`)
	assert.Equal(t, 200, code)

	assert.Equal(t, []string{
		"github [GET] [/api/v3/repos/acme/widget/releases]",
		"github [POST] [/api/v3/repos/acme/widget/releases]",
		"github [POST] [/api/uploads/repos/acme/widget/releases/1/assets]",
		"github [PATCH] [/api/v3/repos/acme/widget/releases/1]",
	}, r.CLICalls(), "requests are logged in order, with no query")
	assert.Equal(t, "v1", gh.Release()["tag_name"])
	assert.Equal(t, []string{"meta#1.json"}, gh.Uploads(), "the names as sent")

	gh.FailUploads()
	code, _ = do(http.MethodPost, gh.URL+"/api/uploads/repos/acme/widget/releases/1/assets?name=b.txt", "b")
	assert.Equal(t, 422, code)
	code, _ = do(http.MethodDelete, gh.URL+"/api/v3/repos/acme/widget/releases/1", "")
	assert.Equal(t, 204, code)
	gh.FailReleases()
	code, _ = do(http.MethodPost, gh.URL+"/api/v3/repos/acme/widget/releases", `{"tag_name":"v2"}`)
	assert.Equal(t, 403, code)
	assert.Equal(t, "v1", gh.Release()["tag_name"], "a refused release is not kept")
	assert.Equal(t, []string{"meta#1.json"}, gh.Uploads(), "a refused upload is not kept")
	assert.Len(t, r.CLICalls(), 7, "failing calls are still recorded")
}
