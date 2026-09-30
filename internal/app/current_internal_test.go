package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

// TestTagOrderFor_Semver_DropsPreReleaseAndOrdersByPrecedence covers T334: semver drops
// pre-release/invalid tags and reorders survivors by SemVer §11 precedence, ignoring build
// metadata, with a stable tie-break for equal-precedence tags.
func TestTagOrderFor_Semver_DropsPreReleaseAndOrdersByPrecedence(t *testing.T) {
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}
	order := tagOrderFor(cfg, "")
	require.NotNil(t, order)

	got := order([]string{"v1.4.0-rc.1", "v1.02.0", "v1.3.0", "v1.4.0+158404"})
	assert.Equal(t, []string{"v1.4.0+158404", "v1.3.0"}, got,
		"pre-release (v1.4.0-rc.1) and invalid SemVer (v1.02.0, leading zero) are dropped; "+
			"v1.4.0+158404 (a release) outranks v1.3.0")
}

// TestTagOrderFor_Semver_StableTies covers the documented tie-break: tags of equal §11 precedence
// (differing only in build metadata) keep their input order rather than being reshuffled.
func TestTagOrderFor_Semver_StableTies(t *testing.T) {
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}
	order := tagOrderFor(cfg, "")

	got := order([]string{"v1.4.0", "v1.4.0+158404", "v1.3.0"})
	assert.Equal(t, []string{"v1.4.0", "v1.4.0+158404", "v1.3.0"}, got,
		"v1.4.0 and v1.4.0+158404 are precedence-equal (build metadata ignored) — stable sort keeps input order")
}

// TestTagOrderFor_SemverPerEnv_ParsesThroughTagFormat covers T334: semver-per-env extracts each
// tag's bare version through the effective tag_format (not a bare prefix strip) before ordering.
func TestTagOrderFor_SemverPerEnv_ParsesThroughTagFormat(t *testing.T) {
	cfg := &config.Config{
		Versioning:   config.Versioning{Strategy: "semver-per-env", TagFormat: "{env}/{version}+{build}"},
		Environments: map[string]config.Environment{"uat": {}},
	}
	order := tagOrderFor(cfg, "uat")
	require.NotNil(t, order)

	got := order([]string{"uat/1.3.0+1", "uat/1.4.0+5", "uat/1.4.0"})
	assert.Equal(t, []string{"uat/1.4.0+5", "uat/1.3.0+1"}, got,
		"tags are parsed through the effective tag_format; uat/1.4.0 has no +build segment and doesn't match")
}

// TestTagOrderFor_Calver_ReturnsNil and TestTagOrderFor_CalverPerEnv_ReturnsNil guard the
// constraint that calver/calver-per-env output stays byte-for-byte unchanged: no order is
// injected, so native keeps walking git's version:refname order.
func TestTagOrderFor_Calver_ReturnsNil(t *testing.T) {
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"}}
	assert.Nil(t, tagOrderFor(cfg, ""))
}

func TestTagOrderFor_CalverPerEnv_ReturnsNil(t *testing.T) {
	cfg := &config.Config{
		Versioning:   config.Versioning{Strategy: "calver-per-env", Format: "YYYY.MM.PATCH"},
		Environments: map[string]config.Environment{"uat": {}},
	}
	assert.Nil(t, tagOrderFor(cfg, "uat"))
}
