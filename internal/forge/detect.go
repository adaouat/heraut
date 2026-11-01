package forge

import (
	neturl "net/url"
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

// parseGitOrigin extracts a forge type, host, project path and (Azure DevOps only) repository name
// from a git origin URL, handling both the SSH (user@host:path.git) and HTTPS
// (https://[user@]host/path.git) forms. ok is false when the host isn't one of the known public
// hosts (github.com, gitlab.com, Azure DevOps), or when an Azure path has no repository in it.
func parseGitOrigin(url string) (typ, host, project, repository string, ok bool) {
	url = strings.TrimSuffix(strings.TrimSpace(url), ".git")

	var rawHost, path string
	switch {
	case strings.HasPrefix(url, "https://"), strings.HasPrefix(url, "http://"):
		rest := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
		h, p, found := strings.Cut(rest, "/")
		if !found {
			return "", "", "", "", false
		}
		rawHost, path = h, p
	case !strings.Contains(url, "://"):
		h, p, found := strings.Cut(url, ":")
		if !found || !strings.Contains(h, "@") {
			return "", "", "", "", false
		}
		rawHost, path = h, p
	default:
		return "", "", "", "", false
	}
	if i := strings.LastIndex(rawHost, "@"); i >= 0 {
		rawHost = rawHost[i+1:]
	}

	switch {
	case rawHost == "github.com":
		return "github", config.DefaultGitHubBaseURL, path, "", true
	case rawHost == "gitlab.com":
		return "gitlab", config.DefaultGitLabBaseURL, path, "", true
	case rawHost == "dev.azure.com", rawHost == "ssh.dev.azure.com", rawHost == "vs-ssh.visualstudio.com":
		project, repository, ok = azurePath("", path)
	case strings.HasSuffix(rawHost, ".visualstudio.com"):
		project, repository, ok = azurePath(strings.TrimSuffix(rawHost, ".visualstudio.com"), path)
	}
	if !ok {
		return "", "", "", "", false
	}
	return "azure_devops", config.DefaultAzureDevOpsBaseURL, project, repository, true
}

// azurePath splits an Azure DevOps origin path into "organization/project" and the repository
// name. org is the organization when the host carries it (legacy org.visualstudio.com), else the
// path's first segment. The web form is [org/]project/_git/repo, or [org/]_git/repo for the
// repository named after its project; SSH is v3/org/project/repo. The legacy DefaultCollection
// segment is dropped. Segments are unescaped, as the CI variables are. The host is always
// dev.azure.com, which serves every organization, legacy ones included.
func azurePath(org, path string) (project, repository string, ok bool) {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if segs[0] == "v3" {
		if org != "" || len(segs) != 4 {
			return "", "", false
		}
		org, segs = segs[1], segs[2:]
		segs = []string{segs[0], "_git", segs[1]}
	} else if org == "" {
		org, segs = segs[0], segs[1:]
	}
	if len(segs) > 0 && segs[0] == "DefaultCollection" {
		segs = segs[1:]
	}
	switch {
	case len(segs) == 3 && segs[1] == "_git":
		project, repository = segs[0], segs[2]
	case len(segs) == 2 && segs[0] == "_git":
		project, repository = segs[1], segs[1]
	default:
		return "", "", false
	}
	for _, s := range []*string{&org, &project, &repository} {
		v, err := neturl.PathUnescape(*s)
		if err != nil || v == "" {
			return "", "", false
		}
		*s = v
	}
	return org + "/" + project, repository, true
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
	if originType, _, originProject, _, originOK := parseGitOrigin(gitOrigin); originOK {
		return originType, originProject, true
	}
	return "", "", false
}
