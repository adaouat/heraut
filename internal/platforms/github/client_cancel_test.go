package github

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/platforms"
)

const uploadPath = "uploads.github.com/repos/acme/widget/releases/42/assets"

// within fails the test when fn has not returned after a few seconds: every case below would hang
// forever without its bound.
func within(t *testing.T, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the publish did not return")
		return nil
	}
}

func setResponseHeaderTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	prev := responseHeaderTimeout
	responseHeaderTimeout = d
	t.Cleanup(func() { responseHeaderTimeout = prev })
}

// A server that accepts the request and never answers must not hang the release after the tag push.
// The bound is on the response headers, not on the whole exchange, so a long upload still completes.
func TestCreateRelease_AServerThatNeverAnswersTimesOut(t *testing.T) {
	setResponseHeaderTimeout(t, 100*time.Millisecond)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body) // the server notices the client leaving only once the body is read
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, testRepository, testToken, noEnv)
	require.NoError(t, err)

	err = within(t, func() error {
		_, err := c.CreateRelease(t.Context(), "v1.2.3", "n", false, false, nil)
		return err
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout awaiting response headers")
}

// Ctrl-C during an upload stops it, and the draft is still deleted, on a context of its own since the
// run's context is already cancelled.
func TestCreateRelease_CancelledDuringAnUploadDeletesTheDraft(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	ctx, cancel := context.WithCancel(t.Context())
	f.override["POST "+uploadPath] = func(_ http.ResponseWriter, r *http.Request) {
		cancel()
		<-r.Context().Done()
	}

	err := within(t, func() error {
		_, err := githubClient(t, f).CreateRelease(ctx, "v1.2.3", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
		return err
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []string{
		"POST api.github.com" + releasesPath,
		"POST " + uploadPath,
		"DELETE api.github.com" + releasesPath + "/" + draftID,
	}, f.requests())
}

// The cleanup has its own deadline, so a DELETE that never answers cannot hang the run either.
func TestCreateRelease_TheDraftCleanupIsBounded(t *testing.T) {
	prev := cleanupTimeout
	cleanupTimeout = 100 * time.Millisecond
	t.Cleanup(func() { cleanupTimeout = prev })
	f := newFakeGitHub(t, githubUploadURL)
	f.override["POST "+uploadPath] = failWith(http.StatusInternalServerError, `{"message":"boom"}`)
	f.override["DELETE api.github.com"+releasesPath+"/"+draftID] = func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}

	err := within(t, func() error {
		_, err := githubClient(t, f).CreateRelease(t.Context(), "v1.2.3", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})
		return err
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uploading asset app.zip")
	assert.Contains(t, err.Error(), "deleting the draft release 42 also failed")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestCreateRelease_ACancelledContextSendsNothing(t *testing.T) {
	f := newFakeGitHub(t, githubUploadURL)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := githubClient(t, f).CreateRelease(ctx, "v1.2.3", "n", false, false, nil)

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, f.requests())
}
