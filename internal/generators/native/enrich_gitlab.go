package native

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/adaouat/heraut/internal/port"
)

// enrichGitLab resolves the merge request associated with each commit SHA via per-commit
// `glab api projects/{id}/repository/commits/{sha}/merge_requests` calls. GitLab has no
// batched per-commit-MR primitive like GitHub's GraphQL associatedPullRequests, so this is one
// call per commit (the enrichment range is the bounded release). Commits with no MR are absent
// from the map. First-time-contributor status is not derived for GitLab (the API has no
// authorAssociation), so FirstTimer stays false and no New Contributors block is produced.
func enrichGitLab(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]prInfo, error) {
	project := url.PathEscape(lc.Owner + "/" + lc.Repo)
	result := make(map[string]prInfo)
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
		mr := mrs[0]
		result[sha] = prInfo{
			Number:      mr.IID,
			URL:         mr.WebURL,
			AuthorLogin: mr.Author.Username,
			RefPrefix:   "!",
		}
	}
	return result, nil
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
