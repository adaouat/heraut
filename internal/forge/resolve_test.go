package forge_test

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolve_GitLabCIZeroConfig(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{
		"GITLAB_CI":       "true",
		"CI_SERVER_URL":   "https://gitlab.example.com",
		"CI_API_V4_URL":   "https://gitlab.example.com/api/v4",
		"CI_PROJECT_PATH": "group/subgroup/project",
		"CI_JOB_TOKEN":    "jobtok",
	}), "")
	require.NoError(t, err)
	require.Len(t, got.Forges, 1)
	f := got.Forges[got.EnrichmentIndex]
	assert.Equal(t, "gitlab", f.Type)
	assert.Equal(t, "https://gitlab.example.com", f.Host)
	assert.Equal(t, "https://gitlab.example.com/api/v4", f.APIURL)
	assert.Equal(t, "group/subgroup/project", f.Project)
	assert.Equal(t, "jobtok", f.Token)
	assert.Equal(t, port.TokenJob, f.TokenKind)
	assert.Equal(t, "rest", f.APIMode)
}

func TestResolve_GitOriginLocalGitLab(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{"GITLAB_TOKEN": "pat"}),
		"git@gitlab.com:group/subgroup/project.git")
	require.NoError(t, err)
	require.Len(t, got.Forges, 1)
	f := got.Forges[0]
	assert.Equal(t, "gitlab", f.Type)
	assert.Equal(t, "https://gitlab.com", f.Host)
	assert.Equal(t, "group/subgroup/project", f.Project)
	assert.Equal(t, "pat", f.Token)
	assert.Equal(t, port.TokenPrivate, f.TokenKind)
}

func TestResolve_AmbiguousZeroConfig(t *testing.T) {
	_, err := forge.Resolve(&config.Config{}, env(map[string]string{
		"GITLAB_TOKEN": "a", "GITHUB_TOKEN": "b",
	}), "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, forge.ErrAmbiguousForge))
}

// Modern form: SYSTEM_COLLECTIONURI carries the org as its first path segment
// ("https://dev.azure.com/myorg/"). The org must appear exactly once across Host+Project — Host
// keeps the org path segment (there is nowhere else to park it), so Project is the team project
// alone. This is the ground truth from the C2 fix: composing Host+"/"+Project must never
// double the org (regression coverage for the duplicated-org bug).
func TestResolve_AzureCIZeroConfig(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{
		"TF_BUILD":              "true",
		"SYSTEM_COLLECTIONURI":  "https://dev.azure.com/myorg/",
		"SYSTEM_TEAMPROJECT":    "myproject",
		"BUILD_REPOSITORY_NAME": "myrepo",
		"SYSTEM_ACCESSTOKEN":    "tok",
	}), "")
	require.NoError(t, err)
	require.Len(t, got.Forges, 1)
	f := got.Forges[got.EnrichmentIndex]
	assert.Equal(t, "azure_devops", f.Type)
	assert.Equal(t, "https://dev.azure.com/myorg", f.Host)
	assert.Equal(t, "myproject", f.Project)
	assert.Equal(t, "myrepo", f.Repository)
	assert.Equal(t, "tok", f.Token)
	assert.Equal(t, port.TokenPrivate, f.TokenKind)
}

// Legacy form: SYSTEM_COLLECTIONURI carries the org as the Host's subdomain
// ("https://myorg.visualstudio.com/"), with no org path segment. Project must NOT re-add the org
// as a path segment — Host's subdomain is the only place it appears — or the org is duplicated
// (the exact C2 bug: API path became /myorg/myorg/myproject/...).
func TestResolve_AzureCIZeroConfigLegacyCollectionURI(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{
		"TF_BUILD":              "true",
		"SYSTEM_COLLECTIONURI":  "https://myorg.visualstudio.com/",
		"SYSTEM_TEAMPROJECT":    "myproject",
		"BUILD_REPOSITORY_NAME": "myrepo",
		"SYSTEM_ACCESSTOKEN":    "tok",
	}), "")
	require.NoError(t, err)
	require.Len(t, got.Forges, 1)
	f := got.Forges[got.EnrichmentIndex]
	assert.Equal(t, "azure_devops", f.Type)
	assert.Equal(t, "https://myorg.visualstudio.com", f.Host)
	assert.Equal(t, "myproject", f.Project)
	assert.Equal(t, "myrepo", f.Repository)
}

func TestResolve_ExplicitForgeFillsFromCI(t *testing.T) {
	cfg := &config.Config{Forges: []config.Forge{{Name: "P", Type: "gitlab", APIMode: "graphql", TokenEnv: "MY_PAT"}}}
	got, err := forge.Resolve(cfg, env(map[string]string{
		"GITLAB_CI": "true", "CI_SERVER_URL": "https://gitlab.example.com",
		"CI_API_V4_URL": "https://gitlab.example.com/api/v4", "CI_PROJECT_PATH": "group/project",
		"MY_PAT": "pat",
	}), "")
	require.NoError(t, err)
	f := got.Forges[0]
	assert.Equal(t, "https://gitlab.example.com", f.Host) // filled from CI
	assert.Equal(t, "group/project", f.Project)           // filled from CI
	assert.Equal(t, "pat", f.Token)                       // token_env wins
	assert.Equal(t, port.TokenPrivate, f.TokenKind)       // explicit token_env → private
	assert.Equal(t, "graphql", f.APIMode)                 // explicit
}

func TestResolve_ExplicitAzureForgeSplitsProjectAndRepository(t *testing.T) {
	cfg := &config.Config{Forges: []config.Forge{{
		Name: "A", Type: "azure_devops", Project: "myorg/myproject", Repository: "myrepo",
	}}}
	got, err := forge.Resolve(cfg, env(nil), "")
	require.NoError(t, err)
	f := got.Forges[0]
	assert.Equal(t, "myorg/myproject", f.Project)
	assert.Equal(t, "myrepo", f.Repository)
}

// I1: an explicit azure_devops forge entry with no repository: must still fall back to the CI's
// BUILD_REPOSITORY_NAME, the same way Host/APIURL/Project already fall back to CI. Before this
// fix, Repository was read from config only (detectCIForge's repository return was discarded),
// so this case produced Repository == "" and a malformed request/links downstream.
func TestResolve_ExplicitAzureForgeRepositoryFallsBackToCI(t *testing.T) {
	cfg := &config.Config{Forges: []config.Forge{{
		Name: "A", Type: "azure_devops", Project: "myorg/myproject",
	}}}
	got, err := forge.Resolve(cfg, env(map[string]string{
		"TF_BUILD":              "true",
		"SYSTEM_COLLECTIONURI":  "https://dev.azure.com/myorg/",
		"SYSTEM_TEAMPROJECT":    "myproject",
		"BUILD_REPOSITORY_NAME": "myrepo",
	}), "")
	require.NoError(t, err)
	f := got.Forges[0]
	assert.Equal(t, "myrepo", f.Repository)
}

func TestResolve_NoForgeOffline(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{}), "")
	require.NoError(t, err)
	assert.Empty(t, got.Forges)
}
