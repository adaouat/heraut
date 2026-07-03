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

// collectContributors returns the release's distinct contributors (first-seen order, deduped by
// git author email). IsFirstTime is true when the email is absent from before. When a PR is known
// for the author's first contributing commit, its handle/number/url are overlaid. Only first-time
// contributors are returned — the "New Contributors" block renders exactly this list.
func collectContributors(commits []parsedCommit, before map[string]bool, prs map[string]PullRequest) []Contributor {
	seen := make(map[string]bool)
	var out []Contributor
	for _, c := range commits {
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
		if pr, ok := prs[c.raw.Hash]; ok {
			contrib.Author.Username = pr.AuthorLogin
			prCopy := pr
			contrib.PR = &prCopy
		}
		out = append(out, contrib)
	}
	return out
}
