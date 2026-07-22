package pipeline

import (
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

const (
	githubDefaultTokenEnv      = "GITHUB_TOKEN"
	gitlabDefaultTokenEnv      = "GITLAB_TOKEN"
	azureDevOpsDefaultTokenEnv = "AZURE_DEVOPS_TOKEN"
	azureDevOpsDefaultBaseURL  = "https://dev.azure.com"
)

// remoteLinkContext builds a port.LinkContext from an explicit changelog.remote block
// (ADR-0026). Unlike a release.platforms entry, this never grants publish capability —
// it only tells git-cliff/heraut where to source PR/MR metadata and link shapes for the
// changelog. Returns nil when r is nil or its type is unrecognized.
func remoteLinkContext(r *config.Remote) *port.LinkContext {
	if r == nil {
		return nil
	}
	switch r.Type {
	case "github":
		owner, repo, _ := strings.Cut(r.Repository, "/")
		return &port.LinkContext{
			BaseURL:  remoteBaseURL(r.BaseURL, "github"),
			Owner:    owner,
			Repo:     repo,
			Platform: "github",
			Token:    os.Getenv(tokenEnvOrDefault(r.TokenEnv, githubDefaultTokenEnv)),
		}
	case "gitlab":
		owner, repo := "", r.Project
		if i := strings.LastIndex(r.Project, "/"); i >= 0 {
			owner, repo = r.Project[:i], r.Project[i+1:]
		}
		return &port.LinkContext{
			BaseURL:  remoteBaseURL(r.BaseURL, "gitlab"),
			Owner:    owner,
			Repo:     repo,
			Platform: "gitlab",
			Token:    os.Getenv(tokenEnvOrDefault(r.TokenEnv, gitlabDefaultTokenEnv)),
		}
	case "azure_devops":
		return &port.LinkContext{
			BaseURL:  remoteBaseURL(r.BaseURL, "azure_devops"),
			Owner:    r.Project,
			Repo:     r.Repository,
			Platform: "azure_devops",
			Token:    os.Getenv(tokenEnvOrDefault(r.TokenEnv, azureDevOpsDefaultTokenEnv)),
		}
	default:
		return nil
	}
}

// remoteBaseURL returns the configured base URL (trailing slash trimmed) when set, else the
// per-type default web/API host. azure_devops has no config.DefaultBaseURL entry, so its
// default is applied here.
func remoteBaseURL(configured, platformType string) string {
	if configured != "" {
		return strings.TrimRight(configured, "/")
	}
	if platformType == "azure_devops" {
		return azureDevOpsDefaultBaseURL
	}
	return config.DefaultBaseURL(platformType)
}

func tokenEnvOrDefault(configured, def string) string {
	if configured != "" {
		return configured
	}
	return def
}

// ambientLinkContext resolves the link context from the ambient CI environment — the host and
// owner/repo of the repository the pipeline is running against. This is the link-host fallback
// that used to live in the embedded Tera templates, relocated into Go so the templates stay
// branch-free (ADR-0022).
//
// BaseURL is the host only, with Owner/Repo split from the CI-provided path: git-cliff's linkEnv
// composes the same {remote} from BaseURL+Owner+Repo, and — crucially — native's remote enrichment
// needs Owner/Repo to address the GitHub/GitLab API. Leaving them empty made native's ambient
// enrichment query `owner:""` and 404 (silently degrading with no attribution). The GitLab
// CI_PROJECT_URL branch is a links-only fallback for when CI_SERVER_URL / CI_PROJECT_PATH are
// absent — enrichment stays degraded there, but links still resolve. Returns nil when no CI host
// is present, in which case the caller falls through to a configured platform's context or to nil.
func ambientLinkContext() *port.LinkContext {
	// GitLab CI: split CI_SERVER_URL + CI_PROJECT_PATH so native enrichment can address the project.
	if server := os.Getenv("CI_SERVER_URL"); server != "" {
		if path := os.Getenv("CI_PROJECT_PATH"); path != "" {
			owner, repo := splitProjectPath(path)
			return &port.LinkContext{BaseURL: strings.TrimRight(server, "/"), Owner: owner, Repo: repo, Platform: "gitlab"}
		}
	}
	if u := os.Getenv("CI_PROJECT_URL"); u != "" {
		return &port.LinkContext{BaseURL: u, Platform: "gitlab"} // links-only fallback
	}
	if server := os.Getenv("GITHUB_SERVER_URL"); server != "" {
		if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
			owner, name, _ := strings.Cut(repo, "/")
			return &port.LinkContext{BaseURL: strings.TrimRight(server, "/"), Owner: owner, Repo: name, Platform: "github"}
		}
	}
	return nil
}

// splitProjectPath splits a GitLab project path ("group[/subgroup]/project") into the owner
// (everything up to the last segment) and the project name (the last segment).
func splitProjectPath(path string) (owner, project string) {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i], path[i+1:]
	}
	return "", path
}

// changelogLinkContext resolves the link context for the committed changelog. An
// explicit changelog.remote block (ADR-0026) takes priority — it is a deliberate user
// override. Otherwise, like ambientLinkContext, it prefers the CI-provided host (origin).
// When no ambient host is available (local/non-CI runs) and there is exactly one
// configured platform, that platform's context is used as a fallback so commit links are
// rendered instead of degrading to bare hashes. With multiple platforms the origin is
// ambiguous, so nil is returned (bare hashes are safer than the wrong host). See
// ADR-0022.
func (p *Pipeline) changelogLinkContext() *port.LinkContext {
	if lc := remoteLinkContext(p.cfg.ChangelogRemote); lc != nil {
		return lc
	}
	if amb := ambientLinkContext(); amb != nil {
		return amb
	}
	if len(p.cfg.Platforms) == 1 {
		lc := p.cfg.Platforms[0].LinkContext()
		return &lc
	}
	return nil
}

// platformLinkContext resolves the link context for one platform: the ambient CI host when
// it describes the *same* platform type (so a self-hosted instance whose real host only
// lives in the CI env is honoured), otherwise the platform's own base_url-derived context.
// The platform-match guard prevents a mismatched CI (e.g. a GitHub release built in
// GitLab CI) from stamping the wrong host. Used for both single and multi-platform release
// notes so the ambient-preference logic is consistent across both paths (ADR-0022).
func (p *Pipeline) platformLinkContext(plat port.Platform) *port.LinkContext {
	lc := plat.LinkContext()
	if amb := ambientLinkContext(); amb != nil && amb.Platform == lc.Platform {
		return amb
	}
	return &lc
}

// singlePlatformLinkContext resolves the link context for a single-platform release.
// It falls back to the ambient CI context alone when no platform is configured.
func (p *Pipeline) singlePlatformLinkContext() *port.LinkContext {
	if len(p.cfg.Platforms) == 0 {
		return ambientLinkContext()
	}
	return p.platformLinkContext(p.cfg.Platforms[0])
}
