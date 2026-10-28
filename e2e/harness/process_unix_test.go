//go:build unix

package harness

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo_StartInterruptAndWait(t *testing.T) {
	r := NewRepo(t)

	p := r.Start("/bin/sh", nil, "-c", `trap 'echo got-int; exit 7' INT; echo ready; while :; do sleep 0.05; done`)
	p.WaitForOutput(t, "ready")
	p.Interrupt(t)
	res := p.Wait(t)

	assert.Equal(t, 7, res.ExitCode)
	assert.Equal(t, "ready\ngot-int\n", res.Stdout)
}

func TestRepo_FakeGitHubHangUploadsUntilTheClientLeaves(t *testing.T) {
	r := NewRepo(t)
	gh := r.FakeGitHub()
	arrived := gh.HangUploads()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, gh.URL+"/api/uploads/repos/acme/widget/releases/1/assets?name=a.txt", strings.NewReader("a"))
		if err != nil {
			done <- err
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		done <- err
	}()

	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("the upload never reached the stand-in")
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("the hung upload did not end when the client left")
	}
	assert.Equal(t, []string{"github [POST] [/api/uploads/repos/acme/widget/releases/1/assets]"}, r.APICalls())
	assert.Empty(t, gh.Uploads(), "a hung upload is not accepted")
}
