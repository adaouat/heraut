package pipeline

import (
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// ambientLinkContext resolves the link context from the ambient CI environment — the host
// of the repository the pipeline is running against. This is the link-host fallback that
// used to live in the embedded Tera templates, relocated into Go so the templates stay
// branch-free (T75 / ADR-0022).
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

// singlePlatformLinkContext resolves the link context for a single-platform release: the
// ambient CI host when it describes the *same* platform (so a self-hosted instance — whose
// real host only lives in the CI env — is honoured), otherwise the platform's own
// base_url-derived context. The platform-match guard prevents a mismatched CI (e.g. a
// GitHub release built in GitLab CI) from stamping the wrong host (T75 / ADR-0022).
func (p *Pipeline) singlePlatformLinkContext() *port.LinkContext {
	amb := ambientLinkContext()
	if len(p.cfg.Platforms) == 0 {
		return amb
	}
	lc := p.cfg.Platforms[0].LinkContext()
	if amb != nil && amb.Platform == lc.Platform {
		return amb
	}
	return &lc
}
