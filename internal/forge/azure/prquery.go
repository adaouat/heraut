package azure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// apiVersion pins the Azure DevOps REST API version for pullrequestquery (ADR-0035).
const apiVersion = "7.1"

// voteApprovedWithSuggestions is the lower of Azure's two approving votes (5, and 10 for approved);
// 0 is no vote and the negative ones wait for the author or reject.
const voteApprovedWithSuggestions = 5

// prQueryChunkSize is the most commit SHAs sent in one pullrequestquery. Azure documents no limit;
// chunking keeps each request and response small, and each within the client timeout.
const prQueryChunkSize = 100

// enrichPRs resolves the pull request associated with each commit SHA via batched Azure
// DevOps `pullrequestquery` POSTs (ADR-0035), at most prQueryChunkSize SHAs each, correlating
// `results[0][sha] → PR`. Commits with no PR are absent from the map. Title and labels are
// populated best-effort from the response (labels may be empty if the API requires an expand);
// first-timer detection and the New Contributors block are computed by the shared local git
// tier, not here. Auth is a PAT (id.Token) via HTTP Basic; any transport error or non-2xx status
// is wrapped so enrichForRelease applies the remote_metadata policy.
func (f *Forge) enrichPRs(ctx context.Context, shas []string) (map[string]port.PullRequest, error) {
	result := make(map[string]port.PullRequest)
	if len(shas) == 0 {
		return result, nil
	}

	if f.id.Project == "" {
		return nil, fmt.Errorf("azure pullrequestquery: Project is empty")
	}
	if f.id.Repository == "" {
		return nil, fmt.Errorf("azure pullrequestquery: Repository is empty")
	}
	for i := 0; i < len(shas); i += prQueryChunkSize {
		chunk := shas[i:min(i+prQueryChunkSize, len(shas))]
		if err := f.queryPRs(ctx, chunk, result); err != nil {
			return nil, fmt.Errorf("azure pullrequestquery (chunk %d): %w", i/prQueryChunkSize, err)
		}
	}
	return result, nil
}

// queryPRs sends one pullrequestquery for shas and adds the pull requests found to result.
func (f *Forge) queryPRs(ctx context.Context, shas []string, result map[string]port.PullRequest) error {
	// Host carries no organization ("https://dev.azure.com", or an Azure DevOps Server root without
	// its collection) and Project is "organization/project" (ADR-0035), so Project is appended
	// as-is, one opaque segment, matching webBase()'s formula: the organization appears once.
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequestquery?api-version=%s",
		strings.TrimRight(f.id.Host, "/"), f.id.Project, url.PathEscape(f.id.Repository), apiVersion)

	body, err := json.Marshal(prQuery{Queries: []prQueryInput{{Type: "lastMergeCommit", Items: shas}}})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if f.id.Token != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+f.id.Token)))
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	var out prQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	if len(out.Results) == 0 {
		return nil
	}

	prWebBase := f.webBase() + "/pullrequest/"
	for sha, prs := range out.Results[0] {
		if len(prs) == 0 {
			continue
		}
		pr := prs[0] // first association wins, matching the GitHub/GitLab drivers
		var labels []string
		for _, l := range pr.Labels {
			labels = append(labels, l.Name)
		}
		var approvers []port.Author
		for _, r := range pr.Reviewers {
			if r.Vote >= voteApprovedWithSuggestions {
				approvers = append(approvers, port.Author{Username: authorLogin(identityRef{DisplayName: r.DisplayName, UniqueName: r.UniqueName})})
			}
		}
		result[sha] = port.PullRequest{
			Number:      pr.PullRequestID,
			URL:         prWebBase + strconv.Itoa(pr.PullRequestID),
			AuthorLogin: authorLogin(pr.CreatedBy),
			RefPrefix:   "!",
			Title:       pr.Title,
			Labels:      labels,
			CreatedAt:   pr.CreationDate,
			MergedAt:    pr.ClosedDate,
			MergedBy:    port.Author{Username: authorLogin(pr.ClosedBy)},
			Approvers:   approvers,
		}
	}
	return nil
}

type prQuery struct {
	Queries []prQueryInput `json:"queries"`
}

type prQueryInput struct {
	Type  string   `json:"type"`
	Items []string `json:"items"`
}

type prQueryResult struct {
	Results []map[string][]azurePR `json:"results"`
}

type azurePR struct {
	PullRequestID int         `json:"pullRequestId"`
	Title         string      `json:"title"`
	CreatedBy     identityRef `json:"createdBy"`
	Labels        []struct {
		Name string `json:"name"`
	} `json:"labels"`
	CreationDate time.Time   `json:"creationDate"`
	ClosedDate   time.Time   `json:"closedDate"`
	ClosedBy     identityRef `json:"closedBy"`
	Reviewers    []struct {
		UniqueName  string `json:"uniqueName"`
		DisplayName string `json:"displayName"`
		Vote        int    `json:"vote"`
	} `json:"reviewers"`
}

type identityRef struct {
	DisplayName string `json:"displayName"`
	UniqueName  string `json:"uniqueName"`
}
