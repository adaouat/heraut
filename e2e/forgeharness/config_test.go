//go:build e2e_forge

package forgeharness

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigAndGuard(t *testing.T) {
	t.Setenv("HERAUT_E2E_GITHUB_REPO", "acme/widget-testing")
	t.Setenv("HERAUT_E2E_GITLAB_PROJECT", "group/sub/widget-testing")
	t.Setenv("HERAUT_E2E_REPO_PATTERN", "")

	c := LoadConfig()

	assert.Equal(t, "acme/widget-testing", c.GitHubRepo)
	assert.Equal(t, "group/sub/widget-testing", c.GitLabProject)
	assert.Equal(t, "*testing*", c.Pattern, "the default pattern is a sandbox naming convention")
	assert.NoError(t, c.Guard("acme/widget-testing"))
	assert.NoError(t, c.Guard("group/sub/widget-testing"), "only the base name is matched")
	assert.Error(t, c.Guard("acme/widget"), "a production-looking name is refused")
	assert.Error(t, c.Guard("testing/widget"), "the owner part does not count")

	t.Setenv("HERAUT_E2E_REPO_PATTERN", "scratch-*")
	assert.NoError(t, LoadConfig().Guard("acme/scratch-one"))
	assert.Error(t, LoadConfig().Guard("acme/widget-testing"))
}

func TestRunIDs(t *testing.T) {
	id := NewRunID(time.Unix(1791548560, 0), "a1b2")

	assert.Equal(t, "e2e-1791548560-a1b2", id)
	assert.Len(t, RandomHex(), 4)

	ts, ok := RunIDTime("refs/heads/e2e/" + id + "-extra")
	require.True(t, ok)
	assert.Equal(t, int64(1791548560), ts.Unix())

	_, ok = RunIDTime("v1.2.3")
	assert.False(t, ok, "an ordinary tag carries no run id")
	_, ok = RunIDTime("e2e-notanumber-zz")
	assert.False(t, ok)
	for _, name := range []string{"e2e-20250101-beef", "foo-e2e-1791548560-a1b2", "e2e-1791548560-a1b2c"} {
		_, ok = RunIDTime(name)
		assert.False(t, ok, "%q is not one of our names", name)
	}
	for _, name := range []string{"e2e-1791548560-a1b2-v0.1.0", "e2e/e2e-1791548560-a1b2", "e2e-1791548560-a1b2/uat/0.1.0"} {
		_, ok = RunIDTime(name)
		assert.True(t, ok, "%q is one of our names", name)
	}
}

func writeFake(t *testing.T, dir, name, script string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755))
}

func TestTokensPreferTheEnvironmentThenTheCLILogin(t *testing.T) {
	dir := t.TempDir()
	writeFake(t, dir, "gh", `[ "$1 $2" = "auth token" ] && echo cli-gh-token`)
	writeFake(t, dir, "glab", `[ "$1 $2 $3" = "auth status --show-token" ] && echo "  ✓ Token found: cli-gl-token"`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, k := range []string{"HERAUT_E2E_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN", "HERAUT_E2E_GITLAB_TOKEN", "GITLAB_TOKEN"} {
		t.Setenv(k, "")
	}

	gh, err := GitHubToken()
	require.NoError(t, err)
	assert.Equal(t, "cli-gh-token", gh)
	gl, err := GitLabToken()
	require.NoError(t, err)
	assert.Equal(t, "cli-gl-token", gl)

	t.Setenv("GH_TOKEN", "env-gh")
	t.Setenv("HERAUT_E2E_GITLAB_TOKEN", "env-gl")
	gh, _ = GitHubToken()
	gl, _ = GitLabToken()
	assert.Equal(t, "env-gh", gh)
	assert.Equal(t, "env-gl", gl, "the dedicated variable wins")

	t.Setenv("HERAUT_E2E_GITHUB_TOKEN", "dedicated")
	gh, _ = GitHubToken()
	assert.Equal(t, "dedicated", gh)
}

func TestEnrichConfig(t *testing.T) {
	t.Setenv("HERAUT_E2E_GITHUB_REPO", "acme/widget-testing")
	t.Setenv("HERAUT_E2E_GITHUB_ENRICH_REPO", "acme/widget-testing-enrich")
	t.Setenv("HERAUT_E2E_GITLAB_ENRICH_PROJECT", "group/widget-testing-enrich")

	c := LoadConfig()
	e := c.ForEnrich("github")

	assert.Equal(t, "acme/widget-testing-enrich", c.GitHubEnrichRepo)
	assert.Equal(t, "group/widget-testing-enrich", c.GitLabEnrichProject)
	assert.Equal(t, "acme/widget-testing-enrich", e.GitHubRepo, "the enrichment copy targets the second pair")
	assert.Equal(t, "acme/widget-testing", c.GitHubRepo, "the original is untouched")
	assert.Equal(t, "group/widget-testing-enrich", c.ForEnrich("gitlab").GitLabProject)
}
