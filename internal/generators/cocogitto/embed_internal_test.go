package cocogitto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmbeddedTemplates_RenderRemoteLinks verifies the embedded Tera templates render
// commit links from cog's repository_url context (populated by --remote/--owner/
// --repository, T69) while staying link-free when it is absent (single-platform /
// no-context — non-regression). Read through the production embed.FS, not the source tree.
// Actual rendering is verified manually against real cog (the suite has no real-binary
// tests). See ADR-0021 / T71b.
func TestEmbeddedTemplates_RenderRemoteLinks(t *testing.T) {
	for _, name := range []string{"release-notes.tera", "changelog.tera"} {
		t.Run(name, func(t *testing.T) {
			data, err := embedded.ReadFile(name)
			require.NoError(t, err)
			tmpl := string(data)
			assert.Contains(t, tmpl, "repository_url", "must reference cog's repository_url context")
			assert.Contains(t, tmpl, "/commit/", "must render a commit link")
			assert.Contains(t, tmpl, "if repository_url", "link must be guarded so absent context stays link-free")
			assert.Contains(t, tmpl, "commit.signature", "must render the author (T76)")
		})
	}
}

// TestEmbeddedCogToml_Cog7Schema guards against the bug T76 fixed: the embedded cog.toml
// used a git-cliff-style [changelog] commit_parsers block that cog 7.0.0 rejects ("unknown
// field commit_parsers"), breaking the default generator: cocogitto path. The valid cog
// schema is the top-level [commit_types] table (changelog_title / omit_from_changelog).
func TestEmbeddedCogToml_Cog7Schema(t *testing.T) {
	data, err := embedded.ReadFile("cog.toml")
	require.NoError(t, err)
	toml := string(data)

	assert.NotContains(t, toml, "commit_parsers", "commit_parsers is not a cog field — it breaks cog 7.0.0")
	assert.Contains(t, toml, "[commit_types]", "type titles/omissions use cog's [commit_types] table")
	// Emoji titles for the kept types (T76 enrichment).
	assert.Contains(t, toml, `"🚀 Features"`)
	assert.Contains(t, toml, `"🐛 Bug Fixes"`)
	// Noise types are still omitted (preserving the old intent).
	assert.Contains(t, toml, "omit_from_changelog")
	for _, omitted := range []string{"chore", "ci", "build", "test", "style"} {
		assert.Contains(t, toml, omitted+" =", "noise type %q must be configured (omitted)", omitted)
	}
}
