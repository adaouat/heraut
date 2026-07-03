package native

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

func azureLC(baseURL string) *port.LinkContext {
	return &port.LinkContext{Platform: "azure_devops", BaseURL: baseURL, Owner: "myorg/myproj", Repo: "myrepo", Token: "tok"}
}

// azurePRQueryBody is a canned pullrequestquery response mapping one commit to one PR.
func azurePRQueryBody(sha string, prID int, display, unique string) string {
	return `{"results":[{"` + sha + `":[{"pullRequestId":` + strconv.Itoa(prID) +
		`,"title":"feat: x","createdBy":{"displayName":"` + display + `","uniqueName":"` + unique + `"}}]}]}`
}

// TestEnrichAzure_MapsPR asserts the request contract (method, path, api-version, auth, body)
// and the mapped PullRequest.
func TestEnrichAzure_MapsPR(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotAuth, gotCType string
	var gotBody azurePRQuery
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		gotAuth, gotCType = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = io.WriteString(w, azurePRQueryBody("abc123", 42, "Jane Doe", "jane@corp.com"))
	}))
	defer srv.Close()

	result, err := enrichAzure(srv.Client(), azureLC(srv.URL), []string{"abc123"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, PullRequest{
		Number:      42,
		URL:         srv.URL + "/myorg/myproj/_git/myrepo/pullrequest/42",
		AuthorLogin: "jane",
		RefPrefix:   "!",
	}, result["abc123"])

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/myorg/myproj/_apis/git/repositories/myrepo/pullrequestquery", gotPath)
	assert.Equal(t, "api-version=7.1", gotQuery)
	assert.Equal(t, "application/json", gotCType)
	assert.Equal(t, "Basic OnRvaw==", gotAuth) // base64(":tok")
	require.Len(t, gotBody.Queries, 1)
	assert.Equal(t, "lastMergeCommit", gotBody.Queries[0].Type)
	assert.Equal(t, []string{"abc123"}, gotBody.Queries[0].Items)
}

// TestEnrichAzure_NoPR_Absent: a commit with no associated PR is absent from the map.
func TestEnrichAzure_NoPR_Absent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"results":[{}]}`)
	}))
	defer srv.Close()

	result, err := enrichAzure(srv.Client(), azureLC(srv.URL), []string{"abc123"})
	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestEnrichAzure_AuthorFallbackDisplayName: no uniqueName → fall back to displayName.
func TestEnrichAzure_AuthorFallbackDisplayName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, azurePRQueryBody("abc123", 7, "Solo Name", ""))
	}))
	defer srv.Close()

	result, err := enrichAzure(srv.Client(), azureLC(srv.URL), []string{"abc123"})
	require.NoError(t, err)
	assert.Equal(t, "Solo Name", result["abc123"].AuthorLogin)
}

// TestEnrichAzure_ErrorStatus: a non-2xx response is a wrapped error (policy decides).
func TestEnrichAzure_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := enrichAzure(srv.Client(), azureLC(srv.URL), []string{"abc123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pullrequestquery")
}

// TestEnrichAzure_MalformedJSON: a 200 with garbage body errors.
func TestEnrichAzure_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "not json")
	}))
	defer srv.Close()

	_, err := enrichAzure(srv.Client(), azureLC(srv.URL), []string{"abc123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pullrequestquery")
}

// TestEnrichAzure_NoShas_NoCall: an empty commit set makes no HTTP call.
func TestEnrichAzure_NoShas_NoCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()

	result, err := enrichAzure(srv.Client(), azureLC(srv.URL), nil)
	require.NoError(t, err)
	assert.Empty(t, result)
	assert.False(t, called, "no commits → no HTTP call")
}

// End-to-end: Azure release-notes enrichment renders "by @author in [!N]".
func TestGenerate_Enrich_Azure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, azurePRQueryBody("abc1234567", 42, "Jane Doe", "jane@corp.com"))
	}))
	defer srv.Close()

	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)                                                                        // previousTag
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil)                                                          // tagDate
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: x", ""), "", nil) // collectCommits
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", azureLC(srv.URL))
	require.NoError(t, err)
	assert.Contains(t, out, "by @jane in [!42]("+srv.URL+"/myorg/myproj/_git/myrepo/pullrequest/42)")
	assert.False(t, g.Degraded())
}
