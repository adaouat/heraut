package github

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/platforms"
)

const (
	createKey = "POST api.github.com" + releasesPath
	listKey   = "GET api.github.com" + releasesPath
	patchKey  = "PATCH api.github.com" + releasesPath + "/" + draftID
)

// noRetryWait makes the retries of this test immediate.
func noRetryWait(t *testing.T) {
	t.Helper()
	prev := retryDelays
	retryDelays = []time.Duration{0, 0}
	t.Cleanup(func() { retryDelays = prev })
}

// sequence answers the n-th request with handlers[n], and every later one with the last handler.
func sequence(handlers ...http.HandlerFunc) http.HandlerFunc {
	var mu sync.Mutex
	n := 0
	return func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		h := handlers[min(n, len(handlers)-1)]
		n++
		mu.Unlock()
		h(w, r)
	}
}

func listing(releases ...map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, append([]map[string]any{}, releases...))
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
	serverError   = failWith(http.StatusBadGateway, `{"message":"Bad Gateway"}`)
	alreadyExists = failWith(http.StatusUnprocessableEntity,
		`{"message":"Validation Failed","errors":[{"resource":"Release","code":"already_exists","field":"tag_name"}]}`)
	publishedV123 = map[string]any{"id": 42, "tag_name": "v1.2.3", "draft": false}
	created       = func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{"id": 42, "tag_name": "v1.2.3", "upload_url": githubUploadURL})
	}
)

func TestCreateRelease_RetriesATransientError(t *testing.T) {
	for name, first := range map[string]http.HandlerFunc{"server error": serverError, "dropped connection": dropConnection} {
		t.Run(name, func(t *testing.T) {
			noRetryWait(t)
			f := newFakeGitHub(t, githubUploadURL)
			f.override[createKey] = sequence(first, created)

			_, err := githubClient(t, f).CreateRelease(t.Context(), "v1.2.3", "", "n", false, false, nil)

			require.NoError(t, err)
			assert.Equal(t, []string{createKey, listKey, createKey}, f.requests())
		})
	}
}

// The create went through but its answer was lost: the release is found by its tag, not created twice.
func TestCreateRelease_FindsTheReleaseALostAnswerCreated(t *testing.T) {
	noRetryWait(t)
	f := newFakeGitHub(t, githubUploadURL)
	f.override[createKey] = serverError
	f.override[listKey] = listing(map[string]any{"id": 7, "tag_name": "v1.0.0"}, publishedV123)

	_, err := githubClient(t, f).CreateRelease(t.Context(), "v1.2.3", "", "n", false, false, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{createKey, listKey}, f.requests())
}

// A retry answered "already exists" means the earlier attempt created the release after all.
func TestCreateRelease_AlreadyExistsOnARetryIsTheEarlierCreate(t *testing.T) {
	noRetryWait(t)
	f := newFakeGitHub(t, githubUploadURL)
	f.override[createKey] = sequence(serverError, alreadyExists)
	f.override[listKey] = sequence(listing(), listing(publishedV123))

	_, err := githubClient(t, f).CreateRelease(t.Context(), "v1.2.3", "", "n", false, false, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{createKey, listKey, createKey, listKey}, f.requests())
}

func TestCreateRelease_GivesUpAfterThreeTransientErrors(t *testing.T) {
	noRetryWait(t)
	f := newFakeGitHub(t, githubUploadURL)
	f.override[createKey] = serverError

	_, err := githubClient(t, f).CreateRelease(t.Context(), "v1.2.3", "", "n", false, false, nil)

	require.Error(t, err)
	assert.Equal(t, []string{createKey, listKey, createKey, listKey, createKey, listKey}, f.requests())
	assert.Contains(t, err.Error(), "creating release v1.2.3")
	assert.Contains(t, err.Error(), "502 Bad Gateway")
	assertReadable(t, err)
}

// A refusal is final: no retry, and no lookup that could take someone else's release for ours.
func TestCreateRelease_ARefusalIsNeitherRetriedNorLookedUp(t *testing.T) {
	for name, refusal := range map[string]http.HandlerFunc{
		"bad credentials": failWith(http.StatusUnauthorized, `{"message":"Bad credentials"}`),
		"already exists":  alreadyExists,
	} {
		t.Run(name, func(t *testing.T) {
			noRetryWait(t)
			f := newFakeGitHub(t, githubUploadURL)
			f.override[createKey] = refusal
			f.override[listKey] = listing(publishedV123)

			_, err := githubClient(t, f).CreateRelease(t.Context(), "v1.2.3", "", "n", false, false, nil)

			require.Error(t, err)
			assert.Equal(t, []string{createKey}, f.requests())
		})
	}
}

// With assets the release starts as a draft: a lost answer is found among the drafts and the run goes
// on with it, instead of leaving a second draft behind.
func TestCreateRelease_FindsTheDraftALostAnswerCreated(t *testing.T) {
	noRetryWait(t)
	f := newFakeGitHub(t, githubUploadURL)
	f.override[createKey] = serverError
	f.override[listKey] = listing(
		map[string]any{"id": 7, "tag_name": "v1.2.3", "draft": false},
		map[string]any{"id": 42, "tag_name": "v1.2.3", "draft": true, "upload_url": githubUploadURL},
	)

	names, err := githubClient(t, f).CreateRelease(t.Context(), "v1.2.3", "", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})

	require.NoError(t, err)
	assert.Equal(t, []string{"app.zip"}, names)
	assert.Equal(t, []string{createKey, listKey, uploadKey, patchKey}, f.requests())
}

func TestCreateRelease_RetriesThePublish(t *testing.T) {
	noRetryWait(t)
	f := newFakeGitHub(t, githubUploadURL)
	f.override[patchKey] = sequence(serverError, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"id": 42, "draft": false})
	})

	_, err := githubClient(t, f).CreateRelease(t.Context(), "v1.2.3", "", "n", false, false, []platforms.Asset{testAsset(t, "app.zip", "x")})

	require.NoError(t, err)
	assert.Equal(t, []string{createKey, uploadKey, patchKey, patchKey}, f.requests())
}
