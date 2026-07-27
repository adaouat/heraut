package native

import (
	"fmt"

	"github.com/adaouat/heraut/internal/port"
)

// enrichResult bundles per-commit remote data: associated PRs and commit-author handles.
type enrichResult struct {
	prs     map[string]PullRequest
	authors map[string]string
}

// enrich resolves PR/MR enrichment for commits via the injected port.Forge (ADR-0043), returning
// an enrichResult. Returns a zero enrichResult when no forge is configured.
func (g *Generator) enrich(lc *port.LinkContext, commits []rawCommit) (enrichResult, error) {
	if g.forge == nil {
		return enrichResult{}, nil
	}
	pc := make([]port.Commit, 0, len(commits))
	for _, c := range commits {
		pc = append(pc, port.Commit{Hash: c.Hash, Author: c.Author, Email: c.Email, Date: c.Date})
	}
	en, err := g.forge.Enrich(pc)
	if err != nil {
		return enrichResult{}, err
	}
	return enrichResult{prs: fromPortPRs(en.PRs), authors: en.Authors}, nil
}

// enrichForRelease applies the remote_metadata policy (ADR-0023 / ADR-0034 §6) around enrich:
//   - "disabled": never fetch.
//   - "required": fetch; any failure is fatal.
//   - "optional" / "" (default): fetch; on failure, drop enrichment, mark the generator
//     degraded, and warn once. Rendering then proceeds without PR attribution.
//
// No forge configured is not itself a failure — it simply yields no enrichment — except under
// "required", where an unconfigured forge cannot satisfy the policy and is a hard error.
func (g *Generator) enrichForRelease(lc *port.LinkContext, commits []rawCommit) (enrichResult, error) {
	if g.cfg.RemoteMetadata == "disabled" {
		return enrichResult{}, nil
	}
	// --force downgrades required to optional: an unavailable or unconfigured remote degrades
	// instead of erroring.
	required := g.cfg.RemoteMetadata == "required" && !g.cfg.Force
	if required && g.forge == nil {
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

// fromPortPRs converts port.PullRequest values into the native render model. Platforms stays nil
// (the port model carries no per-platform bag) and Author.Name/Email stay empty — native's
// contributors tier fills those from local git, not from remote enrichment.
func fromPortPRs(in map[string]port.PullRequest) map[string]PullRequest {
	out := make(map[string]PullRequest, len(in))
	for sha, p := range in {
		approvers := make([]Author, 0, len(p.Approvers))
		for _, a := range p.Approvers {
			approvers = append(approvers, Author{Username: a.Username})
		}
		if len(approvers) == 0 {
			approvers = nil
		}
		out[sha] = PullRequest{
			Number: p.Number, URL: p.URL, AuthorLogin: p.AuthorLogin, RefPrefix: p.RefPrefix,
			Title: p.Title, Labels: p.Labels, CreatedAt: p.CreatedAt, MergedAt: p.MergedAt,
			MergedBy:  Author{Username: p.MergedBy.Username},
			Approvers: approvers,
		}
	}
	return out
}
