package native

import (
	"fmt"
	"os"

	"github.com/adaouat/heraut/internal/port"
)

// enrichResult bundles per-commit remote data: associated PRs and commit-author handles.
type enrichResult struct {
	prs     map[string]PullRequest
	authors map[string]string
}

// enrich resolves PR/MR enrichment for commits via the platform's CLI, returning an enrichResult.
// Returns a zero enrichResult when lc is nil or the platform has no enrichment support yet. This
// is the platform-dispatch seam: GitLab and Azure DevOps slot in as additional cases.
func (g *Generator) enrich(lc *port.LinkContext, commits []rawCommit) (enrichResult, error) {
	if lc == nil {
		return enrichResult{}, nil
	}
	shas := make([]string, 0, len(commits))
	for _, c := range commits {
		shas = append(shas, c.Hash)
	}
	switch lc.Platform {
	case "github":
		prs, authors, err := enrichGitHub(g.runner, lc, shas)
		return enrichResult{prs: prs, authors: authors}, err
	case "gitlab":
		prs, err := enrichGitLab(g.runner, lc, shas)
		return enrichResult{prs: prs}, err
	case "azure_devops":
		prs, err := enrichAzure(g.httpClient, lc, shas)
		return enrichResult{prs: prs}, err
	default:
		return enrichResult{}, nil
	}
}

// enrichable reports whether lc has a platform this generator can fetch metadata from. Under
// remote_metadata: required, an lc that is not enrichable (nil, or an unsupported platform) means
// the requirement cannot be satisfied. Keep the platform set in sync with enrich()'s switch.
func enrichable(lc *port.LinkContext) bool {
	if lc == nil {
		return false
	}
	switch lc.Platform {
	case "github", "gitlab", "azure_devops":
		return true
	default:
		return false
	}
}

// enrichForRelease applies the remote_metadata policy (ADR-0023 / ADR-0034 §6) around enrich:
//   - "disabled": never fetch.
//   - "required": fetch; any failure is fatal.
//   - "optional" / "" (default): fetch; on failure, drop enrichment, mark the generator
//     degraded, and warn once. Rendering then proceeds without PR attribution.
//
// A nil / unsupported platform is not a failure — it simply yields no enrichment.
func (g *Generator) enrichForRelease(lc *port.LinkContext, commits []rawCommit) (enrichResult, error) {
	if g.cfg.RemoteMetadata == "disabled" {
		return enrichResult{}, nil
	}
	// --force downgrades required to optional: an unavailable or unconfigured remote degrades
	// instead of erroring.
	required := g.cfg.RemoteMetadata == "required" && !g.cfg.Force
	if required && !enrichable(lc) {
		return enrichResult{}, fmt.Errorf("remote enrichment (required): no changelog remote or release platform configured to fetch PR/MR metadata from")
	}
	er, err := g.enrich(lc, commits)
	if err != nil {
		if required {
			return enrichResult{}, fmt.Errorf("remote enrichment (required): %w", err)
		}
		if !g.degraded {
			fmt.Fprintf(os.Stderr, "warning: remote enrichment unavailable; rendering without PR attribution: %v\n", err)
		}
		g.degraded = true
		return enrichResult{}, nil
	}
	return er, nil
}
