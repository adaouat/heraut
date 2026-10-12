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

// TestTagOrderFor_Calver_OrdersByDottedIntegersAndKeepsEveryTag covers T356: CalVer orders by the
// lenient dotted-integer comparison (zero padding is just a number, so 2026.10 outranks 2026.5),
// newest first, and keeps every tag — CalVer has no pre-release to drop.
func TestTagOrderFor_Calver_OrdersByDottedIntegersAndKeepsEveryTag(t *testing.T) {
	prefix := "v"
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH", TagPrefix: &prefix}}
	order := tagOrderFor(cfg, "")
	require.NotNil(t, order)

	got := order([]string{"v2026.05.2", "v2026.10.0", "v2026.5.10", "v2025.12.3"})
	assert.Equal(t, []string{"v2026.10.0", "v2026.5.10", "v2026.05.2", "v2025.12.3"}, got)
}

func TestTagOrderFor_Calver_StableTiesAndUnparsableTagsLast(t *testing.T) {
	prefix := "rel-"
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH", TagPrefix: &prefix}}
	order := tagOrderFor(cfg, "")

	got := order([]string{"rel-2026.05.1", "stray", "rel-2026.5.1", "rel-2026.06.0"})
	assert.Equal(t, []string{"rel-2026.06.0", "rel-2026.05.1", "rel-2026.5.1", "stray"}, got,
		"equal versions keep input order; a tag outside the prefix sorts last")
}

func TestTagOrderFor_CalverPerEnv_ParsesThroughTagFormat(t *testing.T) {
	cfg := &config.Config{
		Versioning:   config.Versioning{Strategy: "calver-per-env", Format: "YYYY.MM.PATCH", TagFormat: "{env}/{version}"},
		Environments: map[string]config.Environment{"uat": {}},
	}
	order := tagOrderFor(cfg, "uat")
	require.NotNil(t, order)

	got := order([]string{"uat/2026.05.1", "uat/2026.06.0", "uat/2026.05.10"})
	assert.Equal(t, []string{"uat/2026.06.0", "uat/2026.05.10", "uat/2026.05.1"}, got)
}

// TestNotesTagOrderFor_Calver_MatchesChangelogOrder: CalVer has no pre-release, so a pre-release
// run's notes use the same order as the changelog.
func TestNotesTagOrderFor_Calver_MatchesChangelogOrder(t *testing.T) {
	prefix := "v"
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH", TagPrefix: &prefix}}
	in := []string{"v2026.05.1", "v2026.06.0"}
	assert.Equal(t, tagOrderFor(cfg, "")(in), notesTagOrderFor(cfg, "")(in))
}
