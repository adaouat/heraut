package native

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

const (
	// ghChunkSize is the maximum number of commit SHAs per GraphQL query, bounding
	// the query size and staying well within GitHub's API node limits.
	ghChunkSize = 50

	prFragment = "...on Commit{associatedPullRequests(first:1){nodes{number url author{login}authorAssociation}}}"
)

// prInfo holds enrichment data for the pull request associated with a commit.
type prInfo struct {
	Number      int
	URL         string
	AuthorLogin string
	FirstTimer  bool   // GitHub authorAssociation == "FIRST_TIME_CONTRIBUTOR"
	RefPrefix   string // "#" for GitHub PRs, "!" for GitLab MRs; empty defaults to "#"
}

// enrichGitHub fetches the associated pull request for each SHA via batched gh api graphql
// calls (at most ghChunkSize SHAs per query). Commits with no associated PR are absent from
// the returned map. All errors are wrapped before being returned.
func enrichGitHub(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]prInfo, error) {
	result := make(map[string]prInfo)
	for i := 0; i < len(shas); i += ghChunkSize {
		end := i + ghChunkSize
		if end > len(shas) {
			end = len(shas)
		}
		partial, err := fetchGitHubChunk(runner, lc, shas[i:end])
		if err != nil {
			return nil, fmt.Errorf("enriching GitHub PRs (chunk %d): %w", i/ghChunkSize, err)
		}
		for k, v := range partial {
			result[k] = v
		}
	}
	return result, nil
}

// fetchGitHubChunk issues one gh api graphql call for a chunk of SHAs and returns the
// sha→prInfo map for that chunk. Aliases are local to the chunk (s0, s1, …).
func fetchGitHubChunk(runner port.Runner, lc *port.LinkContext, shas []string) (map[string]prInfo, error) {
	query := buildGitHubQuery(lc.Owner, lc.Repo, shas)
	stdout, _, err := runner.RunEnv(lc.APIEnv(), "gh", "api", "graphql", "-f", "query="+query)
	if err != nil {
		return nil, fmt.Errorf("gh api graphql: %w", err)
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
	AssociatedPullRequests struct {
		Nodes []graphQLPR `json:"nodes"`
	} `json:"associatedPullRequests"`
}

type graphQLPR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	AuthorAssociation string `json:"authorAssociation"`
}

// parseGitHubResponse decodes the gh api graphql JSON and maps each SHA to its prInfo.
// Aliases absent from the response or with empty nodes are silently skipped (no PR).
func parseGitHubResponse(stdout string, shas []string) (map[string]prInfo, error) {
	var resp graphQLResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		return nil, fmt.Errorf("parsing gh api graphql response: %w", err)
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("gh api graphql: %s", resp.Errors[0].Message)
	}
	result := make(map[string]prInfo)
	for i, sha := range shas {
		alias := fmt.Sprintf("s%d", i)
		commit, ok := resp.Data.Repository[alias]
		if !ok || commit == nil || len(commit.AssociatedPullRequests.Nodes) == 0 {
			continue
		}
		pr := commit.AssociatedPullRequests.Nodes[0]
		result[sha] = prInfo{
			Number:      pr.Number,
			URL:         pr.URL,
			AuthorLogin: pr.Author.Login,
			FirstTimer:  pr.AuthorAssociation == "FIRST_TIME_CONTRIBUTOR",
		}
	}
	return result, nil
}
