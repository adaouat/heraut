//go:build unix

package gitlab_test

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/gitlab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Opening a FIFO for reading blocks until a writer appears, and by then the tag is pushed, so a FIFO
// among the assets must fail the release at once instead of hanging it.
func TestCreateRelease_Assets_AFIFOIsRefusedWithoutBlocking(t *testing.T) {
	notInCI(t)
	st := newStub(t)
	tmp := t.TempDir()
	t.Chdir(tmp)
	require.NoError(t, syscall.Mkfifo(filepath.Join(tmp, "pipe"), 0o600))
	t.Setenv("GITLAB_TOKEN", "tok")
	p := gitlab.New(&config.Platform{Project: "grp/repo", BaseURL: st.srv.URL, Assets: []string{"pipe"}})

	done := make(chan error, 1)
	go func() { done <- p.CreateRelease(t.Context(), "v1.0.0", "notes", false) }()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a regular file")
		assert.Empty(t, st.requests)
	case <-time.After(5 * time.Second):
		t.Fatal("CreateRelease blocked on a FIFO")
	}
}
