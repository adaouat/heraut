package native

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// enrichGitLab resolves the merge request associated with each commit SHA via per-commit
// `glab api projects/{id}/repository/commits/{sha}/merge_requests` calls. GitLab has no
// batched per-commit-MR primitive like GitHub's GraphQL associatedPullRequests, so this is one
// call per commit (the enrichment range is the bounded release). Commits with no MR are absent
// from the map.
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
			Title:       mr.Title,
			AuthorLogin: mr.Author.Username,
			Labels:      mr.Labels,
			RefPrefix:   "!",
			CreatedAt:   mr.CreatedAt,
			MergedAt:    mr.MergedAt,
			MergedBy:    Author{Username: mr.MergedBy.Username},
			// Approvers intentionally left nil — the per-commit MR object has no approvers; a
			// separate /approvals call per MR is not paid (best-effort, ADR-0036 / spec).
		}
	}
	return result, nil
}

type gitLabMR struct {
	IID    int      `json:"iid"`
	WebURL string   `json:"web_url"`
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	MergedAt  time.Time `json:"merged_at"`
	MergedBy  struct {
		Username string `json:"username"`
	} `json:"merged_by"`
}

func parseGitLabMRs(stdout string) ([]gitLabMR, error) {
	var mrs []gitLabMR
	if err := json.Unmarshal([]byte(stdout), &mrs); err != nil {
		return nil, fmt.Errorf("parsing glab api merge_requests response: %w", err)
	}
	return mrs, nil
}
