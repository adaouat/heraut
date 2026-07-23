package native

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// enrichGitLab returns sha→MR and sha→commit-author-handle for the given SHAs via two batched
// glab api graphql connection queries. since bounds pagination to the release window; ref is the
// commits(ref:) anchor (the range tip). Returns empty maps for no SHAs.
func enrichGitLab(runner port.Runner, lc *port.LinkContext, shas []string, since time.Time, ref string) (map[string]PullRequest, map[string]string, error) {
	if len(shas) == 0 {
		return map[string]PullRequest{}, map[string]string{}, nil
	}
	want := make(map[string]bool, len(shas))
	for _, s := range shas {
		want[s] = true
	}
	sinceStr := ""
	if !since.IsZero() {
		// Subtract a small buffer so a boundary commit/MR at exactly `since` is not excluded by an
		// exclusive committedAfter/mergedAfter. SHA match is authoritative; this only bounds pagination.
		sinceStr = since.Add(-time.Minute).UTC().Format(time.RFC3339)
	}
	authors, err := fetchGitLabAuthors(runner, lc, ref, sinceStr, want)
	if err != nil {
		return nil, nil, err
	}
	prs, err := fetchGitLabMRs(runner, lc, sinceStr, want)
	if err != nil {
		return nil, nil, err
	}
	return prs, authors, nil
}

func projectPath(lc *port.LinkContext) string { return lc.Owner + "/" + lc.Repo }

func gitLabAuthorsQuery(project, ref, committedAfter, after string) string {
	var extra strings.Builder
	if committedAfter != "" {
		fmt.Fprintf(&extra, `,committedAfter:%s`, gqlString(committedAfter))
	}
	if after != "" {
		fmt.Fprintf(&extra, `,after:%s`, gqlString(after))
	}
	return fmt.Sprintf(`{project(fullPath:%s){repository{commits(ref:%s%s,first:100){nodes{sha author{username}}pageInfo{endCursor hasNextPage}}}}}`,
		gqlString(project), gqlString(ref), extra.String())
}

type gitLabCommitsResponse struct {
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
					PageInfo struct {
						EndCursor   string `json:"endCursor"`
						HasNextPage bool   `json:"hasNextPage"`
					} `json:"pageInfo"`
				} `json:"commits"`
			} `json:"repository"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// fetchGitLabAuthors pages through project.repository.commits(ref:) to resolve a
// sha→author.username map for the SHAs in want. Author can be null (e.g. unlinked commit
// email) — those SHAs are omitted from the returned map. Paging stops early once every SHA
// in want has been seen, even if GitLab reports more pages.
func fetchGitLabAuthors(runner port.Runner, lc *port.LinkContext, ref, committedAfter string, want map[string]bool) (map[string]string, error) {
	authors := make(map[string]string)
	seen := make(map[string]bool)
	after := ""
	for {
		query := gitLabAuthorsQuery(projectPath(lc), ref, committedAfter, after)
		stdout, _, err := runner.RunEnv(lc.APIEnv(), "glab", "api", "graphql", "-f", "query="+query)
		if err != nil {
			return nil, fmt.Errorf("glab api graphql commits: %w", err)
		}
		var resp gitLabCommitsResponse
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			return nil, fmt.Errorf("parsing glab graphql commits response: %w", err)
		}
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("glab graphql commits: %s", resp.Errors[0].Message)
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

func gitLabMRsQuery(project, mergedAfter, after string) string {
	var extra strings.Builder
	if mergedAfter != "" {
		fmt.Fprintf(&extra, `,mergedAfter:%s`, gqlString(mergedAfter))
	}
	if after != "" {
		fmt.Fprintf(&extra, `,after:%s`, gqlString(after))
	}
	return fmt.Sprintf(`{project(fullPath:%s){mergeRequests(state:merged%s,first:100){nodes{iid webUrl title author{username}createdAt mergedAt mergeUser{username}labels{nodes{title}}mergeCommitSha commits{nodes{sha}}}pageInfo{endCursor hasNextPage}}}}`,
		gqlString(project), extra.String())
}

// gitLabMRNode: GitLab GraphQL scalars — iid is a String, the merger is mergeUser (not mergedBy),
// and there is no squashCommitSha field (only squashOnMerge bool), confirmed by the Task 1 spike.
type gitLabMRNode struct {
	IID    string `json:"iid"`
	WebURL string `json:"webUrl"`
	Title  string `json:"title"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	MergedAt  time.Time `json:"mergedAt"`
	MergeUser struct {
		Username string `json:"username"`
	} `json:"mergeUser"`
	Labels struct {
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

type gitLabMRsResponse struct {
	Data struct {
		Project struct {
			MergeRequests struct {
				Nodes    []gitLabMRNode `json:"nodes"`
				PageInfo struct {
					EndCursor   string `json:"endCursor"`
					HasNextPage bool   `json:"hasNextPage"`
				} `json:"pageInfo"`
			} `json:"mergeRequests"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func mrNodeToPR(n gitLabMRNode) PullRequest {
	labels := make([]string, 0, len(n.Labels.Nodes))
	for _, l := range n.Labels.Nodes {
		labels = append(labels, l.Title)
	}
	if len(labels) == 0 {
		labels = nil
	}
	num, _ := strconv.Atoi(n.IID) // GitLab GraphQL iid is a String; unparsable → 0
	return PullRequest{
		Number: num, URL: n.WebURL, Title: n.Title, AuthorLogin: n.Author.Username,
		Labels: labels, RefPrefix: "!", CreatedAt: n.CreatedAt, MergedAt: n.MergedAt,
		MergedBy: Author{Username: n.MergeUser.Username},
	}
}

// landingSHAs are the SHAs by which an MR can match a target-branch commit: the merge commit
// (merge-commit and squash-with-merge-commit merges) and each source commit (fast-forward merges
// land those directly). GitLab GraphQL exposes no squashed-commit SHA, so a squash+fast-forward MR
// matches nothing and its commit renders no ref — a graceful omission.
func landingSHAs(n gitLabMRNode) []string {
	shas := make([]string, 0, len(n.Commits.Nodes)+1)
	if n.MergeCommitSHA != "" {
		shas = append(shas, n.MergeCommitSHA)
	}
	for _, c := range n.Commits.Nodes {
		shas = append(shas, c.SHA)
	}
	return shas
}

func fetchGitLabMRs(runner port.Runner, lc *port.LinkContext, mergedAfter string, want map[string]bool) (map[string]PullRequest, error) {
	prs := make(map[string]PullRequest)
	after := ""
	for {
		query := gitLabMRsQuery(projectPath(lc), mergedAfter, after)
		stdout, _, err := runner.RunEnv(lc.APIEnv(), "glab", "api", "graphql", "-f", "query="+query)
		if err != nil {
			return nil, fmt.Errorf("glab api graphql merge_requests: %w", err)
		}
		var resp gitLabMRsResponse
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			return nil, fmt.Errorf("parsing glab graphql merge_requests response: %w", err)
		}
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("glab graphql merge_requests: %s", resp.Errors[0].Message)
		}
		mrs := resp.Data.Project.MergeRequests
		for _, n := range mrs.Nodes {
			pr := mrNodeToPR(n)
			for _, sha := range landingSHAs(n) {
				if want[sha] {
					if _, ok := prs[sha]; !ok {
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
