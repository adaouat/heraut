package native

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// authorsBefore returns the set of git author emails reachable from prev (git log prev
// --format=%ae). An empty prev (first release) yields an empty set with no git call, so every
// release author counts as new. This is the local tier's first_time source (ADR-0036).
func authorsBefore(runner port.Runner, prev string) (map[string]bool, error) {
	set := make(map[string]bool)
	if prev == "" {
		return set, nil
	}
	stdout, _, err := runner.Run("git", "log", prev, "--format=%ae")
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

// collectContributors returns the release's distinct contributors (first-seen order, deduped by
// git author email). IsFirstTime is true when the email is absent from before. The PR handle /
// number / url are overlaid from the author's **first PR-bearing commit** in the release — their
// earliest commit may be unlinked while a later one carries the PR, and the built-in template
// only renders a contributor once a handle is known. Only first-time contributors are returned —
// the "New Contributors" block renders exactly this list.
func collectContributors(commits []parsedCommit, before map[string]bool, prs map[string]PullRequest) []Contributor {
	seen := make(map[string]bool)
	var out []Contributor
	for i, c := range commits {
		email := c.raw.Email
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
		for _, c2 := range commits[i:] {
			if c2.raw.Email != email {
				continue
			}
			if pr, ok := prs[c2.raw.Hash]; ok {
				contrib.Author.Username = pr.AuthorLogin
				prCopy := pr
				contrib.PR = &prCopy
				break
			}
		}
		out = append(out, contrib)
	}
	return out
}
