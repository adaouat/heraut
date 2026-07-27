package azure

import (
	"bytes"
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

// enrichPRs resolves the pull request associated with each commit SHA via one batched Azure
// DevOps `pullrequestquery` POST (ADR-0035): a single request for the whole release, correlating
// `results[0][sha] → PR`. Commits with no PR are absent from the map. Title and labels are
// populated best-effort from the response (labels may be empty if the API requires an expand);
// first-timer detection and the New Contributors block are computed by the shared local git
// tier, not here. Auth is a PAT (id.Token) via HTTP Basic; any transport error or non-2xx status
// is wrapped so enrichForRelease applies the remote_metadata policy.
func (f *Forge) enrichPRs(shas []string) (map[string]port.PullRequest, error) {
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
	// Project is appended as-is (never split): the organization lives in Host when Host is the
	// dev.azure.com form ("https://dev.azure.com/myorg"), or in Host's subdomain when it's the
	// legacy visualstudio.com form — either way Host already carries it exactly once, so Project
	// composes here as one opaque segment, matching webBase()'s formula (C2: composing an
	// org-splitting Project on top of an org-bearing Host duplicated the organization).
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequestquery?api-version=%s",
		strings.TrimRight(f.id.Host, "/"), f.id.Project, url.PathEscape(f.id.Repository), apiVersion)

	body, err := json.Marshal(prQuery{Queries: []prQueryInput{{Type: "lastMergeCommit", Items: shas}}})
	if err != nil {
		return nil, fmt.Errorf("azure pullrequestquery: encoding request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("azure pullrequestquery: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if f.id.Token != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+f.id.Token)))
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure pullrequestquery: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure pullrequestquery: unexpected status %s", resp.Status)
	}

	var out prQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("azure pullrequestquery: decoding response: %w", err)
	}
	if len(out.Results) == 0 {
		return result, nil
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
			if r.Vote >= 10 {
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
	return result, nil
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
