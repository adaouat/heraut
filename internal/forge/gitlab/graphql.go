package gitlab

import (
	"bytes"
	"context"
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

// gqlCommitsQuery fetches commit-author handles for the release window, one page at a time.
// committedAfter bounds the scan to the window; after advances the cursor. Arguments are inlined
// (quoted via gqlString) exactly as the live-validated legacy query does (see enrichGraphQL) rather
// than declared as typed GraphQL variables.
func gqlCommitsQuery(project, ref, committedAfter, after string) string {
	var extra strings.Builder
	if committedAfter != "" {
		fmt.Fprintf(&extra, `,committedAfter:%s`, gqlString(committedAfter))
	}
	if after != "" {
		fmt.Fprintf(&extra, `,after:%s`, gqlString(after))
	}
	return fmt.Sprintf(`{project(fullPath:%s){repository{commits(ref:%s%s,first:100){`+
		`nodes{sha author{username}}pageInfo{endCursor hasNextPage}}}}}`,
		gqlString(project), gqlString(ref), extra.String())
}

// gqlMRsQuery fetches merged MRs for the release window, one page at a time. mergedAfter is what
// keeps an unsorted connection from returning MRs unrelated to this release.
func gqlMRsQuery(project, mergedAfter, after string) string {
	var extra strings.Builder
	if mergedAfter != "" {
		fmt.Fprintf(&extra, `,mergedAfter:%s`, gqlString(mergedAfter))
	}
	if after != "" {
		fmt.Fprintf(&extra, `,after:%s`, gqlString(after))
	}
	return fmt.Sprintf(`{project(fullPath:%s){mergeRequests(state:merged%s,first:100){`+
		`nodes{iid webUrl title author{username}createdAt mergedAt mergeUser{username}`+
		`labels{nodes{title}}mergeCommitSha commits(first:100){nodes{sha}pageInfo{endCursor hasNextPage}}}`+
		`pageInfo{endCursor hasNextPage}}}}`,
		gqlString(project), extra.String())
}

// gqlMRCommitsQuery fetches the next page of one merge request's commits, for an MR with more than
// the 100 the merge request query returns.
func gqlMRCommitsQuery(project, iid, after string) string {
	return fmt.Sprintf(`{project(fullPath:%s){mergeRequest(iid:%s){commits(first:100,after:%s){`+
		`nodes{sha}pageInfo{endCursor hasNextPage}}}}}`,
		gqlString(project), gqlString(iid), gqlString(after))
}

type gqlMRCommitsResponse struct {
	Data struct {
		Project struct {
			MergeRequest struct {
				Commits gqlCommitPage `json:"commits"`
			} `json:"mergeRequest"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// gqlString renders s as a GraphQL string literal so an interpolated value cannot break out of it.
func gqlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

type gqlPageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

type gqlCommitsResponse struct {
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
					PageInfo gqlPageInfo `json:"pageInfo"`
				} `json:"commits"`
			} `json:"repository"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type gqlMRsResponse struct {
	Data struct {
		Project struct {
			MergeRequests struct {
				Nodes    []gqlMR     `json:"nodes"`
				PageInfo gqlPageInfo `json:"pageInfo"`
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
	MergeCommitSHA string        `json:"mergeCommitSha"`
	Commits        gqlCommitPage `json:"commits"`
}

// gqlCommitPage is one page of a merge request's commits.
type gqlCommitPage struct {
	Nodes []struct {
		SHA string `json:"sha"`
	} `json:"nodes"`
	PageInfo gqlPageInfo `json:"pageInfo"`
}

// oldestCommitDate returns the minimum commit date — the lower bound of the release window — or
// the zero time when commits is empty.
func oldestCommitDate(commits []port.Commit) time.Time {
	var oldest time.Time
	for _, c := range commits {
		if oldest.IsZero() || c.Date.Before(oldest) {
			oldest = c.Date
		}
	}
	return oldest
}

// enrichGraphQL resolves linked commit-author handles and MR refs via two paginated, release-window
// bounded queries. It requires a personal/project access token: GitLab rejects job tokens on
// GraphQL outright, so the guard trips before any request is issued. ref anchors the
// commits(ref:) walk to the release window's true range tip — the caller resolves it (a tag name,
// or a commit SHA for the unreleased section), rather than this function guessing it from commit
// dates, which breaks under rewritten committer dates (rebase/amend/cherry-pick — T153).
func (f *Forge) enrichGraphQL(ctx context.Context, commits []port.Commit, ref string) (port.Enrichment, error) {
	if f.id.TokenKind == port.TokenJob {
		return port.Enrichment{}, fmt.Errorf(
			"%w; set api_mode: rest, or supply a read_api token via token_env", ErrJobTokenGraphQL)
	}

	want := make(map[string]bool, len(commits))
	for _, c := range commits {
		want[c.Hash] = true
	}

	since := ""
	if oldest := oldestCommitDate(commits); !oldest.IsZero() {
		// Subtract a small buffer so a boundary commit/MR at exactly `oldest` is not excluded by an
		// exclusive committedAfter/mergedAfter. SHA matching remains authoritative; this only bounds
		// pagination.
		since = oldest.Add(-time.Minute).UTC().Format(time.RFC3339)
	}

	authors, err := f.fetchGraphQLAuthors(ctx, ref, since, want)
	if err != nil {
		return port.Enrichment{}, err
	}
	prs, err := f.fetchGraphQLMRs(ctx, since, want)
	if err != nil {
		return port.Enrichment{}, err
	}
	return port.Enrichment{PRs: prs, Authors: authors}, nil
}

// fetchGraphQLAuthors pages through project.repository.commits(ref:) to resolve a
// sha→author.username map for the SHAs in want. Paging stops early once every SHA in want has been
// seen, even if GitLab reports more pages.
func (f *Forge) fetchGraphQLAuthors(ctx context.Context, ref, committedAfter string, want map[string]bool) (map[string]string, error) {
	authors := make(map[string]string)
	seen := make(map[string]bool)
	after := ""
	for {
		query := gqlCommitsQuery(f.id.Project, ref, committedAfter, after)
		var resp gqlCommitsResponse
		if err := f.postGraphQL(ctx, query, &resp); err != nil {
			return nil, fmt.Errorf("gitlab graphql commits: %w", err)
		}
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("gitlab graphql commits: %s", resp.Errors[0].Message)
		}
		c := resp.Data.Project.Repository.Commits
		for _, n := range c.Nodes {
			if !want[n.SHA] {
				continue
			}
			seen[n.SHA] = true
			if n.Author != nil && n.Author.Username != "" {
				authors[n.SHA] = n.Author.Username
			}
		}
		if !c.PageInfo.HasNextPage || len(seen) == len(want) {
			return authors, nil
		}
		if c.PageInfo.EndCursor == "" {
			// Malformed hasNextPage:true with no cursor to advance on — stop instead of refetching
			// page 1 forever.
			return authors, nil
		}
		after = c.PageInfo.EndCursor
	}
}

// fetchGraphQLMRs pages through project.mergeRequests(state:merged) to resolve a sha→PR map for the
// SHAs in want. Paging stops early once every SHA in want has a resolved PR.
func (f *Forge) fetchGraphQLMRs(ctx context.Context, mergedAfter string, want map[string]bool) (map[string]port.PullRequest, error) {
	prs := make(map[string]port.PullRequest)
	after := ""
	for {
		query := gqlMRsQuery(f.id.Project, mergedAfter, after)
		var resp gqlMRsResponse
		if err := f.postGraphQL(ctx, query, &resp); err != nil {
			return nil, fmt.Errorf("gitlab graphql merge_requests: %w", err)
		}
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("gitlab graphql merge_requests: %s", resp.Errors[0].Message)
		}
		mrs := resp.Data.Project.MergeRequests
		for _, n := range mrs.Nodes {
			pr := gqlMRToPullRequest(n)
			more, err := f.remainingMRCommits(ctx, n)
			if err != nil {
				return nil, err
			}
			for _, sha := range append(landingSHAs(n), more...) {
				if want[sha] {
					if _, seen := prs[sha]; !seen {
						prs[sha] = pr
					}
				}
			}
		}
		if !mrs.PageInfo.HasNextPage || len(prs) == len(want) {
			return prs, nil
		}
		if mrs.PageInfo.EndCursor == "" {
			// Malformed hasNextPage:true with no cursor to advance on — stop instead of refetching
			// page 1 forever.
			return prs, nil
		}
		after = mrs.PageInfo.EndCursor
	}
}

// remainingMRCommits pages through the commits of n past the first page the merge request query
// returned, so a commit of a long MR (over 100 commits) is still credited to it.
func (f *Forge) remainingMRCommits(ctx context.Context, n gqlMR) ([]string, error) {
	var shas []string
	page := n.Commits.PageInfo
	for page.HasNextPage && page.EndCursor != "" {
		var resp gqlMRCommitsResponse
		if err := f.postGraphQL(ctx, gqlMRCommitsQuery(f.id.Project, n.IID, page.EndCursor), &resp); err != nil {
			return nil, fmt.Errorf("gitlab graphql commits of !%s: %w", n.IID, err)
		}
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("gitlab graphql commits of !%s: %s", n.IID, resp.Errors[0].Message)
		}
		commits := resp.Data.Project.MergeRequest.Commits
		for _, c := range commits.Nodes {
			shas = append(shas, c.SHA)
		}
		page = commits.PageInfo
	}
	return shas, nil
}

// postGraphQL issues query against the instance's /api/graphql endpoint and decodes the response
// into out. Arguments are inlined into the query string itself (see gqlCommitsQuery/gqlMRsQuery),
// so the request carries no variables object.
func (f *Forge) postGraphQL(ctx context.Context, query string, out any) error {
	endpoint := strings.TrimSuffix(f.apiBase(), "/api/v4") + "/api/graphql"
	body, err := json.Marshal(map[string]any{"query": query})
	if err != nil {
		return fmt.Errorf("gitlab graphql: encoding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gitlab graphql: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	f.setAuth(req)

	httpResp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab graphql: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("gitlab graphql: unexpected status %s", httpResp.Status)
	}

	if err := json.NewDecoder(httpResp.Body).Decode(out); err != nil {
		return fmt.Errorf("gitlab graphql: decoding response: %w", err)
	}
	return nil
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
