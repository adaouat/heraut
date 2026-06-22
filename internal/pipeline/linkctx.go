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
			BaseURL:  config.DefaultBaseURL("github"),
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
			BaseURL:  config.DefaultBaseURL("gitlab"),
			Owner:    owner,
			Repo:     repo,
			Platform: "gitlab",
			Token:    os.Getenv(tokenEnvOrDefault(r.TokenEnv, gitlabDefaultTokenEnv)),
		}
	case "azure_devops":
		baseURL := r.APIURL
		if baseURL == "" {
			baseURL = azureDevOpsDefaultBaseURL
		}
		return &port.LinkContext{
			BaseURL:  baseURL,
			Owner:    r.Project,
			Repo:     r.Repository,
			Platform: "azure_devops",
			Token:    os.Getenv(tokenEnvOrDefault(r.TokenEnv, azureDevOpsDefaultTokenEnv)),
		}
	default:
		return nil
	}
}

func tokenEnvOrDefault(configured, def string) string {
	if configured != "" {
		return configured
	}
	return def
}

// ambientLinkContext resolves the link context from the ambient CI environment — the host
// of the repository the pipeline is running against. This is the link-host fallback that
// used to live in the embedded Tera templates, relocated into Go so the templates stay
// branch-free (ADR-0022).
//
// The BaseURL holds the full repo root (Owner/Repo stay empty); gitcliff.linkEnv composes
// the same {remote} from it with no URL-splitting. Returns nil when no CI host is present,
// in which case the caller falls through to a configured platform's context or to nil.
func ambientLinkContext() *port.LinkContext {
	if u := os.Getenv("CI_PROJECT_URL"); u != "" {
		return &port.LinkContext{BaseURL: u, Platform: "gitlab"}
	}
	if server := os.Getenv("GITHUB_SERVER_URL"); server != "" {
		if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
			return &port.LinkContext{BaseURL: strings.TrimRight(server, "/") + "/" + repo, Platform: "github"}
		}
	}
	return nil
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
