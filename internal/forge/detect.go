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

// defaultTokenEnvsFor returns the variables a forge type's token is read from when no token_env
// is set, first set wins. GitHub reads the publish driver's chain for the host.
func defaultTokenEnvsFor(typ, host string) []string {
	if typ == "github" {
		return config.GitHubTokenEnvs(host)
	}
	if name := defaultTokenEnvFor(typ); name != "" {
		return []string{name}
	}
	return nil
}

// defaultTokenEnvFor returns the default token env var name for a forge type, or "" when
// unknown. Zero-config detection counts these alone: GH_TOKEN is often exported for the gh CLI,
// so it never picks GitHub on its own.
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
		host, project := azureCollection(getenv(config.EnvAzureCollectionURI), getenv(config.EnvAzureTeamProject))
		return "azure_devops", host, "", project, getenv(config.EnvAzureRepositoryName), getenv(config.EnvAzureAccessToken), port.TokenPrivate, true
	}
	return "", "", "", "", "", "", port.TokenNone, false
}

// azureCollection splits Azure Pipelines' collection URI into the shape of a forges: entry
// (ADR-0035): a host without the organization and "organization/project", so a field filled from CI
// composes with one set in config without naming the organization twice. The organization is the
// URI's last path segment (dev.azure.com/org, or an Azure DevOps Server's collection), else the
// legacy org.visualstudio.com subdomain, whose host becomes dev.azure.com as for a git origin. A URI
// with neither is kept whole, with the team project alone.
func azureCollection(collectionURI, teamProject string) (host, project string) {
	collectionURI = strings.TrimRight(collectionURI, "/")
	u, err := neturl.Parse(collectionURI)
	if err != nil || u.Host == "" {
		return collectionURI, teamProject
	}
	var org string
	switch {
	case u.Path != "":
		i := strings.LastIndex(u.Path, "/")
		org, host = u.Path[i+1:], u.Scheme+"://"+u.Host+u.Path[:i]
	case strings.HasSuffix(u.Hostname(), ".visualstudio.com"):
		org, host = strings.TrimSuffix(u.Hostname(), ".visualstudio.com"), config.DefaultAzureDevOpsBaseURL
	default:
		return collectionURI, teamProject
	}
	if teamProject == "" {
		return host, ""
	}
	return host, org + "/" + teamProject
}

// parseGitOrigin extracts a forge type, host, project path and (Azure DevOps only) repository name
// from a git origin URL: scp-like SSH (user@host:path.git), or an https, http or ssh URL with or
// without credentials and a port. The host is compared without case; GitHub's and GitLab's SSH-over-443
// hosts count as theirs. ok is false when the host isn't one of the known public hosts (github.com,
// gitlab.com, Azure DevOps), or when the path is empty or, for Azure, has no repository in it.
func parseGitOrigin(origin string) (typ, host, project, repository string, ok bool) {
	rawHost, path, found := splitOrigin(strings.TrimSpace(origin))
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if !found || path == "" {
		return "", "", "", "", false
	}

	switch rawHost = strings.ToLower(rawHost); {
	case rawHost == "github.com", rawHost == "ssh.github.com":
		return "github", config.DefaultGitHubBaseURL, path, "", true
	case rawHost == "gitlab.com", rawHost == "altssh.gitlab.com":
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

// splitOrigin returns an origin's host, without user or port, and its path, still escaped.
func splitOrigin(origin string) (host, path string, ok bool) {
	if strings.Contains(origin, "://") {
		u, err := neturl.Parse(origin)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "ssh") || u.Hostname() == "" {
			return "", "", false
		}
		return u.Hostname(), u.EscapedPath(), true
	}
	h, p, found := strings.Cut(origin, ":")
	if !found || !strings.Contains(h, "@") {
		return "", "", false
	}
	return h[strings.LastIndex(h, "@")+1:], p, true
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
