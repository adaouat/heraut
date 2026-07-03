package native

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"

	"github.com/adaouat/heraut/internal/port"
)

// enrichGitLab resolves the merge request associated with each commit SHA via per-commit
// `glab api projects/{id}/repository/commits/{sha}/merge_requests` calls. GitLab has no
// batched per-commit-MR primitive like GitHub's GraphQL associatedPullRequests, so this is one
// call per commit (the enrichment range is the bounded release). Commits with no MR are absent
// from the map. First-time-contributor status is then resolved per distinct author (see
// markGitLabFirstTimers) since GitLab's MR API carries no authorAssociation field.
func enrichGitLab(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]PullRequest, error) {
	project := url.PathEscape(lc.Owner + "/" + lc.Repo)
	result := make(map[string]PullRequest)
	for _, sha := range shas {
		endpoint := "projects/" + project + "/repository/commits/" + sha + "/merge_requests"
		stdout, _, err := runner.RunEnv(lc.APIEnv(), "glab", "api", endpoint)
		if err != nil {
			return nil, fmt.Errorf("glab api commit merge_requests: %w", err)
		}
		mrs, err := parseGitLabMRs(stdout)
		if err != nil {
			return nil, err
		}
		if len(mrs) == 0 {
			continue
		}
		mr := mrs[0] // first association wins (matches GitHub's associatedPullRequests first:1)
		result[sha] = PullRequest{
			Number:      mr.IID,
			URL:         mr.WebURL,
			AuthorLogin: mr.Author.Username,
			RefPrefix:   "!",
		}
	}
	if err := markGitLabFirstTimers(runner, lc, project, result); err != nil {
		return nil, err
	}
	return result, nil
}

// markGitLabFirstTimers sets FirstTimer on the result entries of authors whose first-ever merged
// MR is in this release. GitLab exposes no authorAssociation, so first-timer status is derived
// per ADR-0034 §4: an author is new when they do not appear in any earlier release's MRs. For
// each distinct author one bounded `glab api` call fetches their earliest merged MR; the author
// is a first-timer when that MR's iid is not lower than the smallest MR iid they have in this
// release (i.e. their earliest merged MR *is* one of this release's MRs). Query failures
// propagate so enrichForRelease applies the remote_metadata policy uniformly.
func markGitLabFirstTimers(runner port.Runner, lc *port.LinkContext, project string, result map[string]PullRequest) error {
	minIID := make(map[string]int)
	for _, pr := range result {
		if pr.AuthorLogin == "" {
			continue
		}
		if cur, ok := minIID[pr.AuthorLogin]; !ok || pr.Number < cur {
			minIID[pr.AuthorLogin] = pr.Number
		}
	}

	authors := make([]string, 0, len(minIID))
	for a := range minIID {
		authors = append(authors, a)
	}
	sort.Strings(authors) // deterministic call order

	firstTimer := make(map[string]bool)
	for _, author := range authors {
		earliest, err := gitLabEarliestMergedMR(runner, lc, project, author)
		if err != nil {
			return err
		}
		// earliest == 0 means no merged MR was returned; conservatively treat as not-new.
		if earliest > 0 && earliest >= minIID[author] {
			firstTimer[author] = true
		}
	}

	for sha, pr := range result {
		if firstTimer[pr.AuthorLogin] {
			pr.FirstTimer = true
			result[sha] = pr
		}
	}
	return nil
}

// gitLabEarliestMergedMR returns the iid of author's oldest merged MR in the project, or 0 when
// they have none. Bounded to a single result (per_page=1, oldest first).
func gitLabEarliestMergedMR(runner port.Runner, lc *port.LinkContext, project, author string) (int, error) {
	endpoint := "projects/" + project + "/merge_requests?author_username=" + url.QueryEscape(author) +
		"&state=merged&order_by=created_at&sort=asc&per_page=1"
	stdout, _, err := runner.RunEnv(lc.APIEnv(), "glab", "api", endpoint)
	if err != nil {
		return 0, fmt.Errorf("glab api author merge_requests: %w", err)
	}
	mrs, err := parseGitLabMRs(stdout)
	if err != nil {
		return 0, err
	}
	if len(mrs) == 0 {
		return 0, nil
	}
	return mrs[0].IID, nil
}

type gitLabMR struct {
	IID    int    `json:"iid"`
	WebURL string `json:"web_url"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
}

func parseGitLabMRs(stdout string) ([]gitLabMR, error) {
	var mrs []gitLabMR
	if err := json.Unmarshal([]byte(stdout), &mrs); err != nil {
		return nil, fmt.Errorf("parsing glab api merge_requests response: %w", err)
	}
	return mrs, nil
}
