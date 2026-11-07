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

// Modern form: SYSTEM_COLLECTIONURI carries the org as its last path segment
// ("https://dev.azure.com/myorg/"). The identity takes the shape of a forges: entry (ADR-0035):
// Host without the org, Project "org/project", so the org appears exactly once whichever source
// filled each field (C2, T444).
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
	assert.Equal(t, "https://dev.azure.com", f.Host)
	assert.Equal(t, "myorg/myproject", f.Project)
	assert.Equal(t, "myrepo", f.Repository)
	assert.Equal(t, "tok", f.Token)
	assert.Equal(t, port.TokenPrivate, f.TokenKind)
}

// Legacy form: SYSTEM_COLLECTIONURI carries the org as the Host's subdomain
// ("https://myorg.visualstudio.com/"). It resolves like a git origin on that host: dev.azure.com,
// which serves legacy organizations too, and Project "org/project".
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
	assert.Equal(t, "https://dev.azure.com", f.Host)
	assert.Equal(t, "myorg/myproject", f.Project)
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

// TestResolve_RejectsPlainHTTPToARemoteHost pins SEC-004: the token travels to the resolved host and
// API URL, so a CI variable or config naming a remote host over plain http is refused.
func TestResolve_RejectsPlainHTTPToARemoteHost(t *testing.T) {
	ci := func(server, api string) func(string) string {
		return env(map[string]string{
			"GITLAB_CI": "true", "CI_SERVER_URL": server, "CI_API_V4_URL": api,
			"CI_PROJECT_PATH": "group/project", "CI_JOB_TOKEN": "jobtok",
		})
	}
	t.Run("CI_SERVER_URL over http", func(t *testing.T) {
		_, err := forge.Resolve(&config.Config{}, ci("http://gitlab.example.com", "https://gitlab.example.com/api/v4"), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "https")
	})
	t.Run("CI_API_V4_URL over http", func(t *testing.T) {
		_, err := forge.Resolve(&config.Config{}, ci("https://gitlab.example.com", "http://gitlab.example.com/api/v4"), "")
		require.Error(t, err)
	})
	t.Run("config base_url over http", func(t *testing.T) {
		cfg := &config.Config{Forges: []config.Forge{{Name: "A", Type: "gitlab", BaseURL: "http://gitlab.example.com"}}}
		_, err := forge.Resolve(cfg, env(nil), "")
		require.Error(t, err)
	})
	t.Run("loopback over http is allowed", func(t *testing.T) {
		got, err := forge.Resolve(&config.Config{}, ci("http://127.0.0.1:8080", "http://127.0.0.1:8080/api/v4"), "")
		require.NoError(t, err)
		require.Len(t, got.Forges, 1)
	})
}

// A GitHub identity's API root follows the same rules as publishing (config.GitHubAPIURL): an
// explicit api_url wins; else GITHUB_API_URL, but only on the instance the forge points at; else
// the host's own API root. Enrichment and publishing then query the same instance.
func TestResolve_GitHubAPIRoot(t *testing.T) {
	actionsOn := func(server, api string) map[string]string {
		return map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_SERVER_URL": server, "GITHUB_API_URL": api, "GITHUB_REPOSITORY": "acme/widget"}
	}
	explicit := func(baseURL, apiURL string) *config.Config {
		return &config.Config{Forges: []config.Forge{{Name: "gh", Type: "github", Repository: "acme/widget", BaseURL: baseURL, APIURL: apiURL}}}
	}
	tests := []struct {
		name   string
		cfg    *config.Config
		env    map[string]string
		origin string
		want   string
	}{
		{name: "GHES forge run on a github.com runner keeps its own API",
			cfg: explicit("https://ghes.example.com", ""), env: actionsOn("https://github.com", "https://api.github.com"),
			want: "https://ghes.example.com/api/v3"},
		{name: "GHES forge run on its own runner takes GITHUB_API_URL",
			cfg: explicit("https://ghes.example.com", ""), env: actionsOn("https://ghes.example.com", "https://ghes-api.example.com"),
			want: "https://ghes-api.example.com"},
		{name: "an explicit api_url wins over GITHUB_API_URL",
			cfg: explicit("https://ghes.example.com", "https://proxy.example.com/api"), env: actionsOn("https://ghes.example.com", "https://ghes-api.example.com"),
			want: "https://proxy.example.com/api"},
		{name: "a ghe.com tenant gets api.<host>",
			cfg: explicit("https://acme.ghe.com", ""), env: map[string]string{},
			want: "https://api.acme.ghe.com"},
		{name: "github.com forge with no CI",
			cfg: explicit("", ""), env: map[string]string{},
			want: "https://api.github.com"},
		{name: "zero config in Actions takes GITHUB_API_URL",
			cfg: &config.Config{}, env: actionsOn("https://acme.ghe.com", "https://api.acme.ghe.com"),
			want: "https://api.acme.ghe.com"},
		{name: "zero config in Actions without GITHUB_SERVER_URL ignores GITHUB_API_URL",
			cfg: &config.Config{}, env: map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_API_URL": "https://api.elsewhere.example.com"},
			want: "https://api.github.com"},
		{name: "zero config from the git origin",
			cfg: &config.Config{}, env: map[string]string{"GITHUB_TOKEN": "t"}, origin: "git@github.com:acme/widget.git",
			want: "https://api.github.com"},
		{name: "zero config from a lone token",
			cfg: &config.Config{}, env: map[string]string{"GITHUB_TOKEN": "t"},
			want: "https://api.github.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := forge.Resolve(tc.cfg, env(tc.env), tc.origin)
			require.NoError(t, err)
			require.Len(t, got.Forges, 1)
			assert.Equal(t, tc.want, got.Forges[0].APIURL)
		})
	}
}

func TestResolve_GitHubInsecureActionsAPIURLIsRefused(t *testing.T) {
	_, err := forge.Resolve(&config.Config{}, env(map[string]string{
		"GITHUB_ACTIONS": "true", "GITHUB_SERVER_URL": "https://ghes.example.com", "GITHUB_API_URL": "http://ghes.example.com/api/v3",
	}), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITHUB_API_URL")
}

// Azure DevOps shows several clone URLs for one repository; each must resolve to the same identity,
// with the repository split from the project and the host on dev.azure.com (the legacy
// visualstudio.com host serves the same organization there).
func TestResolve_GitOriginAzure(t *testing.T) {
	tests := []struct {
		name, origin, project, repository string
	}{
		{"clone URL with the organization as user", "https://myorg@dev.azure.com/myorg/myproject/_git/myrepo", "myorg/myproject", "myrepo"},
		{"https without user", "https://dev.azure.com/myorg/myproject/_git/myrepo", "myorg/myproject", "myrepo"},
		{"ssh", "git@ssh.dev.azure.com:v3/myorg/myproject/myrepo", "myorg/myproject", "myrepo"},
		{"legacy host", "https://myorg.visualstudio.com/myproject/_git/myrepo", "myorg/myproject", "myrepo"},
		{"legacy host with DefaultCollection", "https://myorg.visualstudio.com/DefaultCollection/myproject/_git/myrepo", "myorg/myproject", "myrepo"},
		{"legacy ssh", "myorg@vs-ssh.visualstudio.com:v3/myorg/myproject/myrepo", "myorg/myproject", "myrepo"},
		{"repository named after its project", "https://dev.azure.com/myorg/_git/myrepo", "myorg/myrepo", "myrepo"},
		{"escaped spaces", "https://dev.azure.com/myorg/My%20Project/_git/My%20Repo", "myorg/My Project", "My Repo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := forge.Resolve(&config.Config{}, env(map[string]string{"AZURE_DEVOPS_TOKEN": "pat"}), tc.origin)
			require.NoError(t, err)
			require.Len(t, got.Forges, 1)
			f := got.Forges[0]
			assert.Equal(t, "azure_devops", f.Type)
			assert.Equal(t, "https://dev.azure.com", f.Host)
			assert.Equal(t, tc.project, f.Project)
			assert.Equal(t, tc.repository, f.Repository)
			assert.Equal(t, "pat", f.Token)
		})
	}
}

func TestResolve_GitOriginAzureUnknownLayoutIsNotDetected(t *testing.T) {
	for _, origin := range []string{
		"https://dev.azure.com/myorg/myproject",
		"https://dev.azure.com/myorg/myproject/_wiki/wikis/x",
		"git@ssh.dev.azure.com:v2/myorg/myproject/myrepo",
		"git@ssh.dev.azure.com:v3/myorg/myrepo",
	} {
		t.Run(origin, func(t *testing.T) {
			got, err := forge.Resolve(&config.Config{}, env(nil), origin)
			require.NoError(t, err)
			assert.Empty(t, got.Forges)
		})
	}
}

func TestResolve_ExplicitAzureForgeFillsFromGitOrigin(t *testing.T) {
	cfg := &config.Config{Forges: []config.Forge{{Name: "A", Type: "azure_devops"}}}
	got, err := forge.Resolve(cfg, env(nil), "git@ssh.dev.azure.com:v3/myorg/myproject/myrepo")
	require.NoError(t, err)
	f := got.Forges[0]
	assert.Equal(t, "https://dev.azure.com", f.Host)
	assert.Equal(t, "myorg/myproject", f.Project)
	assert.Equal(t, "myrepo", f.Repository)
}

// A token embedded in an https origin (a CI checkout) is not part of the host.
func TestResolve_GitOriginWithCredentials(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(nil), "https://x-access-token:secret@github.com/owner/repo.git")
	require.NoError(t, err)
	require.Len(t, got.Forges, 1)
	assert.Equal(t, "github", got.Forges[0].Type)
	assert.Equal(t, "owner/repo", got.Forges[0].Project)
}

// Azure DevOps Server: the collection is the last segment of SYSTEM_COLLECTIONURI and plays the
// organization's part, so a forges: entry for it reads base_url: <server>, project: collection/project.
func TestResolve_AzureCIZeroConfigServerCollection(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{
		"TF_BUILD":              "true",
		"SYSTEM_COLLECTIONURI":  "https://ado.example.com/tfs/DefaultCollection/",
		"SYSTEM_TEAMPROJECT":    "myproject",
		"BUILD_REPOSITORY_NAME": "myrepo",
	}), "")
	require.NoError(t, err)
	f := got.Forges[0]
	assert.Equal(t, "https://ado.example.com/tfs", f.Host)
	assert.Equal(t, "DefaultCollection/myproject", f.Project)
}

// An explicit entry filled partly from Azure Pipelines names the organization once, whichever
// fields the entry sets: Host and Project now share the shape Spec 02 documents (T444).
func TestResolve_ExplicitAzureForgeInAzurePipelines(t *testing.T) {
	ci := env(map[string]string{
		"TF_BUILD":              "true",
		"SYSTEM_COLLECTIONURI":  "https://dev.azure.com/myorg/",
		"SYSTEM_TEAMPROJECT":    "myproject",
		"BUILD_REPOSITORY_NAME": "myrepo",
	})
	tests := []struct {
		name  string
		entry config.Forge
	}{
		{"project and repository set", config.Forge{Name: "A", Type: "azure_devops", Project: "myorg/myproject", Repository: "myrepo"}},
		{"base_url set", config.Forge{Name: "A", Type: "azure_devops", BaseURL: "https://dev.azure.com"}},
		{"nothing set", config.Forge{Name: "A", Type: "azure_devops"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := forge.Resolve(&config.Config{Forges: []config.Forge{tc.entry}}, ci, "")
			require.NoError(t, err)
			f := got.Forges[0]
			assert.Equal(t, "https://dev.azure.com", f.Host)
			assert.Equal(t, "myorg/myproject", f.Project)
			assert.Equal(t, "myrepo", f.Repository)
		})
	}
}

// Without SYSTEM_TEAMPROJECT the project stays empty rather than becoming "myorg/".
func TestResolve_AzureCIWithoutTeamProject(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{
		"TF_BUILD":             "true",
		"SYSTEM_COLLECTIONURI": "https://dev.azure.com/myorg/",
	}), "")
	require.NoError(t, err)
	assert.Equal(t, "https://dev.azure.com", got.Forges[0].Host)
	assert.Empty(t, got.Forges[0].Project)
}

// A forge whose base_url names another instance than the CI job's takes nothing from CI: not its API
// URL, not its project, and never its token, which belongs to the CI's own instance.
func TestResolve_ExplicitForgeOnAnotherInstanceTakesNothingFromCI(t *testing.T) {
	gitlabCOM := map[string]string{
		"GITLAB_CI": "true", "CI_SERVER_URL": "https://gitlab.com", "CI_API_V4_URL": "https://gitlab.com/api/v4",
		"CI_PROJECT_PATH": "acme/app", "CI_JOB_TOKEN": "job-token", "SELF_PAT": "self-hosted-pat",
	}
	githubCOM := map[string]string{
		"GITHUB_ACTIONS": "true", "GITHUB_SERVER_URL": "https://github.com", "GITHUB_API_URL": "https://api.github.com",
		"GITHUB_REPOSITORY": "acme/app", "GITHUB_TOKEN": "github-com-token", "GH_ENTERPRISE_TOKEN": "ghes-token",
	}
	tests := []struct {
		name        string
		forge       config.Forge
		env         map[string]string
		wantHost    string
		wantAPIURL  string
		wantProject string
		wantToken   string
	}{
		{name: "self-hosted GitLab from gitlab.com CI",
			forge: config.Forge{Name: "self", Type: "gitlab", BaseURL: "https://gitlab.example.com", Project: "acme/mirror", TokenEnv: "SELF_PAT"},
			env:   gitlabCOM, wantHost: "https://gitlab.example.com", wantProject: "acme/mirror", wantToken: "self-hosted-pat"},
		{name: "self-hosted GitLab without token_env",
			forge: config.Forge{Name: "self", Type: "gitlab", BaseURL: "https://gitlab.example.com", Project: "acme/mirror"},
			env:   gitlabCOM, wantHost: "https://gitlab.example.com", wantProject: "acme/mirror", wantToken: ""},
		{name: "GHES from github.com Actions",
			forge: config.Forge{Name: "ghes", Type: "github", BaseURL: "https://ghe.example.com", Repository: "acme/mirror"},
			env:   githubCOM, wantHost: "https://ghe.example.com", wantProject: "acme/mirror", wantToken: "ghes-token"},
		{name: "same instance, other case and a trailing slash",
			forge: config.Forge{Name: "com", Type: "gitlab", BaseURL: "https://GitLab.com/"},
			env:   gitlabCOM, wantHost: "https://GitLab.com/", wantAPIURL: "https://gitlab.com/api/v4", wantProject: "acme/app", wantToken: "job-token"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := forge.Resolve(&config.Config{Forges: []config.Forge{tc.forge}}, env(tc.env), "")
			require.NoError(t, err)

			f := got.Forges[0]
			assert.Equal(t, tc.wantHost, f.Host)
			assert.Equal(t, tc.wantProject, f.Project)
			assert.Equal(t, tc.wantToken, f.Token)
			if tc.wantAPIURL != "" || tc.forge.Type == "gitlab" {
				assert.Equal(t, tc.wantAPIURL, f.APIURL)
			}
		})
	}
}

// The git origin fills a forge only when it is on the forge's instance too.
func TestResolve_ExplicitForgeOnAnotherInstanceTakesNothingFromTheOrigin(t *testing.T) {
	cfg := &config.Config{Forges: []config.Forge{{Name: "ghes", Type: "github", BaseURL: "https://ghe.example.com"}}}

	got, err := forge.Resolve(cfg, env(nil), "git@github.com:acme/app.git")

	require.NoError(t, err)
	assert.Empty(t, got.Forges[0].Project)
}
