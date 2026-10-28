package github_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/github"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	p := github.New(&config.Platform{Name: "github-internal"})
	assert.Equal(t, "github-internal", p.Name())
}

func TestReleaseURL(t *testing.T) {
	cfg := &config.Platform{Repository: "myorg/myrepo"}
	p := github.New(cfg)
	assert.Equal(t, "https://github.com/myorg/myrepo/releases/tag/v1.2.3", p.ReleaseURL("v1.2.3"))
}

func TestReleaseURL_FromEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
	p := github.New(&config.Platform{})
	assert.Equal(t, "https://github.com/envorg/envrepo/releases/tag/v1.0.0", p.ReleaseURL("v1.0.0"))
}

func TestReleaseURL_SelfHosted(t *testing.T) {
	cfg := &config.Platform{Repository: "org/repo", BaseURL: "https://github.example.com"}
	p := github.New(cfg)
	assert.Equal(t, "https://github.example.com/org/repo/releases/tag/v1.2.3", p.ReleaseURL("v1.2.3"))
}

// TestReleaseURLFromContext_* verify that ReleaseURLFromContext derives the release URL from
// a pre-resolved link context rather than re-resolving from config/env, so the pipeline can
// keep the displayed URL consistent with the one used to generate release notes (ADR-0022).
func TestReleaseURLFromContext_AmbientContext(t *testing.T) {
	// Ambient context: BaseURL is the full project URL (Owner/Repo empty) — built from
	// GITHUB_SERVER_URL + GITHUB_REPOSITORY in ambientLinkContext().
	cfg := &config.Platform{Repository: "test/gh-repo"}
	p := github.New(cfg)
	lc := &port.LinkContext{BaseURL: "https://github.com/test/gh-repo", Platform: "github"}
	assert.Equal(t, "https://github.com/test/gh-repo/releases/tag/v1.0.0", p.ReleaseURLFromContext("v1.0.0", lc))
}

func TestReleaseURLFromContext_PlatformContext(t *testing.T) {
	// Platform context: BaseURL is the host (Owner/Repo set) — built from LinkContext().
	cfg := &config.Platform{Repository: "test/gh-repo"}
	p := github.New(cfg)
	lc := &port.LinkContext{BaseURL: "https://github.com", Owner: "test", Repo: "gh-repo", Platform: "github"}
	assert.Equal(t, "https://github.com/test/gh-repo/releases/tag/v1.0.0", p.ReleaseURLFromContext("v1.0.0", lc))
}

func TestReleaseURLFromContext_Nil_FallsBackToReleaseURL(t *testing.T) {
	cfg := &config.Platform{Repository: "org/repo"}
	p := github.New(cfg)
	assert.Equal(t, p.ReleaseURL("v1.0.0"), p.ReleaseURLFromContext("v1.0.0", nil))
}

// Tags may carry SemVer build metadata ("+"), which must reach the URL as %2B (ADR-0064).
func TestReleaseURL_EscapesPlusInTag(t *testing.T) {
	p := github.New(&config.Platform{Repository: "acme/widget"})
	assert.Equal(t, "https://github.com/acme/widget/releases/tag/v1.4.0%2B158404", p.ReleaseURL("v1.4.0+158404"))

	ambient := &port.LinkContext{BaseURL: "https://github.com/acme/widget", Platform: "github"}
	assert.Equal(t, "https://github.com/acme/widget/releases/tag/uat/7.4.1%2B158404", p.ReleaseURLFromContext("uat/7.4.1+158404", ambient))

	platform := &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"}
	assert.Equal(t, "https://github.com/acme/widget/releases/tag/v1.4.0%2B158404", p.ReleaseURLFromContext("v1.4.0+158404", platform))
}

func TestLinkContext(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	cfg := &config.Platform{Repository: "acme/widget", BaseURL: "https://github.com"}
	p := github.New(cfg)
	assert.Equal(t, port.LinkContext{
		BaseURL:  "https://github.com",
		Owner:    "acme",
		Repo:     "widget",
		Platform: "github",
	}, p.LinkContext())
}

func TestLinkContext_DefaultBaseURL(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	// BaseURL empty (config not normalized) → falls back to the default host.
	cfg := &config.Platform{Repository: "acme/widget"}
	lc := github.New(cfg).LinkContext()
	assert.Equal(t, "https://github.com", lc.BaseURL)
}

func TestLinkContext_FromEnv(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
	lc := github.New(&config.Platform{}).LinkContext()
	assert.Equal(t, "envorg", lc.Owner)
	assert.Equal(t, "envrepo", lc.Repo)
}

func TestHasAssets(t *testing.T) {
	assert.False(t, github.New(&config.Platform{}).HasAssets())
	assert.True(t, github.New(&config.Platform{Assets: []string{"dist/*"}}).HasAssets())
}

func TestCheckAssets(t *testing.T) {
	repo := t.TempDir()
	t.Chdir(repo)
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o644))

	assert.NoError(t, github.New(&config.Platform{}).CheckAssets())
	assert.NoError(t, github.New(&config.Platform{Assets: []string{"dist/*"}, AllowMissingAssets: true}).CheckAssets())

	err := github.New(&config.Platform{Assets: []string{filepath.Join(outside, "*.txt")}}).CheckAssets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
}

// ADR-0076: a pattern matching nothing is an error unless release.allow_missing_assets is set.
func TestCheckAssets_UnmatchedPatternIsRefusedUnlessAllowed(t *testing.T) {
	t.Chdir(t.TempDir())

	err := github.New(&config.Platform{Assets: []string{"dist/*"}}).CheckAssets()
	require.ErrorIs(t, err, port.ErrAssetUnmatched)

	assert.NoError(t, github.New(&config.Platform{Assets: []string{"dist/*"}, AllowMissingAssets: true}).CheckAssets())
}

// ADR-0077 lifts the ADR-0076 addendum: the name travels as a query parameter, so a "#" in a path is
// accepted (GitHub publishes it under a rewritten name, with a warning at upload).
func TestCheckAssets_PathWithAHashIsAccepted(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.MkdirAll("dist", 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("dist", "app#1.txt"), []byte("x"), 0o644))

	assert.NoError(t, github.New(&config.Platform{Assets: []string{"dist/*"}}).CheckAssets())
}

// GitHub renames some characters and compares asset names without case, so two files whose names
// differ only there would collide after the tag is pushed (422 already_exists). The preflight
// refuses them first, in a dry run too. Each file sits in its own directory, so the pair can exist on
// a case-insensitive filesystem.
func TestCheckAssets_RefusesNamesGitHubWouldMerge(t *testing.T) {
	for _, pair := range [][2]string{
		{"Case.txt", "case.txt"},
		{"dup#1.txt", "dup.1.txt"},
		{"sp ace.txt", "sp.ace.txt"},
		{"Straße.txt", "strasse.txt"},
		{"Smørrebrød.txt", "smorrebrod.txt"},
	} {
		t.Run(pair[0]+" and "+pair[1], func(t *testing.T) {
			t.Chdir(t.TempDir())
			for i, name := range pair {
				dir := []string{"a", "b"}[i]
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644))
			}

			err := github.New(&config.Platform{Assets: []string{"a/*", "b/*"}}).CheckAssets()

			require.Error(t, err)
			assert.Contains(t, err.Error(), pair[0])
			assert.Contains(t, err.Error(), pair[1])
			assert.Contains(t, err.Error(), "GitHub")
			assert.NotErrorIs(t, err, port.ErrAssetUnmatched, "a dry run must refuse it too")
		})
	}
}

// Characters GitHub keeps ("+", "@", "-", "_") must not be treated as renamed, or a release GitHub
// accepts would be refused.
func TestCheckAssets_NamesGitHubKeepsApartPass(t *testing.T) {
	for _, pair := range [][2]string{
		{"a+b.txt", "a.b.txt"},
		{"a@b.txt", "a.b.txt"},
		{"a-b.txt", "a_b.txt"},
		{"a--b.txt", "a-b.txt"},
	} {
		t.Run(pair[0]+" and "+pair[1], func(t *testing.T) {
			t.Chdir(t.TempDir())
			for i, name := range pair {
				dir := []string{"a", "b"}[i]
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644))
			}

			assert.NoError(t, github.New(&config.Platform{Assets: []string{"a/*", "b/*"}}).CheckAssets())
		})
	}
}

// A loopback base_url is a GitHub Enterprise Server to the driver, so the API sits under /api/v3
// and the test never leaves the machine (Spec 06 § Contract shows this test).
func TestCreateRelease_SendsTheRelease(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("GH_TOKEN", "test-token")
	var got []string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	t.Cleanup(srv.Close)

	plat := github.New(&config.Platform{Repository: "acme/widget", BaseURL: srv.URL})
	require.NoError(t, plat.CreateRelease("v1.2.3", "release notes body", false))

	assert.Equal(t, []string{"POST /api/v3/repos/acme/widget/releases"}, got)
	assert.Equal(t, "v1.2.3", body["tag_name"])
	assert.Equal(t, "release notes body", body["body"])
}
