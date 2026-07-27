package azure_test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/forge/azure"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForge_Links(t *testing.T) {
	f := azure.New(port.ForgeIdentity{
		Type: "azure_devops", Host: "https://dev.azure.example.com",
		Project: "myorg/myproject", Repository: "myrepo",
	}, nil)
	assert.Equal(t, "azure_devops", f.Type())
	assert.Equal(t, "https://dev.azure.example.com/myorg/myproject/_git/myrepo/commit/deadbeef", f.CommitURL("deadbeef"))
	assert.Equal(t, "https://dev.azure.example.com/myorg/myproject/_git/myrepo/pullrequest/42", f.ChangeURL(42))
}

func TestEnrich_MapsPRsAndLocalAuthors(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"abc123":[{
			"pullRequestId":42,"title":"Add widget",
			"createdBy":{"displayName":"Alice","uniqueName":"alice@example.com"},
			"labels":[{"name":"feature"}],
			"creationDate":"2026-07-01T10:00:00Z","closedDate":"2026-07-02T11:00:00Z",
			"closedBy":{"displayName":"Bob","uniqueName":"bob@example.com"},
			"reviewers":[{"displayName":"Carol","uniqueName":"carol@example.com","vote":10}]
		}]}]}`))
	}))
	defer srv.Close()

	f := azure.New(port.ForgeIdentity{
		Type: "azure_devops", Host: srv.URL, Project: "myorg/myproject", Repository: "myrepo",
		Token: "pat", TokenKind: port.TokenPrivate,
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice", Email: "alice@example.com", Date: time.Now()}})
	require.NoError(t, err)

	assert.Contains(t, gotPath, "/myorg/myproject/_apis/git/repositories/myrepo/pullrequestquery")
	assert.True(t, strings.HasPrefix(gotAuth, "Basic "), "Azure authenticates with HTTP Basic")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(":pat"))
	assert.Equal(t, want, gotAuth, "the PAT is the password with an empty username")

	pr := en.PRs["abc123"]
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "!", pr.RefPrefix)
	assert.Equal(t, "Add widget", pr.Title)
	assert.Equal(t, "alice", pr.AuthorLogin, "email local-part is the handle")
	assert.Equal(t, []string{"feature"}, pr.Labels)
	assert.Equal(t, "bob", pr.MergedBy.Username)
	require.Len(t, pr.Approvers, 1)
	assert.Equal(t, "carol", pr.Approvers[0].Username, "only vote >= 10 counts as approval")

	// Azure exposes no linked handle: `by @` comes from the local git author.
	assert.Equal(t, "alice", en.Authors["abc123"])
}

func TestEnrich_ErrorsAndEmpty(t *testing.T) {
	t.Run("non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		f := azure.New(port.ForgeIdentity{Host: srv.URL, Project: "myorg/myproject", Repository: "myrepo"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("empty project is an error", func(t *testing.T) {
		f := azure.New(port.ForgeIdentity{Host: "https://dev.azure.com", Project: "", Repository: "myrepo"}, nil)
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Project")
	})

	// I1: an azure identity with an empty Repository (e.g. an explicit forges: entry with no
	// repository: and no CI to fill it from) must fail naming the missing field, not issue a
	// malformed "repositories//pullrequestquery" request.
	t.Run("empty repository is an error", func(t *testing.T) {
		f := azure.New(port.ForgeIdentity{Host: "https://dev.azure.com", Project: "myorg/myproject", Repository: ""}, nil)
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Repository")
	})

	// Legacy-CI-shaped Project (team project only, no organization prefix — the org lives in the
	// Host subdomain instead, see TestEndpoint_NoDuplicateOrganization) must NOT be rejected: it is
	// a valid identity shape now that the org no longer needs to live inside Project.
	t.Run("project without organization is valid", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{}]}`))
		}))
		defer srv.Close()
		f := azure.New(port.ForgeIdentity{Host: srv.URL, Project: "myproject", Repository: "myrepo"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.NoError(t, err)
	})

	t.Run("no commits", func(t *testing.T) {
		f := azure.New(port.ForgeIdentity{Host: "https://dev.azure.example.com", Project: "myorg/myproject", Repository: "myrepo"}, nil)
		en, err := f.Enrich(nil)
		require.NoError(t, err)
		assert.Empty(t, en.PRs)
	})
}

// C2 regression: identities produced by forge.Resolve from Azure Pipelines' ambient CI
// environment must compose exactly one occurrence of the organization across the API endpoint and
// web links, for both CI forms — modern (org as a dev.azure.com path segment) and legacy (org as
// the visualstudio.com subdomain). A hand-built identity would hide this: it was Resolve's
// CI-derived Host+Project pair that duplicated the org (C2). The identity is driven from
// forge.Resolve with an injected getenv (not hand-built), so the resolver's own Host/Project
// composition is what is under test.
func TestEndpoint_NoDuplicateOrganization(t *testing.T) {
	tests := []struct {
		name          string
		collectionURI string
		wantEndpoint  string
		wantWebBase   string
	}{
		{
			name:          "modern dev.azure.com",
			collectionURI: "https://dev.azure.com/myorg/",
			wantEndpoint:  "https://dev.azure.com/myorg/myproject/_apis/git/repositories/myrepo/pullrequestquery?api-version=7.1",
			wantWebBase:   "https://dev.azure.com/myorg/myproject/_git/myrepo",
		},
		{
			name:          "legacy visualstudio.com",
			collectionURI: "https://myorg.visualstudio.com/",
			wantEndpoint:  "https://myorg.visualstudio.com/myproject/_apis/git/repositories/myrepo/pullrequestquery?api-version=7.1",
			wantWebBase:   "https://myorg.visualstudio.com/myproject/_git/myrepo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := forge.Resolve(&config.Config{}, func(k string) string {
				switch k {
				case "TF_BUILD":
					return "true"
				case "SYSTEM_COLLECTIONURI":
					return tc.collectionURI
				case "SYSTEM_TEAMPROJECT":
					return "myproject"
				case "BUILD_REPOSITORY_NAME":
					return "myrepo"
				default:
					return ""
				}
			}, "")
			require.NoError(t, err)
			require.Len(t, resolved.Forges, 1)
			id := resolved.Forges[0]

			// Web link: exercised against the real (unmodified) resolved Host — no test server needed.
			assert.Equal(t, tc.wantWebBase+"/commit/deadbeef", azure.New(id, nil).CommitURL("deadbeef"))

			// API endpoint: recorded via a capturing http.RoundTripper, so the resolved Host (with its
			// org path segment, when present) drives the real request URL instead of being swapped
			// out for a test server's origin.
			var gotURL string
			f := azure.New(id, &http.Client{Transport: capturingTransport{onRequest: func(r *http.Request) (*http.Response, error) {
				gotURL = r.URL.String()
				return &http.Response{
					StatusCode: 200,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"results":[{}]}`)),
				}, nil
			}}})
			_, err = f.Enrich([]port.Commit{{Hash: "abc123"}})
			require.NoError(t, err)
			assert.Equal(t, tc.wantEndpoint, gotURL)
		})
	}
}

// capturingTransport is a minimal http.RoundTripper stand-in so TestEndpoint_NoDuplicateOrganization
// can observe the exact outgoing request URL without a real network call or an httptest server (which
// would force substituting a fake origin for the resolved Host under test).
type capturingTransport struct {
	onRequest func(*http.Request) (*http.Response, error)
}

func (c capturingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return c.onRequest(r) }

// I2: pins the exact HTTP method, api-version query parameter, Content-Type, and decoded request
// body Azure's pullrequestquery call sends — the only protection against a malformed request shape
// silently passing (httptest accepts any method/path/body).
func TestEnrich_RequestShape(t *testing.T) {
	var gotMethod, gotQuery, gotContentType string
	var gotBody prQueryWire
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotQuery = r.URL.RawQuery
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{}]}`))
	}))
	defer srv.Close()

	f := azure.New(port.ForgeIdentity{Host: srv.URL, Project: "myorg/myproject", Repository: "myrepo"}, srv.Client())
	_, err := f.Enrich([]port.Commit{{Hash: "abc123"}, {Hash: "def456"}})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "api-version=7.1", gotQuery)
	assert.Equal(t, "application/json", gotContentType)
	require.Len(t, gotBody.Queries, 1)
	assert.Equal(t, "lastMergeCommit", gotBody.Queries[0].Type)
	assert.ElementsMatch(t, []string{"abc123", "def456"}, gotBody.Queries[0].Items)
}

type prQueryWire struct {
	Queries []struct {
		Type  string   `json:"type"`
		Items []string `json:"items"`
	} `json:"queries"`
}
