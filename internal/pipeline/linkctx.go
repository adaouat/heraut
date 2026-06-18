package pipeline

import (
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

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

// changelogLinkContext resolves the link context for the committed changelog. Like
// ambientLinkContext it prefers the CI-provided host (origin). When no ambient host is
// available (local/non-CI runs) and there is exactly one configured platform, that
// platform's context is used as a fallback so commit links are rendered instead of
// degrading to bare hashes. With multiple platforms the origin is ambiguous, so nil is
// returned (bare hashes are safer than the wrong host). See ADR-0022.
func (p *Pipeline) changelogLinkContext() *port.LinkContext {
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
