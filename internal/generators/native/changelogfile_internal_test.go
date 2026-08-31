package native

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnchorLine(t *testing.T) {
	assert.Equal(t, "<!-- heraut-release: v1.2.3 -->", anchorLine("v1.2.3"))
}

func TestParseChangelog_NoAnchors(t *testing.T) {
	pre, secs, has := parseChangelog("# Changelog\n\n## [1.0.0] - 2026-01-01\n\n- x\n")
	assert.False(t, has)
	assert.Empty(t, secs)
	assert.Equal(t, "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n- x\n", pre)
}

func TestParseChangelog_SplitsSections(t *testing.T) {
	content := "# Changelog\n\n" +
		"<!-- heraut-release: v2.0.0 -->\n## [2.0.0]\n\n- two\n\n" +
		"<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	pre, secs, has := parseChangelog(content)
	require.True(t, has)
	assert.Equal(t, "# Changelog\n\n", pre)
	require.Len(t, secs, 2)
	assert.Equal(t, "v2.0.0", secs[0].tag)
	assert.Equal(t, "v1.0.0", secs[1].tag)
	assert.Equal(t, "<!-- heraut-release: v2.0.0 -->\n## [2.0.0]\n\n- two", secs[0].text)
}

// TestParseChangelog_StripsFooterRegion covers ADR-0050: a trailing footer-anchor-marked region
// must never be folded into the last section's captured text, or it would survive incremental
// splices forever as if it were part of that release's own body.
func TestParseChangelog_StripsFooterRegion(t *testing.T) {
	content := "# Changelog\n\n<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- x\n" +
		footerAnchor + "\n_footer text_\n"
	pre, secs, has := parseChangelog(content)
	require.True(t, has)
	require.Len(t, secs, 1)
	assert.Equal(t, "<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- x", secs[0].text)
	assert.Equal(t, "# Changelog\n\n", pre)
}

func TestSpliceSection_InsertsAboveTop(t *testing.T) {
	existing := "# Changelog\n\n" +
		"<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	got, err := spliceSection(existing, "## [2.0.0]\n\n- two", "v2.0.0", "# Changelog\n\n", "")
	require.NoError(t, err)
	want := "# Changelog\n\n" +
		"<!-- heraut-release: v2.0.0 -->\n## [2.0.0]\n\n- two\n\n" +
		"<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	assert.Equal(t, want, got)
}

func TestSpliceSection_ReplacesSameTag(t *testing.T) {
	existing := "# Changelog\n\n" +
		"<!-- heraut-release: v2.0.0 -->\n## [2.0.0]\n\n- OLD\n\n" +
		"<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	got, err := spliceSection(existing, "## [2.0.0]\n\n- NEW", "v2.0.0", "# Changelog\n\n", "")
	require.NoError(t, err)
	assert.Contains(t, got, "- NEW")
	assert.NotContains(t, got, "- OLD")
	assert.Equal(t, 1, strings.Count(got, "heraut-release: v2.0.0"), "no duplicate section")
	assert.Contains(t, got, "<!-- heraut-release: v1.0.0 -->", "older section preserved")
}

func TestSpliceSection_ForeignFileErrors(t *testing.T) {
	_, err := spliceSection("# Changelog\n\n## [1.0.0]\n\n- x\n", "## [2.0.0]\n\n- y", "v2.0.0", "# Changelog\n\n", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoAnchors))
}

// TestSpliceSection_RefreshesPreambleAndPostamble covers ADR-0050: an incremental splice discards
// whatever preamble/footer were previously on disk and always uses the caller-supplied fresh ones
// — never the stale content parsed out of existing, and never duplicated.
func TestSpliceSection_RefreshesPreambleAndPostamble(t *testing.T) {
	existing := "# Stale Title\n\n<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n" +
		footerAnchor + "\n_OLD FOOTER_\n"
	got, err := spliceSection(existing, "## [1.1.0]\n\n- two", "v1.1.0", "# Fresh Title\n\n", "\n_NEW FOOTER_\n")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(got, "# Fresh Title\n\n"), "the fresh preamble replaces the stale one")
	assert.NotContains(t, got, "Stale Title")
	assert.Contains(t, got, "_NEW FOOTER_")
	assert.NotContains(t, got, "OLD FOOTER", "the stale footer region is discarded, not preserved")
	assert.Equal(t, 1, strings.Count(got, footerAnchor), "exactly one footer anchor, not duplicated")
}

// TestSpliceSection_NoPostambleOmitsAnchor covers nulling the footer block (footer: ""): no anchor
// line is written when there is nothing to mark, keeping an unconfigured footer's on-disk shape
// unchanged from before ADR-0050.
func TestSpliceSection_NoPostambleOmitsAnchor(t *testing.T) {
	existing := "# Changelog\n\n<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	got, err := spliceSection(existing, "## [1.1.0]\n\n- two", "v1.1.0", "# Changelog\n\n", "")
	require.NoError(t, err)
	assert.NotContains(t, got, footerAnchor)
}
