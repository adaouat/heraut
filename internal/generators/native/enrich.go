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
// an enrichResult. Returns a zero enrichResult when no forge is configured. ref is the
// git-resolvable tip of the release window — a tag name, or the literal "HEAD" for the unreleased
// section — passed on to the forge as the true range anchor (T153).
func (g *Generator) enrich(commits []rawCommit, ref string) (enrichResult, error) {
	if g.forge == nil {
		return enrichResult{}, nil
	}
	resolvedRef, err := g.resolveEnrichRef(ref)
	if err != nil {
		return enrichResult{}, fmt.Errorf("resolving enrichment ref: %w", err)
	}
	pc := make([]port.Commit, 0, len(commits))
	for _, c := range commits {
		pc = append(pc, port.Commit{Hash: c.Hash, Author: c.Author, Email: c.Email, Date: c.Date})
	}
	en, err := g.forge.Enrich(pc, resolvedRef)
	if err != nil {
		return enrichResult{}, err
	}
	return enrichResult{prs: fromPortPRs(en.PRs), authors: en.Authors}, nil
}

// resolveEnrichRef resolves ref to a value a remote forge API can look up. "HEAD" is a local git
// shorthand no forge understands, so it is resolved to the actual commit SHA via `git rev-parse
// HEAD`; any other value (a tag name) is already forge-resolvable and returned unchanged (T153).
func (g *Generator) resolveEnrichRef(ref string) (string, error) {
	if ref != "HEAD" {
		return ref, nil
	}
	return headSHA(g.runner)
}

// enrichForRelease applies the remote_metadata policy (ADR-0023 / ADR-0034 §6) around enrich:
//   - "disabled": never fetch.
//   - "required": fetch; any failure is fatal.
//   - "optional" / "" (default): fetch; on failure, drop enrichment, mark the generator
//     degraded, and warn once. Rendering then proceeds without PR attribution.
//
// No forge configured is not itself a failure — it simply yields no enrichment — except under
// "required", where an unconfigured forge cannot satisfy the policy and is a hard error. ref is
// the release window's git-resolvable tip, forwarded to enrich (T153).
func (g *Generator) enrichForRelease(commits []rawCommit, ref string) (enrichResult, error) {
	if g.cfg.RemoteMetadata == "disabled" {
		return enrichResult{}, nil
	}
	// --force downgrades required to optional: an unavailable or unconfigured remote degrades
	// instead of erroring.
	required := g.cfg.RemoteMetadata == "required" && !g.cfg.Force
	if required && g.forge == nil {
		return enrichResult{}, fmt.Errorf("remote enrichment (required): no forge resolved to fetch PR/MR metadata from — configure a forges: entry, run in a supported CI environment, or use a recognised git origin")
	}
	er, err := g.enrich(commits, ref)
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
