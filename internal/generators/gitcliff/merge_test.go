package gitcliff_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/generators/gitcliff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge_NoOverride(t *testing.T) {
	base := `
[changelog]
header = "# Changelog"
trim = true

[git]
tag_pattern = "v[0-9].*"
`
	result, err := gitcliff.MergeTOML(base, "")
	require.NoError(t, err)
	assert.Contains(t, result, "header")
	assert.Contains(t, result, "tag_pattern")
}

func TestMerge_ScalarOverride(t *testing.T) {
	base := `
[changelog]
header = "# Changelog"
trim = true
`
	override := `
[changelog]
header = "# My App"
`
	result, err := gitcliff.MergeTOML(base, override)
	require.NoError(t, err)
	assert.Contains(t, result, "# My App")
	// trim should still be present from base
	assert.Contains(t, result, "trim")
}

func TestMerge_ArrayReplaces(t *testing.T) {
	base := `
[git]
commit_parsers = [
  { message = "^feat", group = "Features" },
  { message = "^fix", group = "Bug Fixes" },
]
`
	override := `
[git]
commit_parsers = [
  { message = "^feat", group = "New Features" },
]
`
	result, err := gitcliff.MergeTOML(base, override)
	require.NoError(t, err)
	assert.Contains(t, result, "New Features")
	// Original "Bug Fixes" should be gone (array replaced, not merged)
	assert.NotContains(t, result, "Bug Fixes")
}

func TestMerge_NewTableAdded(t *testing.T) {
	base := `
[changelog]
header = "# Changelog"
`
	override := `
[remote.gitlab]
owner = "myorg"
repo = "myrepo"
`
	result, err := gitcliff.MergeTOML(base, override)
	require.NoError(t, err)
	assert.Contains(t, result, "myorg")
	assert.Contains(t, result, "# Changelog")
}

func TestMerge_InvalidTOML(t *testing.T) {
	_, err := gitcliff.MergeTOML("not valid {{{ toml", "")
	require.Error(t, err)
}

func TestMerge_EmptyBase(t *testing.T) {
	// empty base TOML → baseMap is nil, covered by the nil-check branch
	result, err := gitcliff.MergeTOML("", "[changelog]\nheader = \"x\"")
	require.NoError(t, err)
	assert.Contains(t, result, "header")
}

func TestMerge_InvalidOverrideTOML(t *testing.T) {
	_, err := gitcliff.MergeTOML("[changelog]\nheader = \"x\"", "not valid {{{ toml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing override TOML")
}
