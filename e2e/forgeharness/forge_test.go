//go:build e2e_forge

package forgeharness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAPI installs name as an executable that logs "name arg arg…" per call and prints reply
// (the first entry of replies whose key is a substring of the joined args; list the generic keys last).
func fakeAPI(t *testing.T, name string, replies [][2]string) (calls func() []string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	var cases strings.Builder
	for _, kv := range replies {
		if strings.HasPrefix(kv[1], "!") {
			cases.WriteString("  *\"" + kv[0] + "\"*) echo '" + kv[1][1:] + "' >&2; exit 1 ;;\n")
			continue
		}
		cases.WriteString("  *\"" + kv[0] + "\"*) cat <<'JSON'\n" + kv[1] + "\nJSON\n;;\n")
	}
	script := "#!/bin/sh\necho \"$*\" >> '" + log + "'\ncase \"$*\" in\n" + cases.String() + "  *) echo '{}' ;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() []string {
		b, err := os.ReadFile(log)
		if err != nil {
			return nil
		}
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
}

func TestGitHubForge(t *testing.T) {
	calls := fakeAPI(t, "gh", [][2]string{
		{"releases/tags/nope", "!gh: Not Found (HTTP 404)"},
		{"releases?per_page=100", `[]`},
		{"releases/tags/e2e-1-aaaa-v0.1.0", `{"id":7,"tag_name":"e2e-1-aaaa-v0.1.0","body":"### Features","prerelease":true,"draft":false,"assets":[{"name":"a"}]}`},
		{"git/ref/tags/e2e-1-aaaa-v0.1.0", `{"object":{"type":"tag","sha":"TAGSHA"}}`},
		{"git/matching-refs/heads/e2e/", `[{"ref":"refs/heads/e2e/e2e-1-aaaa"}]`},
		{"git/matching-refs/tags/e2e-", `[{"ref":"refs/tags/e2e-1-aaaa-v0.1.0"}]`},
		{"git/tags/TAGSHA", `{"object":{"sha":"COMMITSHA"}}`},
		{"repos/acme/widget-testing", `{"private":true}`},
	})
	f := NewGitHub(Config{GitHubRepo: "acme/widget-testing", Pattern: "*testing*"}, "tok")

	assert.Equal(t, "github", f.Name())
	assert.Equal(t, "repository", f.CoordinatesKey())
	assert.Equal(t, "GH_TOKEN", f.TokenEnv())
	assert.True(t, f.HasPreReleaseFlag())
	assert.Equal(t, "a%2Bb/c", f.URLTag("a+b/c"), "GitHub release URLs escape + but keep /")
	assert.Equal(t, "https://github.com/acme/widget-testing.git", f.CloneURL())
	assert.NotContains(t, f.CloneURL(), "tok", "the token never goes into the URL")
	assert.True(t, strings.HasPrefix(f.GitAuthHeader(), "Authorization: Basic "))

	require.NoError(t, f.Check())
	rel, ok, err := f.Release("e2e-1-aaaa-v0.1.0")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, Release{Tag: "e2e-1-aaaa-v0.1.0", Body: "### Features", Prerelease: true, AssetCount: 1}, rel)
	_, ok, err = f.Release("nope")
	require.NoError(t, err)
	assert.False(t, ok)
	sha, err := f.TagCommit("e2e-1-aaaa-v0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "COMMITSHA", sha, "an annotated tag is peeled to its commit")
	branches, _ := f.Branches("e2e/")
	tags, _ := f.Tags("e2e-")
	assert.Equal(t, []string{"e2e/e2e-1-aaaa"}, branches)
	assert.Equal(t, []string{"e2e-1-aaaa-v0.1.0"}, tags)

	require.NoError(t, f.DeleteRelease("e2e-1-aaaa-v0.1.0"))
	require.NoError(t, f.DeleteTag("e2e-1-aaaa-v0.1.0"))
	require.NoError(t, f.DeleteBranch("e2e/e2e-1-aaaa"))
	all := strings.Join(calls(), "\n")
	assert.Contains(t, all, "api -X DELETE repos/acme/widget-testing/releases/7")
	assert.Contains(t, all, "api -X DELETE repos/acme/widget-testing/git/refs/tags/e2e-1-aaaa-v0.1.0")
	assert.Contains(t, all, "api -X DELETE repos/acme/widget-testing/git/refs/heads/e2e/e2e-1-aaaa")
}

func TestGitHubCheckRefusesAPublicOrMisnamedRepo(t *testing.T) {
	fakeAPI(t, "gh", [][2]string{{"repos/acme/widget-testing", `{"private":false}`}})

	err := NewGitHub(Config{GitHubRepo: "acme/widget-testing", Pattern: "*testing*"}, "tok").Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not private")

	err = NewGitHub(Config{GitHubRepo: "acme/widget", Pattern: "*testing*"}, "tok").Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox pattern")
}

func TestGitLabForge(t *testing.T) {
	calls := fakeAPI(t, "glab", [][2]string{
		{"projects/group%2Fwidget-testing/releases/e2e-1-aaaa%2Fuat%2F0.1.0", `{"tag_name":"e2e-1-aaaa/uat/0.1.0","description":"### Features","assets":{"links":[{"name":"a"}]}}`},
		{"repository/tags/e2e-1-aaaa%2Fuat%2F0.1.0", `{"commit":{"id":"COMMITSHA"}}`},
		{"repository/branches?search=e2e%2F", `[{"name":"e2e/e2e-1-aaaa"},{"name":"other"}]`},
		{"repository/tags?search=e2e-", `[{"name":"e2e-1-aaaa/uat/0.1.0"},{"name":"v1"}]`},
		{"projects/group%2Fwidget-testing", `{"visibility":"private"}`},
	})
	f := NewGitLab(Config{GitLabProject: "group/widget-testing", Pattern: "*testing*"}, "tok")

	assert.Equal(t, "gitlab", f.Name())
	assert.Equal(t, "project", f.CoordinatesKey())
	assert.Equal(t, "GITLAB_TOKEN", f.TokenEnv())
	assert.False(t, f.HasPreReleaseFlag())
	assert.Equal(t, "a%2Bb%2Fc", f.URLTag("a+b/c"), "GitLab release URLs escape + and /")
	assert.Equal(t, "https://gitlab.com/group/widget-testing.git", f.CloneURL())

	require.NoError(t, f.Check())
	rel, ok, err := f.Release("e2e-1-aaaa/uat/0.1.0")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, Release{Tag: "e2e-1-aaaa/uat/0.1.0", Body: "### Features", AssetCount: 1}, rel)
	sha, err := f.TagCommit("e2e-1-aaaa/uat/0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "COMMITSHA", sha)
	branches, _ := f.Branches("e2e/")
	tags, _ := f.Tags("e2e-")
	assert.Equal(t, []string{"e2e/e2e-1-aaaa"}, branches, "the search is a substring match, so non-prefixed names are dropped")
	assert.Equal(t, []string{"e2e-1-aaaa/uat/0.1.0"}, tags)

	require.NoError(t, f.DeleteRelease("e2e-1-aaaa/uat/0.1.0"))
	require.NoError(t, f.DeleteTag("e2e-1-aaaa/uat/0.1.0"))
	require.NoError(t, f.DeleteBranch("e2e/e2e-1-aaaa"))
	all := strings.Join(calls(), "\n")
	assert.Contains(t, all, "api -X DELETE projects/group%2Fwidget-testing/releases/e2e-1-aaaa%2Fuat%2F0.1.0")
	assert.Contains(t, all, "api -X DELETE projects/group%2Fwidget-testing/repository/tags/e2e-1-aaaa%2Fuat%2F0.1.0")
	assert.Contains(t, all, "api -X DELETE projects/group%2Fwidget-testing/repository/branches/e2e%2Fe2e-1-aaaa")
}

func TestGitHubReleaseLookupFallsBackToDraftsAndIsNotFooledByTheTagName(t *testing.T) {
	calls := fakeAPI(t, "gh", [][2]string{
		{"releases/tags/e2e-1-aaaa-v0.2.0", "!gh: Not Found (HTTP 404)"},
		{"releases/tags/e2e-1-aaaa-v0.1.0+158404", "!gh: Server Error (HTTP 500)"},
		{"releases?per_page=100", `[{"id":9,"tag_name":"e2e-1-aaaa-v0.2.0","draft":true,"assets":[]}]`},
	})
	f := NewGitHub(Config{GitHubRepo: "acme/widget-testing", Pattern: "*testing*"}, "tok")

	rel, ok, err := f.Release("e2e-1-aaaa-v0.2.0")
	require.NoError(t, err)
	require.True(t, ok, "the by-tag endpoint 404s for a draft: the list is consulted")
	assert.True(t, rel.Draft)
	require.NoError(t, f.DeleteRelease("e2e-1-aaaa-v0.2.0"))
	assert.Contains(t, strings.Join(calls(), "\n"), "api -X DELETE repos/acme/widget-testing/releases/9")

	_, ok, err = f.Release("e2e-1-aaaa-v0.1.0+158404")
	assert.Error(t, err, "a 500 is an error even though the tag name contains 404-looking digits")
	assert.False(t, ok)
	assert.Error(t, f.DeleteRelease("e2e-1-aaaa-v0.1.0+158404"), "and deleting must not treat it as already gone")
}

func TestGitLabDeleteReleaseIsIdempotentAndURLsAreSpelledPerForge(t *testing.T) {
	fakeAPI(t, "glab", [][2]string{
		{"releases/gone", "!glab: 404 Not Found (HTTP 404)"},
		{"releases/broken", "!glab: Internal Server Error (HTTP 500)"},
	})
	f := NewGitLab(Config{GitLabProject: "group/widget-testing", Pattern: "*testing*"}, "tok")

	assert.NoError(t, f.DeleteRelease("gone"), "a tag that never got a release is not an error")
	assert.Error(t, f.DeleteRelease("broken"))
	assert.Equal(t, "https://gitlab.com/group/widget-testing/-/releases/v1%2B2", f.ReleaseURL("v1+2"))
	gh := NewGitHub(Config{GitHubRepo: "acme/widget-testing"}, "tok")
	assert.Equal(t, "https://github.com/acme/widget-testing/releases/tag/v1%2B2", gh.ReleaseURL("v1+2"))
}
