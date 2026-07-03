package native

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// azureAPIVersion pins the Azure DevOps REST API version for pullrequestquery (ADR-0035).
const azureAPIVersion = "7.1"

// enrichAzure resolves the pull request associated with each commit SHA via one batched
// Azure DevOps `pullrequestquery` POST (ADR-0035): a single request for the whole release,
// correlating `results[0][sha] → PR`. Commits with no PR are absent from the map. First-time
// contributor status is not derived (Azure PRs carry no authorAssociation), so FirstTimer stays
// false and no New Contributors block is produced. Auth is a PAT (LinkContext.Token) via HTTP
// Basic; any transport error or non-2xx status is wrapped so enrichForRelease applies the
// remote_metadata policy.
func enrichAzure(client *http.Client, lc *port.LinkContext, shas []string) (map[string]PullRequest, error) {
	result := make(map[string]PullRequest)
	if len(shas) == 0 {
		return result, nil
	}

	org, project, ok := splitAzureOwner(lc.Owner)
	if !ok {
		return nil, fmt.Errorf("azure pullrequestquery: LinkContext.Owner %q is not organization/project", lc.Owner)
	}
	endpoint := fmt.Sprintf("%s/%s/%s/_apis/git/repositories/%s/pullrequestquery?api-version=%s",
		strings.TrimRight(lc.BaseURL, "/"),
		url.PathEscape(org), url.PathEscape(project), url.PathEscape(lc.Repo), azureAPIVersion)

	body, err := json.Marshal(azurePRQuery{Queries: []azurePRQueryInput{{Type: "lastMergeCommit", Items: shas}}})
	if err != nil {
		return nil, fmt.Errorf("azure pullrequestquery: encoding request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("azure pullrequestquery: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if lc.Token != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+lc.Token)))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure pullrequestquery: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure pullrequestquery: unexpected status %s", resp.Status)
	}

	var out azurePRQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("azure pullrequestquery: decoding response: %w", err)
	}
	if len(out.Results) == 0 {
		return result, nil
	}

	prWebBase := azureRepoRoot(lc) + "/pullrequest/"
	for sha, prs := range out.Results[0] {
		if len(prs) == 0 {
			continue
		}
		pr := prs[0] // first association wins, matching the GitHub/GitLab drivers
		result[sha] = PullRequest{
			Number:      pr.PullRequestID,
			URL:         prWebBase + strconv.Itoa(pr.PullRequestID),
			AuthorLogin: azureAuthorLogin(pr.CreatedBy),
			RefPrefix:   "!",
		}
	}
	return result, nil
}

// splitAzureOwner splits an Azure "organization/project" owner (ADR-0026) into its two segments.
func splitAzureOwner(owner string) (org, project string, ok bool) {
	org, project, ok = strings.Cut(owner, "/")
	if !ok || org == "" || project == "" {
		return "", "", false
	}
	return org, project, true
}

// azureAuthorLogin renders an "@handle"-friendly author from an Azure identity: the local-part of
// uniqueName (usually an email) when present, else the full displayName. Azure has no login handle.
func azureAuthorLogin(id azureIdentityRef) string {
	if id.UniqueName != "" {
		if local, _, ok := strings.Cut(id.UniqueName, "@"); ok && local != "" {
			return local
		}
		return id.UniqueName
	}
	return id.DisplayName
}

type azurePRQuery struct {
	Queries []azurePRQueryInput `json:"queries"`
}

type azurePRQueryInput struct {
	Type  string   `json:"type"`
	Items []string `json:"items"`
}

type azurePRQueryResult struct {
	Results []map[string][]azurePR `json:"results"`
}

type azurePR struct {
	PullRequestID int              `json:"pullRequestId"`
	Title         string           `json:"title"`
	CreatedBy     azureIdentityRef `json:"createdBy"`
}

type azureIdentityRef struct {
	DisplayName string `json:"displayName"`
	UniqueName  string `json:"uniqueName"`
}
