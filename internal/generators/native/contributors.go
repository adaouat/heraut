package native

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// authorsBefore returns the set of git author emails reachable from prev (git log prev
// --format=%aE, through .mailmap, as the commit walk reads them). An empty prev (first release) yields an empty set with no git call, so every
// release author counts as new. This is the local tier's first_time source (ADR-0036).
func authorsBefore(runner port.Runner, prev string) (map[string]bool, error) {
	set := make(map[string]bool)
	if prev == "" {
		return set, nil
	}
	stdout, _, err := runner.Run("git", "log", prev, "--format=%aE")
	if err != nil {
		return nil, fmt.Errorf("listing authors before %s: %w", prev, err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(stdout), "\n") {
		if e := strings.TrimSpace(line); e != "" {
			set[e] = true
		}
	}
	return set, nil
}

// toParsedCommits wraps raw commits for collectContributors, which only reads c.raw — contributor
// identity and first-time status are independent of conventional-commit classification, so no
// parsing (or the group/excludes filtering that goes with it) is needed here.
func toParsedCommits(commits []rawCommit) []parsedCommit {
	out := make([]parsedCommit, len(commits))
	for i, rc := range commits {
		out[i] = parsedCommit{raw: rc}
	}
	return out
}

// renderedCommits returns the subset of commits (original order) that survive classification —
// those present in groups after rendering.excludes filtering. Contributors are computed from these
// so an author whose only commits are excluded (e.g. a bot's chore(deps)) is not celebrated in the
// "New Contributors" block (T149) — "new contributors" are the authors whose work is shown.
func renderedCommits(commits []rawCommit, groups []group) []rawCommit {
	shown := make(map[string]bool)
	for _, g := range groups {
		for _, pc := range g.commits {
			shown[pc.raw.Hash] = true
		}
	}
	out := make([]rawCommit, 0, len(commits))
	for _, c := range commits {
		if shown[c.Hash] {
			out = append(out, c)
		}
	}
	return out
}

// collectContributors returns the release's distinct contributors (first-seen order, deduped by
// git author email). IsFirstTime is true when the email is absent from before. The handle is
// overlaid from the author's **first PR-bearing commit** in the release — their earliest commit
// may be unlinked while a later one carries the PR, and the built-in template only renders a
// contributor once a handle is known. PRs collects every distinct PR (deduped by number, since a
// rebase-merged PR can attach the same number to several commits) referenced by the contributor's
// commits, not just the first (T262) — someone who opens more than one PR in their first release
// is credited for all of them. Only first-time contributors are returned — the "New Contributors"
// block renders exactly this list. Entries are then merged by forge handle (mergeByHandle).
func collectContributors(commits []parsedCommit, before map[string]bool, prs map[string]PullRequest) []Contributor {
	seen := make(map[string]bool)
	returning := make(map[string]bool) // handles of this release's returning authors
	var out []Contributor
	for i, c := range commits {
		email := c.raw.Email
		if before[email] {
			if pr, ok := prs[c.raw.Hash]; ok && pr.AuthorLogin != "" {
				returning[pr.AuthorLogin] = true
			}
		}
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		if before[email] {
			continue // returning contributor
		}
		contrib := Contributor{
			Author:      Author{Name: c.raw.Author, Email: email},
			IsFirstTime: true,
		}
		prSeen := make(map[int]bool)
		for _, c2 := range commits[i:] {
			if c2.raw.Email != email {
				continue
			}
			pr, ok := prs[c2.raw.Hash]
			if !ok || prSeen[pr.Number] {
				continue
			}
			prSeen[pr.Number] = true
			if contrib.Author.Username == "" {
				contrib.Author.Username = pr.AuthorLogin
			}
			contrib.PRs = append(contrib.PRs, pr)
		}
		out = append(out, contrib)
	}
	return mergeByHandle(out, returning)
}

// mergeByHandle treats a forge handle as one person across emails: entries sharing a handle fold
// into the first, crediting each PR once, and an entry whose handle a returning author of this
// release also has is dropped, since that person isn't new. Entries without a handle (no
// enrichment) stay per email.
func mergeByHandle(entries []Contributor, returning map[string]bool) []Contributor {
	var out []Contributor
	at := make(map[string]int) // handle → index in out
	for _, e := range entries {
		handle := e.Author.Username
		if handle == "" {
			out = append(out, e)
			continue
		}
		if returning[handle] {
			continue
		}
		i, dup := at[handle]
		if !dup {
			at[handle] = len(out)
			out = append(out, e)
			continue
		}
		for _, pr := range e.PRs {
			if !slices.ContainsFunc(out[i].PRs, func(p PullRequest) bool { return p.Number == pr.Number }) {
				out[i].PRs = append(out[i].PRs, pr)
			}
		}
	}
	return out
}
