package azure_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	t.Run("project without organization is an error", func(t *testing.T) {
		f := azure.New(port.ForgeIdentity{Host: "https://dev.azure.example.com", Project: "myproject", Repository: "myrepo"}, nil)
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err, "Project must be organization/project")
	})

	t.Run("no commits", func(t *testing.T) {
		f := azure.New(port.ForgeIdentity{Host: "https://dev.azure.example.com", Project: "myorg/myproject", Repository: "myrepo"}, nil)
		en, err := f.Enrich(nil)
		require.NoError(t, err)
		assert.Empty(t, en.PRs)
	})
}
