package forge

import (
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// defaultHostFor returns the default web host for a forge type, or "" when unknown.
func defaultHostFor(typ string) string {
	switch typ {
	case "github":
		return config.DefaultGitHubBaseURL
	case "gitlab":
		return config.DefaultGitLabBaseURL
	case "azure_devops":
		return config.DefaultAzureDevOpsBaseURL
	default:
		return ""
	}
}

// defaultTokenEnvFor returns the default token env var name for a forge type, or "" when
// unknown.
func defaultTokenEnvFor(typ string) string {
	switch typ {
	case "github":
		return config.EnvGitHubToken
	case "gitlab":
		return config.EnvGitLabToken
	case "azure_devops":
		return config.EnvAzureDevOpsToken
	default:
		return ""
	}
}

// detectCIForge inspects the ambient CI environment (via getenv) for one of the three known CI
// systems' markers, in the order GitLab → GitHub → Azure. ok is false when none is present. See
// design spec §3 for the source table.
func detectCIForge(getenv func(string) string) (typ, host, apiURL, project, repository, token string, kind port.TokenKind, ok bool) {
	if getenv(config.EnvGitLabCI) != "" {
		return "gitlab", getenv(config.EnvGitLabServerURL), getenv(config.EnvGitLabAPIURL), getenv(config.EnvGitLabProjectPath), "", getenv(config.EnvGitLabJobToken), port.TokenJob, true
	}
	if getenv(config.EnvGitHubActions) != "" {
		return "github", getenv(config.EnvGitHubServerURL), getenv(config.EnvGitHubAPIURL), getenv(config.EnvGitHubRepository), "", getenv(config.EnvGitHubToken), port.TokenPrivate, true
	}
	if getenv(config.EnvAzureBuild) != "" {
		host := strings.TrimRight(getenv(config.EnvAzureCollectionURI), "/")
		teamProject := getenv(config.EnvAzureTeamProject)
		return "azure_devops", host, "", teamProject, getenv(config.EnvAzureRepositoryName), getenv(config.EnvAzureAccessToken), port.TokenPrivate, true
	}
	return "", "", "", "", "", "", port.TokenNone, false
}

// parseGitOrigin extracts a forge type, host, and project path from a git origin URL, handling
// both the SSH (git@host:path.git) and HTTPS (https://host/path.git) forms. ok is false when the
// host isn't one of the known public hosts (github.com, gitlab.com, dev.azure.com).
func parseGitOrigin(url string) (typ, host, project string, ok bool) {
	url = strings.TrimSuffix(strings.TrimSpace(url), ".git")

	var rawHost, path string
	switch {
	case strings.HasPrefix(url, "git@"):
		rest := strings.TrimPrefix(url, "git@")
		h, p, found := strings.Cut(rest, ":")
		if !found {
			return "", "", "", false
		}
		rawHost, path = h, p
	case strings.HasPrefix(url, "https://"), strings.HasPrefix(url, "http://"):
		rest := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
		h, p, found := strings.Cut(rest, "/")
		if !found {
			return "", "", "", false
		}
		rawHost, path = h, p
	default:
		return "", "", "", false
	}

	switch rawHost {
	case "github.com":
		return "github", config.DefaultGitHubBaseURL, path, true
	case "gitlab.com":
		return "gitlab", config.DefaultGitLabBaseURL, path, true
	case "dev.azure.com":
		return "azure_devops", config.DefaultAzureDevOpsBaseURL, path, true
	default:
		return "", "", "", false
	}
}

// DetectForWizard identifies a forge's platform type and project/repository path for wizard
// pre-fill: CI environment markers take priority, then a git origin URL matching one of the
// three known public hosts. ok is false when neither source identifies a recognized type —
// unlike Resolve's zero-config path, this never falls back to inspecting ambient token env vars,
// since the wizard always asks the user to pick (or confirm) a type explicitly when detection is
// inconclusive rather than guessing from which token happens to be set.
func DetectForWizard(getenv func(string) string, gitOrigin string) (typ, projectOrRepo string, ok bool) {
	if ciType, _, _, ciProject, ciRepository, _, _, ciOK := detectCIForge(getenv); ciOK {
		project := ciProject
		if project == "" {
			project = ciRepository
		}
		return ciType, project, true
	}
	if originType, _, originProject, originOK := parseGitOrigin(gitOrigin); originOK {
		return originType, originProject, true
	}
	return "", "", false
}
