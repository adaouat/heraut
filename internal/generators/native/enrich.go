package native

import (
	"fmt"
	"os"

	"github.com/adaouat/heraut/internal/port"
)

// enrich resolves PR/MR enrichment for commits via the platform's CLI, returning a SHA→PullRequest
// map. Returns nil when lc is nil or the platform has no enrichment support yet. This is the
// platform-dispatch seam: GitLab and Azure DevOps slot in as additional cases.
func (g *Generator) enrich(lc *port.LinkContext, commits []rawCommit) (map[string]PullRequest, error) {
	if lc == nil {
		return nil, nil
	}
	shas := make([]string, 0, len(commits))
	for _, c := range commits {
		shas = append(shas, c.Hash)
	}
	switch lc.Platform {
	case "github":
		return enrichGitHub(g.runner, lc, shas)
	case "gitlab":
		return enrichGitLab(g.runner, lc, shas)
	case "azure_devops":
		return enrichAzure(g.httpClient, lc, shas)
	default:
		return nil, nil
	}
}

// enrichForRelease applies the remote_metadata policy (ADR-0023 / ADR-0034 §6) around enrich:
//   - "disabled": never fetch.
//   - "required": fetch; any failure is fatal.
//   - "optional" / "" (default): fetch; on failure, drop enrichment, mark the generator
//     degraded, and warn once. Rendering then proceeds without PR attribution.
//
// A nil / unsupported platform is not a failure — it simply yields no enrichment.
func (g *Generator) enrichForRelease(lc *port.LinkContext, commits []rawCommit) (map[string]PullRequest, error) {
	if g.cfg.RemoteMetadata == "disabled" {
		return nil, nil
	}
	enrichment, err := g.enrich(lc, commits)
	if err != nil {
		if g.cfg.RemoteMetadata == "required" {
			return nil, fmt.Errorf("remote enrichment (required): %w", err)
		}
		if !g.degraded {
			fmt.Fprintf(os.Stderr, "warning: remote enrichment unavailable; rendering without PR attribution: %v\n", err)
		}
		g.degraded = true
		return nil, nil
	}
	return enrichment, nil
}
