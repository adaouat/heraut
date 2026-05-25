package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildChecksums returns a checksums.txt body with the SHA-256 of content for binName.
func buildChecksums(binName string, content []byte) []byte {
	sum := sha256.Sum256(content)
	return []byte(hex.EncodeToString(sum[:]) + "  " + binName + "\n")
}

// newReleaseServer starts an httptest.Server that serves:
//   - GET /releases/latest  → JSON release manifest with tagName
//   - GET /download/<name>  → bytes from assets map
//
// Returns the server and its /releases/latest URL.
func newReleaseServer(t *testing.T, tagName string, assets map[string][]byte) (*httptest.Server, string) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases/latest":
			rel := Release{TagName: tagName}
			for name := range assets {
				rel.Assets = append(rel.Assets, Asset{
					Name:               name,
					BrowserDownloadURL: srv.URL + "/download/" + name,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rel)
		case strings.HasPrefix(r.URL.Path, "/download/"):
			name := strings.TrimPrefix(r.URL.Path, "/download/")
			if data, ok := assets[name]; ok {
				_, _ = w.Write(data)
			} else {
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, srv.URL + "/releases/latest"
}

// ---- Check ----------------------------------------------------------------

func TestUpdater_Check_UpToDate(t *testing.T) {
	_, latestURL := newReleaseServer(t, "v1.2.3", nil)
	u := New("v1.2.3", WithLatestURL(latestURL))

	latest, available, err := u.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", latest)
	assert.False(t, available)
}

func TestUpdater_Check_UpdateAvailable(t *testing.T) {
	_, latestURL := newReleaseServer(t, "v1.3.0", nil)
	u := New("v1.2.3", WithLatestURL(latestURL))

	latest, available, err := u.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "v1.3.0", latest)
	assert.True(t, available)
}

func TestUpdater_Check_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	u := New("v1.2.3", WithLatestURL(srv.URL+"/releases/latest"))
	_, _, err := u.Check(context.Background())
	assert.Error(t, err)
}

// ---- Hint -----------------------------------------------------------------

func TestUpdater_Hint_PrintsWhenAvailable(t *testing.T) {
	_, latestURL := newReleaseServer(t, "v1.3.0", nil)
	u := New("v1.2.3",
		WithLatestURL(latestURL),
		withCacheDir(t.TempDir()),
	)

	var buf strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u.Hint(ctx, &buf)

	assert.Contains(t, buf.String(), "hint: heraut v1.3.0 available")
	assert.Contains(t, buf.String(), "heraut self-update")
}

func TestUpdater_Hint_SilentWhenUpToDate(t *testing.T) {
	_, latestURL := newReleaseServer(t, "v1.2.3", nil)
	u := New("v1.2.3",
		WithLatestURL(latestURL),
		withCacheDir(t.TempDir()),
	)

	var buf strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u.Hint(ctx, &buf)

	assert.Empty(t, buf.String())
}

func TestUpdater_Hint_SkipsDevBuild(t *testing.T) {
	var called int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
	}))
	t.Cleanup(srv.Close)

	u := New("dev", WithLatestURL(srv.URL+"/releases/latest"))

	var buf strings.Builder
	u.Hint(context.Background(), &buf)

	assert.Empty(t, buf.String())
	assert.Zero(t, atomic.LoadInt32(&called), "no HTTP call expected for dev build")
}

func TestUpdater_Hint_SkipsEmptyURL(t *testing.T) {
	u := New("v1.2.3", WithLatestURL(""))

	var buf strings.Builder
	u.Hint(context.Background(), &buf)
	assert.Empty(t, buf.String())
}

func TestUpdater_Hint_24hCache(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		rel := Release{TagName: "v1.3.0"}
		_ = json.NewEncoder(w).Encode(rel)
	}))
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()
	fixedNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	u := New("v1.2.3",
		WithLatestURL(srv.URL+"/releases/latest"),
		withCacheDir(cacheDir),
		withNow(func() time.Time { return fixedNow }),
	)

	ctx := context.Background()
	var buf strings.Builder

	u.Hint(ctx, &buf)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls), "first call should fetch")

	buf.Reset()
	u.Hint(ctx, &buf)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls), "second call within 24h should use cache")
	assert.Contains(t, buf.String(), "v1.3.0", "cached hint still printed")
}

func TestUpdater_Hint_StaleCache(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		rel := Release{TagName: "v1.4.0"}
		_ = json.NewEncoder(w).Encode(rel)
	}))
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()
	fixedNow := time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC)
	staleTime := fixedNow.Add(-25 * time.Hour)

	// Write a stale cache entry
	staleCache := updateCache{CheckedAt: staleTime, LatestVersion: "v1.3.0"}
	data, _ := json.Marshal(staleCache)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "update-check.json"), data, 0o644))

	u := New("v1.2.3",
		WithLatestURL(srv.URL+"/releases/latest"),
		withCacheDir(cacheDir),
		withNow(func() time.Time { return fixedNow }),
	)

	var buf strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u.Hint(ctx, &buf)

	assert.EqualValues(t, 1, atomic.LoadInt32(&calls), "stale cache should trigger HTTP fetch")
	assert.Contains(t, buf.String(), "v1.4.0")
}

// ---- Do -------------------------------------------------------------------

func TestUpdater_Do_UpToDate(t *testing.T) {
	_, latestURL := newReleaseServer(t, "v1.2.3", nil)
	u := New("v1.2.3", WithLatestURL(latestURL))

	var buf strings.Builder
	err := u.Do(context.Background(), &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "up to date")
}

func TestUpdater_Do_Success(t *testing.T) {
	newContent := []byte("fake-new-binary-content")
	binName := assetName(bareVersion("v1.3.0"), "linux", "amd64")
	csumName := checksumAssetName(bareVersion("v1.3.0"))

	_, latestURL := newReleaseServer(t, "v1.3.0", map[string][]byte{
		binName:  newContent,
		csumName: buildChecksums(binName, newContent),
	})

	// Create a fake "current binary" in a temp dir
	dir := t.TempDir()
	execPath := filepath.Join(dir, "heraut")
	require.NoError(t, os.WriteFile(execPath, []byte("old-binary"), 0o755))

	u := New("v1.2.3",
		WithLatestURL(latestURL),
		withExecutable(func() (string, error) { return execPath, nil }),
		withPlatform("linux", "amd64"),
		withCacheDir(t.TempDir()),
	)

	var buf strings.Builder
	err := u.Do(context.Background(), &buf)
	require.NoError(t, err)

	got, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, newContent, got, "binary should be replaced with new content")
	assert.Contains(t, buf.String(), "v1.3.0")
}

func TestUpdater_Do_ChecksumMismatch(t *testing.T) {
	newContent := []byte("fake-new-binary")
	binName := assetName(bareVersion("v1.3.0"), "linux", "amd64")
	csumName := checksumAssetName(bareVersion("v1.3.0"))

	_, latestURL := newReleaseServer(t, "v1.3.0", map[string][]byte{
		binName:  newContent,
		csumName: []byte(fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), binName)), // wrong checksum
	})

	dir := t.TempDir()
	execPath := filepath.Join(dir, "heraut")
	oldContent := []byte("old-binary")
	require.NoError(t, os.WriteFile(execPath, oldContent, 0o755))

	u := New("v1.2.3",
		WithLatestURL(latestURL),
		withExecutable(func() (string, error) { return execPath, nil }),
		withPlatform("linux", "amd64"),
	)

	err := u.Do(context.Background(), &strings.Builder{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum")

	// Original binary must be untouched
	got, _ := os.ReadFile(execPath)
	assert.Equal(t, oldContent, got)

	// Temp file must be cleaned up
	_, statErr := os.Stat(execPath + ".new")
	assert.True(t, os.IsNotExist(statErr), ".new file should be removed on checksum failure")
}

func TestUpdater_Do_AssetNotFound(t *testing.T) {
	// No assets for freebsd/mips
	_, latestURL := newReleaseServer(t, "v1.3.0", map[string][]byte{
		assetName("1.3.0", "linux", "amd64"): []byte("content"),
	})

	u := New("v1.2.3",
		WithLatestURL(latestURL),
		withPlatform("freebsd", "mips"),
	)

	err := u.Do(context.Background(), &strings.Builder{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no asset found")
}
