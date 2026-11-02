package gitlab_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/forge/gitlab"
	"github.com/adaouat/heraut/internal/port"
)

// stalledServer cancels the run once a request arrives and never answers it, the way Ctrl-C lands
// during a pull-request lookup that hangs.
func stalledServer(t *testing.T, cancel context.CancelFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body) // the server notices the client leaving only once the body is read
		cancel()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv
}

// enrichWithin fails the test when Enrich has not returned after a few seconds.
func enrichWithin(t *testing.T, ctx context.Context, f port.Forge) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := f.Enrich(ctx, []port.Commit{{Hash: "abc123", Author: "Alice", Date: time.Now()}}, "v1.0.0")
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Enrich did not return")
		return nil
	}
}

func TestEnrich_StopsWhenTheContextIsCancelled(t *testing.T) {
	for _, mode := range []string{"rest", "graphql"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			srv := stalledServer(t, cancel)
			f := gitlab.New(port.ForgeIdentity{
				Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "g/p",
				Token: "t", TokenKind: port.TokenPrivate, APIMode: mode,
			}, srv.Client())

			require.ErrorIs(t, enrichWithin(t, ctx, f), context.Canceled)
		})
	}
}

// GraphQL asks for the commit authors, then for the merge requests: Ctrl-C during the second query
// stops it too.
func TestEnrich_GraphQLStopsDuringTheMergeRequestQuery(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		requests++
		if requests == 1 {
			_, _ = w.Write([]byte(`{"data":{}}`)) // no author found, one page
			return
		}
		cancel()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	f := gitlab.New(port.ForgeIdentity{
		Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "g/p",
		Token: "t", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())

	require.ErrorIs(t, enrichWithin(t, ctx, f), context.Canceled)
	require.Equal(t, 2, requests)
}
