package gitlab

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	createPath  = "POST /api/v4/projects/acme/widget/releases"
	releasePath = "GET /api/v4/projects/acme/widget/releases/v1.0.0"
)

// noRetryWait makes this test's retries immediate, and client-go retries a 5xx once.
func noRetryWait(t *testing.T) {
	t.Helper()
	prevDelays, prevRetries := retryDelays, serverErrorRetries
	retryDelays, serverErrorRetries = []time.Duration{0, 0}, 1
	t.Cleanup(func() { retryDelays, serverErrorRetries = prevDelays, prevRetries })
}

// scriptedGitLab answers each "METHOD path" with the next handler of its script, the last one
// repeating, and records the requests in order.
type scriptedGitLab struct {
	srv *httptest.Server

	mu       sync.Mutex
	scripts  map[string][]http.HandlerFunc
	served   map[string]int
	requests []string
}

func newScriptedGitLab(t *testing.T, scripts map[string][]http.HandlerFunc) *scriptedGitLab {
	t.Helper()
	s := &scriptedGitLab{scripts: scripts, served: map[string]int{}}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		s.mu.Lock()
		s.requests = append(s.requests, key)
		script := s.scripts[key]
		n := s.served[key]
		s.served[key]++
		s.mu.Unlock()
		if len(script) == 0 {
			answer(http.StatusNotFound, `{"message":"404 Not Found"}`)(w, r)
			return
		}
		script[min(n, len(script)-1)](w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *scriptedGitLab) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func (s *scriptedGitLab) client(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(s.srv.URL, "tok", false)
	require.NoError(t, err)
	return c
}

func answer(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// dropConnection closes the connection without an answer, as a proxy that times out does.
func dropConnection(w http.ResponseWriter, _ *http.Request) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err == nil {
		_ = conn.Close()
	}
}

var (
	created       = answer(http.StatusCreated, `{"tag_name":"v1.0.0","description":"the notes"}`)
	ourRelease    = answer(http.StatusOK, `{"tag_name":"v1.0.0","description":"the notes"}`)
	otherRelease  = answer(http.StatusOK, `{"tag_name":"v1.0.0","description":"written by hand"}`)
	alreadyExists = answer(http.StatusConflict, `{"message":"Release already exists"}`)
	badGateway    = answer(http.StatusBadGateway, `{"message":"502 Bad Gateway"}`)
)

func countOf(requests []string, key string) int {
	n := 0
	for _, r := range requests {
		if r == key {
			n++
		}
	}
	return n
}

func TestCreateRelease_RetriesALostAnswer(t *testing.T) {
	noRetryWait(t)
	s := newScriptedGitLab(t, map[string][]http.HandlerFunc{createPath: {dropConnection, created}})

	err := s.client(t).CreateRelease(t.Context(), "acme/widget", "v1.0.0", "the notes", nil)

	require.NoError(t, err)
	assert.Equal(t, []string{createPath, releasePath, createPath}, s.recorded())
}

// The create went through but its answer was lost: the release is found by its tag, not created twice.
func TestCreateRelease_FindsTheReleaseALostAnswerCreated(t *testing.T) {
	noRetryWait(t)
	s := newScriptedGitLab(t, map[string][]http.HandlerFunc{createPath: {dropConnection}, releasePath: {ourRelease}})

	err := s.client(t).CreateRelease(t.Context(), "acme/widget", "v1.0.0", "the notes", nil)

	require.NoError(t, err)
	assert.Equal(t, []string{createPath, releasePath}, s.recorded())
}

// client-go retries a 5xx on its own, so "already exists" can answer heraut's first call: the
// release is this run's when it carries this run's notes.
func TestCreateRelease_AlreadyExistsWithTheseNotesIsThisRunsRelease(t *testing.T) {
	noRetryWait(t)
	s := newScriptedGitLab(t, map[string][]http.HandlerFunc{createPath: {alreadyExists}, releasePath: {ourRelease}})

	err := s.client(t).CreateRelease(t.Context(), "acme/widget", "v1.0.0", "the notes", nil)

	require.NoError(t, err)
	assert.Equal(t, []string{createPath, releasePath}, s.recorded())
}

func TestCreateRelease_AlreadyExistsWithOtherNotesFails(t *testing.T) {
	noRetryWait(t)
	s := newScriptedGitLab(t, map[string][]http.HandlerFunc{createPath: {alreadyExists}, releasePath: {otherRelease}})

	err := s.client(t).CreateRelease(t.Context(), "acme/widget", "v1.0.0", "the notes", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating release v1.0.0")
	assert.Contains(t, err.Error(), "409")
	assert.Equal(t, []string{createPath, releasePath}, s.recorded())
}

func TestCreateRelease_ARefusalIsNotLookedUp(t *testing.T) {
	noRetryWait(t)
	s := newScriptedGitLab(t, map[string][]http.HandlerFunc{
		createPath:  {answer(http.StatusForbidden, `{"message":"403 Forbidden"}`)},
		releasePath: {ourRelease},
	})

	err := s.client(t).CreateRelease(t.Context(), "acme/widget", "v1.0.0", "the notes", nil)

	require.Error(t, err)
	assert.Equal(t, []string{createPath}, s.recorded())
}

// client-go has already retried the 5xx: heraut looks the release up once and does not retry on top.
func TestCreateRelease_AServerErrorIsLookedUpNotRetriedAgain(t *testing.T) {
	noRetryWait(t)
	for name, tc := range map[string]struct {
		lookup  http.HandlerFunc
		wantErr bool
	}{
		"the create went through": {lookup: ourRelease},
		"it did not":              {lookup: answer(http.StatusNotFound, `{"message":"404 Not Found"}`), wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			s := newScriptedGitLab(t, map[string][]http.HandlerFunc{createPath: {badGateway}, releasePath: {tc.lookup}})

			err := s.client(t).CreateRelease(t.Context(), "acme/widget", "v1.0.0", "the notes", nil)

			got := s.recorded()
			assert.Equal(t, releasePath, got[len(got)-1], "one lookup, after client-go's own retries")
			assert.Equal(t, 1, countOf(got, releasePath))
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, strings.Contains(err.Error(), "502"), err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCreateRelease_GivesUpAfterThreeLostAnswers(t *testing.T) {
	noRetryWait(t)
	s := newScriptedGitLab(t, map[string][]http.HandlerFunc{createPath: {dropConnection}})

	err := s.client(t).CreateRelease(t.Context(), "acme/widget", "v1.0.0", "the notes", nil)

	require.Error(t, err)
	assert.Equal(t, []string{createPath, releasePath, createPath, releasePath, createPath, releasePath}, s.recorded())
}
