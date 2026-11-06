package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSchema = `{
  "type": "object",
  "properties": {
    "version": {"type": "string", "description": "Schema version."},
    "versioning": {"$ref": "#/definitions/Versioning"},
    "changelog": {"$ref": "#/definitions/Doc"},
    "forges": {"type": "array", "description": "Platforms.", "items": {"$ref": "#/definitions/Forge"}},
    "environments": {"type": "object", "description": "Per env.", "additionalProperties": {"$ref": "#/definitions/Env"}},
    "rendering": {"$ref": "#/definitions/Rendering"}
  },
  "definitions": {
    "Versioning": {"type": "object", "description": "How versions work.", "properties": {
      "strategy": {"type": "string", "enum": ["semver", "calver"], "description": "The strategy | how."},
      "tags": {"type": "array", "items": {"type": "string"}, "description": "Tag list <glob>."}
    }},
    "Doc": {"type": "object", "description": "A document.", "properties": {
      "output": {"type": "string", "default": "CHANGELOG.md", "description": "Where."},
      "rendering": {"$ref": "#/definitions/Rendering"}
    }},
    "Rendering": {"type": "object", "description": "Looks.", "properties": {
      "excludes": {"type": "array", "items": {"type": "string"}, "description": "Dropped."}
    }},
    "Forge": {"type": "object", "description": "One forge.", "properties": {
      "name": {"type": "string", "description": "Its name."},
      "draft": {"type": "boolean", "description": "Draft."}
    }},
    "Env": {"type": "object", "description": "One environment.", "properties": {
      "changelog": {"$ref": "#/definitions/Doc"}
    }}
  }
}`

func TestConfigReference(t *testing.T) {
	got, err := configReference([]byte(testSchema))
	require.NoError(t, err)

	for _, want := range []string{
		"## `version`\n",
		"| `version` | string |  | Schema version. |",
		"## `versioning`\n\nHow versions work.\n",
		"| `versioning.strategy` | `semver` \\| `calver` |  | The strategy \\| how. |",
		"| `versioning.tags` | list of string |  | Tag list \\<glob\\>. |",
		"| `changelog.output` | string | `CHANGELOG.md` | Where. |",
		"| `forges[].name` | string |  | Its name. |",
		"| `forges[].draft` | boolean |  | Draft. |",
		"## `environments`\n\nPer env.\n",
		"| `environments.<name>.changelog` | object |  | A document. Same keys as `changelog`. |",
	} {
		assert.Contains(t, got, want)
	}
	assert.NotContains(t, got, "`environments.<name>.changelog.output`", "a definition is expanded once, then referred to")
	assert.Contains(t, got, "| `changelog.rendering` | object |  | Looks. Same keys as `rendering`. |",
		"a definition with its own top-level key is expanded there, even when another key reaches it first")
	assert.Contains(t, got, "| `rendering.excludes` | list of string |  | Dropped. |")
}

func TestReplaceGenerated(t *testing.T) {
	doc := "# Title\n\nIntro.\n\n" + beginMarker + "\nold\n" + endMarker + "\n\nOutro.\n"

	got, err := replaceGenerated(doc, "new\n")
	require.NoError(t, err)
	assert.Equal(t, "# Title\n\nIntro.\n\n"+beginMarker+"\nnew\n"+endMarker+"\n\nOutro.\n", got)

	_, err = replaceGenerated("no markers", "new\n")
	assert.Error(t, err)
}

// The committed configuration reference matches schema.json; `mise run docs:gen` rewrites it.
func TestConfigReference_IsCurrent(t *testing.T) {
	schema, err := os.ReadFile("../../schema.json")
	require.NoError(t, err)
	doc, err := os.ReadFile("../../docs/reference/configuration.md")
	require.NoError(t, err)

	table, err := configReference(schema)
	require.NoError(t, err)
	want, err := replaceGenerated(string(doc), table)
	require.NoError(t, err)
	assert.Equal(t, want, string(doc), "docs/reference/configuration.md is stale: run mise run docs:gen")
}
