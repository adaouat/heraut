package github_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/platforms/github"
	"github.com/adaouat/heraut/internal/port"
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
