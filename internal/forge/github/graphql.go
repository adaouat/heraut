package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

const (
	// ghChunkSize is the maximum number of commit SHAs per GraphQL query, bounding
	// the query size and staying well within GitHub's API node limits.
	ghChunkSize = 50

	prFragment = "...on Commit{author{user{login}}associatedPullRequests(first:1){nodes{number url title author{login}labels(first:20){nodes{name}}createdAt mergedAt mergedBy{login}latestReviews(first:20){nodes{state author{login}}}}}}"
)

// enrichGraphQL fetches the associated pull request and commit-author handle for each SHA via
// batched GraphQL queries (at most ghChunkSize SHAs per query). Commits with no associated PR are
// absent from the PR map; commits whose author email isn't linked to a GitHub user are absent
// from the authors map. All errors are wrapped before being returned.
func (f *Forge) enrichGraphQL(commits []port.Commit) (port.Enrichment, error) {
	shas := make([]string, len(commits))
	for i, c := range commits {
		shas[i] = c.Hash
	}

	owner, repo, _ := strings.Cut(f.id.Project, "/")

	prs := make(map[string]port.PullRequest)
	authors := make(map[string]string)
	for i := 0; i < len(shas); i += ghChunkSize {
		end := i + ghChunkSize
		if end > len(shas) {
			end = len(shas)
		}
		p, a, err := f.fetchChunk(owner, repo, shas[i:end])
		if err != nil {
			return port.Enrichment{}, fmt.Errorf("enriching GitHub PRs (chunk %d): %w", i/ghChunkSize, err)
		}
		for k, v := range p {
			prs[k] = v
		}
		for k, v := range a {
			authors[k] = v
		}
	}
	return port.Enrichment{PRs: prs, Authors: authors}, nil
}

// fetchChunk issues one GraphQL POST for a chunk of SHAs and returns the sha→PullRequest and
// sha→author-handle maps for that chunk. Aliases are local to the chunk (s0, s1, …).
func (f *Forge) fetchChunk(owner, repo string, shas []string) (map[string]port.PullRequest, map[string]string, error) {
	query := buildGitHubQuery(owner, repo, shas)

	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling GraphQL request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, f.graphqlEndpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("building GraphQL request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if f.id.Token != "" {
		req.Header.Set("Authorization", "bearer "+f.id.Token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("github graphql request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading github graphql response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("github graphql: unexpected status %s", resp.Status)
	}

	return parseGitHubResponse(respBody, shas)
}

// buildGitHubQuery constructs a compact batched GraphQL query: one aliased object per SHA.
func buildGitHubQuery(owner, repo string, shas []string) string {
	var sb strings.Builder
	sb.WriteString(`{repository(owner:`)
	sb.WriteString(gqlString(owner))
	sb.WriteString(`,name:`)
	sb.WriteString(gqlString(repo))
	sb.WriteString(`){`)
	for i, sha := range shas {
		fmt.Fprintf(&sb, `s%d:object(oid:%s){%s}`, i, gqlString(sha), prFragment)
	}
	sb.WriteString("}}")
	return sb.String()
}

type graphQLResponse struct {
	Data struct {
		Repository map[string]*graphQLCommit `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type graphQLCommit struct {
	Author struct {
		User *struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"author"`
	AssociatedPullRequests struct {
		Nodes []graphQLPR `json:"nodes"`
	} `json:"associatedPullRequests"`
}

type graphQLPR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	CreatedAt time.Time `json:"createdAt"`
	MergedAt  time.Time `json:"mergedAt"`
	MergedBy  struct {
		Login string `json:"login"`
	} `json:"mergedBy"`
	LatestReviews struct {
		Nodes []struct {
			State  string `json:"state"`
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
		} `json:"nodes"`
	} `json:"latestReviews"`
}

// parseGitHubResponse decodes the GraphQL JSON and maps each SHA to its PullRequest and
// commit-author handle. Aliases absent from the response are silently skipped. A commit with an
// author but no associated PR still records its handle; empty PR nodes yield no PR entry.
func parseGitHubResponse(body []byte, shas []string) (map[string]port.PullRequest, map[string]string, error) {
	var resp graphQLResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("parsing github graphql response: %w", err)
	}
	if len(resp.Errors) > 0 {
		return nil, nil, fmt.Errorf("github graphql: %s", resp.Errors[0].Message)
	}
	prs := make(map[string]port.PullRequest)
	authors := make(map[string]string)
	for i, sha := range shas {
		alias := fmt.Sprintf("s%d", i)
		commit, ok := resp.Data.Repository[alias]
		if !ok || commit == nil {
			continue
		}
		if commit.Author.User != nil && commit.Author.User.Login != "" {
			authors[sha] = commit.Author.User.Login
		}
		if len(commit.AssociatedPullRequests.Nodes) == 0 {
			continue
		}
		pr := commit.AssociatedPullRequests.Nodes[0]
		var labels []string
		for _, l := range pr.Labels.Nodes {
			labels = append(labels, l.Name)
		}
		var approvers []port.Author
		for _, r := range pr.LatestReviews.Nodes {
			if r.State == "APPROVED" && r.Author.Login != "" {
				approvers = append(approvers, port.Author{Username: r.Author.Login})
			}
		}
		prs[sha] = port.PullRequest{
			Number:      pr.Number,
			URL:         pr.URL,
			Title:       pr.Title,
			AuthorLogin: pr.Author.Login,
			Labels:      labels,
			RefPrefix:   "#",
			CreatedAt:   pr.CreatedAt,
			MergedAt:    pr.MergedAt,
			MergedBy:    port.Author{Username: pr.MergedBy.Login},
			Approvers:   approvers,
		}
	}
	return prs, authors, nil
}

// gqlString renders a Go string as a double-quoted GraphQL string literal.
func gqlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
