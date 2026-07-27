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

func TestResolve_NoForgeOffline(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{}), "")
	require.NoError(t, err)
	assert.Empty(t, got.Forges)
}
