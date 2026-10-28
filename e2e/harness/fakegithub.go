package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// GitHub is a loopback stand-in for the GitHub REST API, laid out as a GitHub Enterprise Server
// without subdomain isolation: the API under /api/v3 and uploads under /api/uploads, on one origin.
// It answers 200 to reads (an empty array for a releases list), 201 to release creation and uploads
// (with the uploaded name rewritten as GitHub does: "#" becomes "."), 200 to the publish PATCH and
// 204 to a delete. Every request is logged as "github [METHOD] [/path]" in the call log both
// stand-ins share, so a test can assert on the order of calls across forges.
type GitHub struct {
	URL string

	repo         *Repo
	mu           sync.Mutex
	failReleases bool
	failUploads  bool
	hangUploads  chan struct{} // closed when the first hung upload arrives; nil when uploads answer
	hangOnce     sync.Once
	release      map[string]any
	uploads      []string
}

// FakeGitHub starts the stand-in for this repository (once; later calls return the same one) and
// shuts it down with the test. Point a forge's base_url at URL. The call log exists from here
// on.
func (r *Repo) FakeGitHub() *GitHub {
	r.t.Helper()
	if r.github != nil {
		return r.github
	}
	if r.logDir == "" {
		r.logDir = r.t.TempDir()
	}
	g := &GitHub{repo: r}
	srv := httptest.NewServer(http.HandlerFunc(g.serve))
	r.t.Cleanup(srv.Close)
	g.URL = srv.URL
	r.github = g
	return g
}

// FailReleases makes every following release creation answer 403 (the call is still logged).
func (g *GitHub) FailReleases() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failReleases = true
}

// FailUploads makes every following asset upload answer 422, as GitHub does for a duplicate name
// (the call is still logged).
func (g *GitHub) FailUploads() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failUploads = true
}

// HangUploads makes every following asset upload wait without answering until the client gives up
// (the call is still logged, and the upload is not accepted). The returned channel is closed when
// the first such upload arrives, so a test can act while the client waits.
func (g *GitHub) HangUploads() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.hangUploads = make(chan struct{})
	return g.hangUploads
}

// Release is the JSON body of the last accepted release creation (nil if none).
func (g *GitHub) Release() map[string]any {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.release
}

// Uploads lists the asset names of the accepted uploads, as sent, oldest first.
func (g *GitHub) Uploads() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.uploads...)
}

func (g *GitHub) serve(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	g.log(req.Method, req.URL.Path)

	isRelease := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/releases")
	isUpload := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/assets")
	g.mu.Lock()
	refuseRelease, refuseUpload := isRelease && g.failReleases, isUpload && g.failUploads
	hang := g.hangUploads
	g.mu.Unlock()

	if isUpload && hang != nil {
		g.hangOnce.Do(func() { close(hang) })
		<-req.Context().Done()
		return
	}

	w.Header().Set("Content-Type", "application/json")
	switch {
	case refuseRelease:
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Resource not accessible by integration"}`))
	case refuseUpload:
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Validation Failed","errors":[{"resource":"ReleaseAsset","code":"already_exists","field":"name"}]}`))
	case isRelease:
		var m map[string]any
		if json.Unmarshal(body, &m) == nil {
			g.mu.Lock()
			g.release = m
			g.mu.Unlock()
		}
		uploads := g.URL + "/api/uploads" + strings.TrimPrefix(req.URL.Path, "/api/v3") + "/1/assets{?name,label}"
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "upload_url": uploads})
	case isUpload:
		name := req.URL.Query().Get("name")
		g.mu.Lock()
		g.uploads = append(g.uploads, name)
		g.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"name": strings.ReplaceAll(name, "#", ".")})
	case req.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/releases"):
		_, _ = w.Write([]byte(`[]`))
	default:
		_, _ = w.Write([]byte(`{"id":1}`))
	}
}

func (g *GitHub) log(method, path string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(g.repo.logDir, "calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintf(f, "github [%s] [%s]\n", method, path)
}
