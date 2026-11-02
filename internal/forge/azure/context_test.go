package azure_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/forge/azure"
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
	ctx, cancel := context.WithCancel(t.Context())
	srv := stalledServer(t, cancel)
	f := azure.New(port.ForgeIdentity{
		Type: "azure_devops", Host: srv.URL, Project: "myorg/myproject", Repository: "myrepo",
		Token: "pat", TokenKind: port.TokenPrivate,
	}, srv.Client())

	require.ErrorIs(t, enrichWithin(t, ctx, f), context.Canceled)
}
