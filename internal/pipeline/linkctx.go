package pipeline

import (
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

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

// linkContextFromIdentity converts a resolved forge identity into the link context used to render
// commit/MR links, so links resolve from the same source as enrichment (ADR-0043).
//
// A partial identity must never outrank the ambient/platform fallbacks below it: an empty
// Project (e.g. the token-only auto-detection branch in forge.Resolve, which cannot infer a
// project from a bare token env var) would otherwise stamp a host with no owner/repo, breaking
// both links and enrichment (a self-hosted GitLab/GHES 404s on `…/projects//…`). azure_devops
// addresses a repo as organization/project + a separate repository name (Project stays unsplit as
// Owner, matching what internal/forge/azure expects), so an azure identity with an empty
// Repository is likewise treated as partial and falls through to nil.
func linkContextFromIdentity(id port.ForgeIdentity) *port.LinkContext {
	if id.Type == "" || id.Host == "" || id.Project == "" {
		return nil
	}
	if id.Type == "azure_devops" {
		if id.Repository == "" {
			return nil
		}
		return &port.LinkContext{
			BaseURL:  strings.TrimRight(id.Host, "/"),
			Owner:    id.Project,
			Repo:     id.Repository,
			Platform: id.Type,
			Token:    id.Token,
		}
	}
	owner, repo := splitProjectPath(id.Project)
	return &port.LinkContext{
		BaseURL:  strings.TrimRight(id.Host, "/"),
		Owner:    owner,
		Repo:     repo,
		Platform: id.Type,
		Token:    id.Token,
	}
}

// changelogLinkContext resolves the link context for the committed changelog. The resolved
// forge identity (ADR-0043) takes priority — it is the single source of truth for enrichment
// and links. Next is, like ambientLinkContext, the CI-provided host (origin). When no ambient
// host is available (local/non-CI runs) and there is exactly one configured platform, that
// platform's context is used as a fallback so commit links are rendered instead of degrading
// to bare hashes. With multiple platforms the origin is ambiguous, so nil is returned (bare
// hashes are safer than the wrong host). See ADR-0022.
func (p *Pipeline) changelogLinkContext() *port.LinkContext {
	if p.cfg.ForgeIdentity != nil {
		if lc := linkContextFromIdentity(*p.cfg.ForgeIdentity); lc != nil {
			return lc
		}
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

// changelogLinkContext resolves the link context for the standalone `heraut changelog` flow.
// It mirrors the release pipeline's precedence — the resolved forge identity (ADR-0043), then
// the ambient CI host (ADR-0022) — minus the single-platform fallback, since a changelog-only
// run configures no release platforms.
func (p *ChangelogPipeline) changelogLinkContext() *port.LinkContext {
	if p.cfg.ForgeIdentity != nil {
		if lc := linkContextFromIdentity(*p.cfg.ForgeIdentity); lc != nil {
			return lc
		}
	}
	return ambientLinkContext()
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
