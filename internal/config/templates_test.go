package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func unmarshalTemplates(t *testing.T, doc string) TemplateOverrides {
	t.Helper()
	var got TemplateOverrides
	require.NoError(t, yaml.Unmarshal([]byte(doc), &got))
	return got
}

func TestTemplateOverrides_FlatKeysUnchanged(t *testing.T) {
	got := unmarshalTemplates(t, `
title: "# Changelog"
subtitle: "All notable changes."
footer: "_footer_"
changelog: "root override"
release_notes: "root override"
`)
	assert.Equal(t, TemplateOverrides{
		"title":         "# Changelog",
		"subtitle":      "All notable changes.",
		"footer":        "_footer_",
		"changelog":     "root override",
		"release_notes": "root override",
	}, got)
}

func TestTemplateOverrides_NestedReleaseAndCommitFlattenToDottedKeys(t *testing.T) {
	got := unmarshalTemplates(t, `
release:
  section: "## {{ .Version }}"
  group: "### {{ .Name }}"
  contributors: "contribs"
  stats: "stats"
  footer: "rel-footer"
commit:
  message: "- {{ .Description }}"
  ticket: "[{{ .Text }}]"
  contributor: "* {{ .Author.Username }}"
`)
	assert.Equal(t, TemplateOverrides{
		"release.section":      "## {{ .Version }}",
		"release.group":        "### {{ .Name }}",
		"release.contributors": "contribs",
		"release.stats":        "stats",
		"release.footer":       "rel-footer",
		"commit.message":       "- {{ .Description }}",
		"commit.ticket":        "[{{ .Text }}]",
		"commit.contributor":   "* {{ .Author.Username }}",
	}, got)
}

func TestTemplateOverrides_MixedFlatAndNestedKeys(t *testing.T) {
	got := unmarshalTemplates(t, `
title: "# Changelog"
release:
  section: "## {{ .Version }}"
`)
	assert.Equal(t, TemplateOverrides{
		"title":           "# Changelog",
		"release.section": "## {{ .Version }}",
	}, got)
}

// A non-mapping value under "release"/"commit" is not special-cased: it decodes as an ordinary
// flat key. It is not a valid block on its own, but rejecting it is validateTemplateSnippets'
// job (ADR-0059's "unknown template block" error), not the unmarshaler's.
func TestTemplateOverrides_NonMappingReleaseKeyPassesThroughAsFlatKey(t *testing.T) {
	got := unmarshalTemplates(t, `release: "not a mapping"`)
	assert.Equal(t, TemplateOverrides{"release": "not a mapping"}, got)
}

func TestTemplateOverrides_EmptyNestedObjectContributesNothing(t *testing.T) {
	got := unmarshalTemplates(t, `release: {}`)
	assert.Empty(t, got)
}

func TestTemplateOverrides_DeeplyNestedSubValueErrors(t *testing.T) {
	var got TemplateOverrides
	err := yaml.Unmarshal([]byte("release:\n  section:\n    nested: x"), &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release.section")
}

// TestTemplateOverrides_CommitTrailersSkipped covers ADR-0060: commit.trailers is a list of
// rules, not a template-snippet string like every other commit.* key, so TemplateOverrides must
// not try to flatten it into its flat map (Rendering.UnmarshalYAML extracts it separately).
func TestTemplateOverrides_CommitTrailersSkipped(t *testing.T) {
	got := unmarshalTemplates(t, `
commit:
  message: "- {{ .Description }}"
  trailers:
    - token: Co-authored-by
      hide: true
`)
	assert.Equal(t, TemplateOverrides{"commit.message": "- {{ .Description }}"}, got,
		"commit.trailers must not appear in the flattened map")
}
