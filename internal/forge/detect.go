package forge

import (
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// Default web hosts and token env var names per forge type. The github/gitlab hosts mirror
// config.go's DefaultBaseURL; the azure host and all three token-env names mirror
// internal/pipeline/linkctx.go. They are copied (not imported) because internal/forge sits
// below internal/pipeline in the layer stack and must not import it.
const (
	githubDefaultHost      = "https://github.com"
	gitlabDefaultHost      = "https://gitlab.com"
	azureDevOpsDefaultHost = "https://dev.azure.com"

	githubDefaultTokenEnv      = "GITHUB_TOKEN"
	gitlabDefaultTokenEnv      = "GITLAB_TOKEN"
	azureDevOpsDefaultTokenEnv = "AZURE_DEVOPS_TOKEN"
)

// defaultHostFor returns the default web host for a forge type, or "" when unknown.
func defaultHostFor(typ string) string {
	switch typ {
	case "github":
		return githubDefaultHost
	case "gitlab":
		return gitlabDefaultHost
	case "azure_devops":
		return azureDevOpsDefaultHost
	default:
		return ""
	}
}

// defaultTokenEnvFor returns the default token env var name for a forge type, or "" when
// unknown.
func defaultTokenEnvFor(typ string) string {
	switch typ {
	case "github":
		return githubDefaultTokenEnv
	case "gitlab":
		return gitlabDefaultTokenEnv
	case "azure_devops":
		return azureDevOpsDefaultTokenEnv
	default:
		return ""
	}
}

// detectCIForge inspects the ambient CI environment (via getenv) for one of the three known CI
// systems' markers, in the order GitLab → GitHub → Azure. ok is false when none is present. See
// design spec §3 for the source table.
func detectCIForge(getenv func(string) string) (typ, host, apiURL, project, repository, token string, kind port.TokenKind, ok bool) {
	if getenv("GITLAB_CI") != "" {
		return "gitlab", getenv("CI_SERVER_URL"), getenv("CI_API_V4_URL"), getenv("CI_PROJECT_PATH"), "", getenv("CI_JOB_TOKEN"), port.TokenJob, true
	}
	if getenv("GITHUB_ACTIONS") != "" {
		return "github", getenv("GITHUB_SERVER_URL"), getenv("GITHUB_API_URL"), getenv("GITHUB_REPOSITORY"), "", getenv("GITHUB_TOKEN"), port.TokenPrivate, true
	}
	if getenv("TF_BUILD") != "" {
		org := azureOrgFromCollectionURI(getenv("SYSTEM_COLLECTIONURI"))
		teamProject := getenv("SYSTEM_TEAMPROJECT")
		project := teamProject
		if org != "" && teamProject != "" {
			project = org + "/" + teamProject
		}
		return "azure_devops", getenv("SYSTEM_COLLECTIONURI"), "", project, getenv("BUILD_REPOSITORY_NAME"), getenv("SYSTEM_ACCESSTOKEN"), port.TokenPrivate, true
	}
	return "", "", "", "", "", "", port.TokenNone, false
}

// azureOrgFromCollectionURI extracts the organization name from an Azure Pipelines
// SYSTEM_COLLECTIONURI, e.g. "https://dev.azure.com/myorg/" → "myorg". Returns "" when the URI
// has no path segment (malformed or unset).
func azureOrgFromCollectionURI(uri string) string {
	trimmed := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(uri, "https://"), "http://"), "/")
	_, path, found := strings.Cut(trimmed, "/")
	if !found {
		return ""
	}
	org, _, _ := strings.Cut(path, "/")
	return org
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
		return "github", "https://github.com", path, true
	case "gitlab.com":
		return "gitlab", "https://gitlab.com", path, true
	case "dev.azure.com":
		return "azure_devops", "https://dev.azure.com", path, true
	default:
		return "", "", "", false
	}
}
