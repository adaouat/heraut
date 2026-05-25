package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultProjectURL = "https://github.com/adaouat/heraut"
	defaultLatestURL  = "https://api.github.com/repos/adaouat/heraut/releases/latest"
)

// updateCache is persisted to $XDG_CACHE_HOME/heraut/update-check.json.
type updateCache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

// Updater checks for and performs heraut self-updates.
type Updater struct {
	currentVersion string
	projectURL     string
	latestURL      string
	client         *http.Client
	// injectable for tests
	cacheDir   func() (string, error)
	executable func() (string, error)
	now        func() time.Time
	goos       string
	goarch     string
}

// Option configures an Updater.
type Option func(*Updater)

// WithLatestURL overrides the GitHub Releases API endpoint. Intended for tests.
func WithLatestURL(url string) Option {
	return func(u *Updater) { u.latestURL = url }
}

// unexported test-only options ------------------------------------------------

func withCacheDir(dir string) Option {
	return func(u *Updater) {
		u.cacheDir = func() (string, error) { return dir, nil }
	}
}

func withExecutable(fn func() (string, error)) Option {
	return func(u *Updater) { u.executable = fn }
}

func withNow(fn func() time.Time) Option {
	return func(u *Updater) { u.now = fn }
}

func withPlatform(goos, goarch string) Option {
	return func(u *Updater) {
		u.goos = goos
		u.goarch = goarch
	}
}

// New constructs an Updater. currentVersion should be the build-time version
// (e.g. "v1.2.3" or "dev"). Use WithLatestURL to override the API endpoint in tests.
func New(currentVersion string, opts ...Option) *Updater {
	u := &Updater{
		currentVersion: currentVersion,
		projectURL:     defaultProjectURL,
		latestURL:      defaultLatestURL,
		client:         &http.Client{},
		cacheDir:       defaultCacheDir,
		executable:     os.Executable,
		now:            time.Now,
		goos:           runtime.GOOS,
		goarch:         runtime.GOARCH,
	}
	for _, o := range opts {
		o(u)
	}
	return u
}

// Check fetches the latest release and returns (latestTag, updateAvailable, error).
// It always fetches — it does not use the 24 h cache.
func (u *Updater) Check(ctx context.Context) (string, bool, error) {
	rel, err := fetchRelease(ctx, u.client, u.latestURL)
	if err != nil {
		return "", false, err
	}
	latest := rel.TagName
	return latest, latest != u.currentVersion, nil
}

// Hint checks (with 24 h caching) whether an update is available and prints a
// hint line to w if so. Errors are silently swallowed — the hint must never
// affect a command's exit code. ctx should carry a short timeout (≈500 ms).
func (u *Updater) Hint(ctx context.Context, w io.Writer) {
	if u.currentVersion == "dev" || u.latestURL == "" {
		return
	}
	latest, err := u.hintLatest(ctx)
	if err != nil || latest == "" || latest == u.currentVersion {
		return
	}
	_, _ = fmt.Fprintf(w, "hint: heraut %s available — run: heraut self-update\n", latest)
}

// hintLatest returns the latest version using the 24 h cache, fetching only when stale.
func (u *Updater) hintLatest(ctx context.Context) (string, error) {
	cache, err := u.readCache()
	if err == nil && cache != nil && u.now().Sub(cache.CheckedAt) < 24*time.Hour {
		return cache.LatestVersion, nil
	}

	rel, err := fetchRelease(ctx, u.client, u.latestURL)
	if err != nil {
		return "", err
	}
	_ = u.writeCache(&updateCache{CheckedAt: u.now(), LatestVersion: rel.TagName})
	return rel.TagName, nil
}

// Do performs the full self-update: fetch → find asset → download → verify
// SHA-256 → chmod → atomic replace → macOS quarantine removal.
func (u *Updater) Do(ctx context.Context, w io.Writer) error {
	rel, err := fetchRelease(ctx, u.client, u.latestURL)
	if err != nil {
		return fmt.Errorf("fetching release: %w", err)
	}

	latest := rel.TagName
	if latest == u.currentVersion {
		_, _ = fmt.Fprintln(w, "heraut is already up to date")
		return nil
	}

	bare := bareVersion(latest)
	binName := assetName(bare, u.goos, u.goarch)
	csumName := checksumAssetName(bare)

	binAsset := findAsset(rel.Assets, binName)
	if binAsset == nil {
		return fmt.Errorf("no asset found for %s/%s (looking for %q)", u.goos, u.goarch, binName)
	}
	csumAsset := findAsset(rel.Assets, csumName)
	if csumAsset == nil {
		return fmt.Errorf("no checksum file found (looking for %q)", csumName)
	}

	execPath, err := u.executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	tmpPath := execPath + ".new"

	if err := u.downloadFile(ctx, binAsset.BrowserDownloadURL, tmpPath); err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}

	if err := u.verifyChecksum(ctx, tmpPath, binName, csumAsset.BrowserDownloadURL); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}

	if err := u.atomicReplace(tmpPath, execPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	removeQuarantine(execPath)

	_, _ = fmt.Fprintf(w, "heraut updated to %s\n", latest)
	return nil
}

func (u *Updater) downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d downloading binary", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (u *Updater) verifyChecksum(ctx context.Context, binPath, binName, csumURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, csumURL, nil)
	if err != nil {
		return err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	csumData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	expected := ""
	for _, line := range strings.Split(string(csumData), "\n") {
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[1]) == binName {
			expected = strings.TrimSpace(parts[0])
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("no checksum entry for %q in checksums file", binName)
	}

	f, err := os.Open(binPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("SHA-256 mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

// atomicReplace moves src to dst. On Windows, renames dst to dst.old first
// because a running executable cannot be overwritten directly.
func (u *Updater) atomicReplace(src, dst string) error {
	if u.goos == "windows" {
		old := dst + ".old"
		_ = os.Remove(old) // clean up any leftover from a previous update
		if err := os.Rename(dst, old); err != nil {
			return permissionError(dst, err)
		}
	}
	if err := os.Rename(src, dst); err != nil {
		return permissionError(dst, err)
	}
	return nil
}

// ---- cache helpers ----------------------------------------------------------

func defaultCacheDir() (string, error) {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "heraut"), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolving cache dir: %w", err)
	}
	return filepath.Join(base, "heraut"), nil
}

func (u *Updater) cacheFilePath() (string, error) {
	dir, err := u.cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "update-check.json"), nil
}

func (u *Updater) readCache() (*updateCache, error) {
	path, err := u.cacheFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cache: %w", err)
	}
	var c updateCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing cache: %w", err)
	}
	return &c, nil
}

func (u *Updater) writeCache(c *updateCache) error {
	path, err := u.cacheFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ---- error helpers ----------------------------------------------------------

func permissionError(path string, err error) error {
	if os.IsPermission(err) {
		return fmt.Errorf("permission denied replacing %s\n  hint: run with elevated privileges: sudo heraut self-update", path)
	}
	return fmt.Errorf("replacing binary: %w", err)
}
