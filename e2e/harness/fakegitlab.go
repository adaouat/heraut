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

// GitLab is a loopback stand-in for the GitLab REST API. It answers 200 to reads (an empty array
// for a releases list), 201 to uploads and release creation, and logs every request as
// "gitlab [METHOD] [/decoded/path]" in the call log the fake CLIs share, so a test can assert on
// the order of CLI and API calls together.
type GitLab struct {
	URL string

	repo         *Repo
	mu           sync.Mutex
	failReleases bool
	release      map[string]any
}

// FakeGitLab starts the stand-in for this repository (once; later calls return the same one) and
// shuts it down with the test. Point a forge's base_url at URL. The call log lives next to the
// fake CLIs, so it exists from here on.
func (r *Repo) FakeGitLab() *GitLab {
	r.t.Helper()
	if r.gitlab != nil {
		return r.gitlab
	}
	if r.binDir == "" {
		r.binDir = r.t.TempDir()
	}
	g := &GitLab{repo: r}
	srv := httptest.NewServer(http.HandlerFunc(g.serve))
	r.t.Cleanup(srv.Close)
	g.URL = srv.URL
	r.gitlab = g
	return g
}

// FailReleases makes every following release creation answer 403 (the call is still logged).
func (g *GitLab) FailReleases() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failReleases = true
}

// Release is the JSON body of the last accepted release creation (nil if none).
func (g *GitLab) Release() map[string]any {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.release
}

func (g *GitLab) serve(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	g.log(req.Method, req.URL.Path)

	isRelease := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/releases")
	g.mu.Lock()
	refuse := isRelease && g.failReleases
	g.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case refuse:
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
	case isRelease:
		var m map[string]any
		if json.Unmarshal(body, &m) == nil {
			g.mu.Lock()
			g.release = m
			g.mu.Unlock()
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/releases"):
		_, _ = w.Write([]byte(`[]`))
	case req.Method == http.MethodGet:
		_, _ = w.Write([]byte(`{}`))
	default:
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}
}

func (g *GitLab) log(method, path string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(g.repo.binDir, "calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintf(f, "gitlab [%s] [%s]\n", method, path)
}
