package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// restMR is one merge request as returned by the REST API. Unlike GraphQL, iid is a number and
// labels is a flat string array.
type restMR struct {
	IID      int      `json:"iid"`
	WebURL   string   `json:"web_url"`
	Title    string   `json:"title"`
	Labels   []string `json:"labels"`
	Author   restUser `json:"author"`
	MergedBy restUser `json:"merged_by"`

	CreatedAt time.Time `json:"created_at"`
	MergedAt  time.Time `json:"merged_at"`
}

type restUser struct {
	Username string `json:"username"`
}

// enrichREST resolves each commit's MR via GET /projects/{id}/repository/commits/{sha}/merge_requests
// — one of the endpoints GitLab allows a CI job token to call. Author handles come from the local
// git metadata (REST exposes no linked username).
func (f *Forge) enrichREST(commits []port.Commit) (port.Enrichment, error) {
	prs := make(map[string]port.PullRequest, len(commits))
	for _, c := range commits {
		mrs, err := f.commitMRs(c.Hash)
		if err != nil {
			return port.Enrichment{}, err
		}
		if len(mrs) == 0 {
			continue
		}
		prs[c.Hash] = mrToPullRequest(mrs[0]) // first association wins, matching the other drivers
	}
	return port.Enrichment{PRs: prs, Authors: gitAuthors(commits)}, nil
}

// commitMRs fetches the merge requests that introduced one commit.
func (f *Forge) commitMRs(sha string) ([]restMR, error) {
	endpoint := fmt.Sprintf("%s/projects/%s/repository/commits/%s/merge_requests",
		f.apiBase(), url.PathEscape(f.id.Project), url.PathEscape(sha))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab commit merge_requests: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	f.setAuth(req)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab commit merge_requests: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab commit merge_requests: unexpected status %s", resp.Status)
	}

	var mrs []restMR
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return nil, fmt.Errorf("gitlab commit merge_requests: decoding response: %w", err)
	}
	return mrs, nil
}

// mrToPullRequest normalizes a REST merge request into the port model. RefPrefix is "!" — GitLab
// renders merge requests as !N.
func mrToPullRequest(m restMR) port.PullRequest {
	return port.PullRequest{
		Number:      m.IID,
		URL:         m.WebURL,
		Title:       m.Title,
		AuthorLogin: m.Author.Username,
		Labels:      m.Labels,
		RefPrefix:   "!",
		CreatedAt:   m.CreatedAt,
		MergedAt:    m.MergedAt,
		MergedBy:    port.Author{Username: m.MergedBy.Username},
	}
}
