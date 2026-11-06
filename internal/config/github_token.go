package config

import (
	"net/url"
	"strings"
)

// GitHubTokenEnvs lists the variables a GitHub token is read from when no token_env is set, first
// set wins: gh's chain for the host. github.com and *.ghe.com tenants read GH_TOKEN, then
// GITHUB_TOKEN; a GitHub Enterprise Server reads its enterprise names, then GH_TOKEN, as gh does.
// Publishing and pull-request enrichment both use it.
func GitHubTokenEnvs(baseURL string) []string {
	if isGitHubEnterpriseServer(baseURL) {
		return []string{EnvGitHubEnterpriseCLIToken, EnvGitHubEnterpriseToken, EnvGitHubCLIToken}
	}
	return []string{EnvGitHubCLIToken, EnvGitHubToken}
}

func isGitHubEnterpriseServer(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return true
	}
	host := strings.ToLower(u.Hostname())
	return host != "github.com" && !strings.HasSuffix(host, ".ghe.com")
}
