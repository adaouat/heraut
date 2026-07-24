package native

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// gqlString renders s as a GraphQL string literal (surrounding quotes + escaping), so values
// interpolated into a query (project paths, refs, cursors, SHAs) cannot break out of it. A JSON
// string literal is a valid GraphQL string literal for these inputs; for quote-free values the
// output is byte-identical to the previous `"%s"` interpolation.
func gqlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

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
		prs, authors, err := enrichGitLab(g.runner, lc, shas, oldestCommitDate(commits), newestSHA(commits))
		return enrichResult{prs: prs, authors: authors}, err
	case "azure_devops":
		prs, err := enrichAzure(g.httpClient, lc, shas)
		return enrichResult{prs: prs, authors: azureCommitAuthors(commits)}, err
	default:
		return enrichResult{}, nil
	}
}

// oldestCommitDate returns the minimum committed date over commits (bounds the GitLab fetch), or
// the zero time when empty.
func oldestCommitDate(commits []rawCommit) time.Time {
	var oldest time.Time
	for _, c := range commits {
		if oldest.IsZero() || c.Date.Before(oldest) {
			oldest = c.Date
		}
	}
	return oldest
}

// newestSHA returns the hash of the newest-dated commit (the range tip; the commits(ref:) anchor),
// or "" when empty. Accepted heuristic: under rewritten committer dates (rebase/amend) the
// max-date commit may not be an ancestor of the true tip, so those commits' authors won't
// resolve — graceful (missing `by @`, never wrong data), since SHA match remains authoritative.
func newestSHA(commits []rawCommit) string {
	var newest rawCommit
	for _, c := range commits {
		if newest.Date.IsZero() || c.Date.After(newest.Date) {
			newest = c
		}
	}
	return newest.Hash
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
			g.degradedReason = fmt.Sprintf("remote enrichment unavailable; rendering without PR attribution: %v", err)
		}
		g.degraded = true
		return enrichResult{}, nil
	}
	return er, nil
}
