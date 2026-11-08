package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
)

type runKey struct{}

// recordingForge records the context Enrich was called with.
type recordingForge struct{ ctx context.Context }

func (f *recordingForge) Type() string                     { return "github" }
func (f *recordingForge) Identity() port.ForgeIdentity     { return port.ForgeIdentity{Type: "github"} }
func (f *recordingForge) CommitURL(string) string          { return "" }
func (f *recordingForge) ChangeURL(int) string             { return "" }
func (f *recordingForge) CompareURL(string, string) string { return "" }
func (f *recordingForge) Enrich(ctx context.Context, _ []port.Commit, _ string) (port.Enrichment, error) {
	f.ctx = ctx
	return port.Enrichment{}, nil
}

// A rotating changelog builds its generator on each run: the command's context still reaches the
// forge through it.
func TestRotatingGenerator_PassesItsContextToTheForge(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // latestMatchingTag: no prior tags
	mr.QueueResponse("", "", nil) // listTags: none
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com", "2026-05-01T00:00:00Z", "feat: initial", ""), "", nil)
	mr.QueueResponse("aaa1111111\n", "", nil) // git rev-parse HEAD (enrichment ref)

	forge := &recordingForge{}
	driver := &config.ContentDriver{Output: filepath.Join(t.TempDir(), "CHANGELOG_{YYYY}.md")}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	gen := buildGenerator(mr, driver, native.ModeChangelog, "", false, forge, "", nil)
	wrapped := wrapWithRotation(gen, mr, cfg, driver, "", false, forge, "", nil)

	ctx := context.WithValue(t.Context(), runKey{}, "this run")
	_, err := wrapped.Generate(ctx, "2026.05.0", nil)
	require.NoError(t, err)

	require.NotNil(t, forge.ctx)
	assert.Equal(t, "this run", forge.ctx.Value(runKey{}))
}

// Ctrl-C during heraut notes stops a pull-request lookup that stalls, even under the default
// optional policy, which would otherwise print the notes without their PR links.
func TestGenerateNotes_StopsWhenTheContextIsCancelled(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("GH_TOKEN", "test-token")
	commit, tag, _ := standaloneNotesRepo(t)
	commit("feat: add login")
	tag("v1.0.0")

	ctx, cancel := context.WithCancel(t.Context())
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body) // the server notices the client leaving only once the body is read
		cancel()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	cfg := standaloneNotesCfg()
	cfg.Commits = nil
	cfg.Forges = []config.Forge{{Name: "gh", Type: "github", Repository: "acme/widget", BaseURL: srv.URL}}

	done := make(chan error, 1)
	go func() {
		_, err := GenerateNotes(ctx, execadapter.New(false, false), cfg, NotesOpts{Tag: "v1.0.0"})
		done <- err
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("heraut notes did not return")
	}
}

// failingForge's Enrich fails, as an unreachable forge does.
type failingForge struct{ recordingForge }

func (f *failingForge) Enrich(context.Context, []port.Commit, string) (port.Enrichment, error) {
	return port.Enrichment{}, errors.New("connection refused")
}

// A rotating changelog reports a degraded enrichment like a plain one, so the step says "without
// enrichment" and the reason is shown.
func TestRotatingGenerator_ReportsADegradedEnrichment(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // latestMatchingTag: no prior tags
	mr.QueueResponse("", "", nil) // listTags: none
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com", "2026-05-01T00:00:00Z", "feat: initial", ""), "", nil)
	mr.QueueResponse("aaa1111111\n", "", nil) // git rev-parse HEAD (enrichment ref)

	forge := &failingForge{}
	driver := &config.ContentDriver{Output: filepath.Join(t.TempDir(), "CHANGELOG_{YYYY}.md")}
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	gen := buildGenerator(mr, driver, native.ModeChangelog, "", false, forge, "", nil)
	wrapped := wrapWithRotation(gen, mr, cfg, driver, "", false, forge, "", nil)

	d, ok := wrapped.(interface {
		Degraded() bool
		DegradedReason() string
	})
	require.True(t, ok, "the rotating generator reports degradation")
	assert.False(t, d.Degraded(), "nothing generated yet")

	_, err := wrapped.Generate(t.Context(), "2026.05.0", nil)
	require.NoError(t, err)

	assert.True(t, d.Degraded())
	assert.Contains(t, d.DegradedReason(), "connection refused")
}
