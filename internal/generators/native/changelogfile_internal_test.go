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

func TestSpliceSection_InsertsAboveTop(t *testing.T) {
	existing := "# Changelog\n\n" +
		"<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	got, err := spliceSection(existing, "## [2.0.0]\n\n- two", "v2.0.0")
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
	got, err := spliceSection(existing, "## [2.0.0]\n\n- NEW", "v2.0.0")
	require.NoError(t, err)
	assert.Contains(t, got, "- NEW")
	assert.NotContains(t, got, "- OLD")
	assert.Equal(t, 1, strings.Count(got, "heraut-release: v2.0.0"), "no duplicate section")
	assert.Contains(t, got, "<!-- heraut-release: v1.0.0 -->", "older section preserved")
}

func TestSpliceSection_ForeignFileErrors(t *testing.T) {
	_, err := spliceSection("# Changelog\n\n## [1.0.0]\n\n- x\n", "## [2.0.0]\n\n- y", "v2.0.0")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoAnchors))
}
