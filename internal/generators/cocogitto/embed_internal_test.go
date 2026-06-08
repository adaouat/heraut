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
		})
	}
}
