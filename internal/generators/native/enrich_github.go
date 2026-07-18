package native

import (
	"encoding/json"
	"fmt"
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

// enrichGitHub fetches the associated pull request and commit-author handle for each SHA via
// batched gh api graphql calls (at most ghChunkSize SHAs per query). Commits with no associated
// PR are absent from the PR map; commits whose author email isn't linked to a GitHub user are
// absent from the authors map. All errors are wrapped before being returned.
func enrichGitHub(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]PullRequest, map[string]string, error) {
	prs := make(map[string]PullRequest)
	authors := make(map[string]string)
	for i := 0; i < len(shas); i += ghChunkSize {
		end := i + ghChunkSize
		if end > len(shas) {
			end = len(shas)
		}
		p, a, err := fetchGitHubChunk(runner, lc, shas[i:end])
		if err != nil {
			return nil, nil, fmt.Errorf("enriching GitHub PRs (chunk %d): %w", i/ghChunkSize, err)
		}
		for k, v := range p {
			prs[k] = v
		}
		for k, v := range a {
			authors[k] = v
		}
	}
	return prs, authors, nil
}

// fetchGitHubChunk issues one gh api graphql call for a chunk of SHAs and returns the
// sha→PullRequest and sha→author-handle maps for that chunk. Aliases are local to the chunk
// (s0, s1, …).
func fetchGitHubChunk(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]PullRequest, map[string]string, error) {
	query := buildGitHubQuery(lc.Owner, lc.Repo, shas)
	stdout, _, err := runner.RunEnv(lc.APIEnv(), "gh", "api", "graphql", "-f", "query="+query)
	if err != nil {
		return nil, nil, fmt.Errorf("gh api graphql: %w", err)
	}
	return parseGitHubResponse(stdout, shas)
}

// buildGitHubQuery constructs a compact batched GraphQL query: one aliased object per SHA.
func buildGitHubQuery(owner, repo string, shas []string) string {
	var sb strings.Builder
	sb.WriteString(`{repository(owner:"`)
	sb.WriteString(owner)
	sb.WriteString(`",name:"`)
	sb.WriteString(repo)
	sb.WriteString(`"){`)
	for i, sha := range shas {
		fmt.Fprintf(&sb, `s%d:object(oid:"%s"){%s}`, i, sha, prFragment)
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

// parseGitHubResponse decodes the gh api graphql JSON and maps each SHA to its PullRequest and
// commit-author handle. Aliases absent from the response are silently skipped. A commit with an
// author but no associated PR still records its handle; empty PR nodes yield no PR entry.
func parseGitHubResponse(stdout string, shas []string) (map[string]PullRequest, map[string]string, error) {
	var resp graphQLResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		return nil, nil, fmt.Errorf("parsing gh api graphql response: %w", err)
	}
	if len(resp.Errors) > 0 {
		return nil, nil, fmt.Errorf("gh api graphql: %s", resp.Errors[0].Message)
	}
	prs := make(map[string]PullRequest)
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
		var approvers []Author
		for _, r := range pr.LatestReviews.Nodes {
			if r.State == "APPROVED" && r.Author.Login != "" {
				approvers = append(approvers, Author{Username: r.Author.Login})
			}
		}
		prs[sha] = PullRequest{
			Number:      pr.Number,
			URL:         pr.URL,
			Title:       pr.Title,
			AuthorLogin: pr.Author.Login,
			Labels:      labels,
			CreatedAt:   pr.CreatedAt,
			MergedAt:    pr.MergedAt,
			MergedBy:    Author{Username: pr.MergedBy.Login},
			Approvers:   approvers,
		}
	}
	return prs, authors, nil
}
