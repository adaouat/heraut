package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// ErrJobTokenGraphQL reports the one combination GitLab forbids: a CI job token on GraphQL.
var ErrJobTokenGraphQL = errors.New("gitlab graphql: CI job tokens cannot authenticate GraphQL")

// gqlQuery is one batched query fetching commit-author handles and merged MRs for the project.
// mergeCommitSha and commits.nodes.sha are the SHAs by which an MR can match a target-branch
// commit (ADR-0042); GraphQL exposes no squashed-commit SHA.
const gqlQuery = `query($path:ID!,$ref:String!){project(fullPath:$path){` +
	`repository{commits(ref:$ref,first:100){nodes{sha author{username}}}}` +
	`mergeRequests(state:merged,first:100){nodes{iid webUrl title author{username}` +
	`createdAt mergedAt mergeUser{username}labels{nodes{title}}mergeCommitSha commits{nodes{sha}}}}` +
	`}}`

type gqlResponse struct {
	Data struct {
		Project struct {
			Repository struct {
				Commits struct {
					Nodes []struct {
						SHA    string `json:"sha"`
						Author *struct {
							Username string `json:"username"`
						} `json:"author"`
					} `json:"nodes"`
				} `json:"commits"`
			} `json:"repository"`
			MergeRequests struct {
				Nodes []gqlMR `json:"nodes"`
			} `json:"mergeRequests"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// gqlMR mirrors GitLab's GraphQL scalars: iid is a String and the merger is mergeUser.
type gqlMR struct {
	IID       string                    `json:"iid"`
	WebURL    string                    `json:"webUrl"`
	Title     string                    `json:"title"`
	Author    struct{ Username string } `json:"author"`
	CreatedAt time.Time                 `json:"createdAt"`
	MergedAt  time.Time                 `json:"mergedAt"`
	MergeUser struct{ Username string } `json:"mergeUser"`
	Labels    struct {
		Nodes []struct {
			Title string `json:"title"`
		} `json:"nodes"`
	} `json:"labels"`
	MergeCommitSHA string `json:"mergeCommitSha"`
	Commits        struct {
		Nodes []struct {
			SHA string `json:"sha"`
		} `json:"nodes"`
	} `json:"commits"`
}

// enrichGraphQL resolves linked commit-author handles and MR refs in one batched query. It
// requires a personal/project access token: GitLab rejects job tokens on GraphQL outright, so the
// guard trips before any request is issued.
func (f *Forge) enrichGraphQL(commits []port.Commit) (port.Enrichment, error) {
	if f.id.TokenKind == port.TokenJob {
		return port.Enrichment{}, fmt.Errorf(
			"%w; set api_mode: rest, or supply a read_api token via token_env", ErrJobTokenGraphQL)
	}

	want := make(map[string]bool, len(commits))
	for _, c := range commits {
		want[c.Hash] = true
	}

	resp, err := f.postGraphQL(newestHash(commits))
	if err != nil {
		return port.Enrichment{}, err
	}

	authors := make(map[string]string)
	for _, n := range resp.Data.Project.Repository.Commits.Nodes {
		if want[n.SHA] && n.Author != nil && n.Author.Username != "" {
			authors[n.SHA] = n.Author.Username
		}
	}

	prs := make(map[string]port.PullRequest)
	for _, n := range resp.Data.Project.MergeRequests.Nodes {
		pr := gqlMRToPullRequest(n)
		for _, sha := range landingSHAs(n) {
			if want[sha] {
				if _, seen := prs[sha]; !seen {
					prs[sha] = pr
				}
			}
		}
	}
	return port.Enrichment{PRs: prs, Authors: authors}, nil
}

// postGraphQL issues the batched query against the instance's /api/graphql endpoint.
func (f *Forge) postGraphQL(ref string) (*gqlResponse, error) {
	endpoint := strings.TrimSuffix(f.apiBase(), "/api/v4") + "/api/graphql"
	body, err := json.Marshal(map[string]any{
		"query":     gqlQuery,
		"variables": map[string]string{"path": f.id.Project, "ref": ref},
	})
	if err != nil {
		return nil, fmt.Errorf("gitlab graphql: encoding request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gitlab graphql: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	f.setAuth(req)

	httpResp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab graphql: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab graphql: unexpected status %s", httpResp.Status)
	}

	var out gqlResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gitlab graphql: decoding response: %w", err)
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("gitlab graphql: %s", out.Errors[0].Message)
	}
	return &out, nil
}

// gqlMRToPullRequest normalizes a GraphQL merge request; iid is a String scalar, so it is parsed
// (an unparsable value yields 0).
func gqlMRToPullRequest(n gqlMR) port.PullRequest {
	labels := make([]string, 0, len(n.Labels.Nodes))
	for _, l := range n.Labels.Nodes {
		labels = append(labels, l.Title)
	}
	if len(labels) == 0 {
		labels = nil
	}
	num, _ := strconv.Atoi(n.IID)
	return port.PullRequest{
		Number: num, URL: n.WebURL, Title: n.Title, AuthorLogin: n.Author.Username,
		Labels: labels, RefPrefix: "!", CreatedAt: n.CreatedAt, MergedAt: n.MergedAt,
		MergedBy: port.Author{Username: n.MergeUser.Username},
	}
}

// landingSHAs are the SHAs by which an MR can match a target-branch commit: the merge commit and
// each source commit (fast-forward merges land those directly).
func landingSHAs(n gqlMR) []string {
	shas := make([]string, 0, len(n.Commits.Nodes)+1)
	if n.MergeCommitSHA != "" {
		shas = append(shas, n.MergeCommitSHA)
	}
	for _, c := range n.Commits.Nodes {
		shas = append(shas, c.SHA)
	}
	return shas
}

// newestHash returns the hash of the newest-dated commit — the commits(ref:) anchor.
func newestHash(commits []port.Commit) string {
	var newest port.Commit
	for _, c := range commits {
		if newest.Date.IsZero() || c.Date.After(newest.Date) {
			newest = c
		}
	}
	return newest.Hash
}
