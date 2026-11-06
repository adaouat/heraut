package config_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maxDescription keeps a description readable in an editor's hover popup.
const maxDescription = 300

// internalReference matches what a user reading a hover cannot follow: ADR numbers, roadmap task
// ids and spec paths.
var internalReference = regexp.MustCompile(`ADR-\d|\bT\d{2,}\b|docs/specs|§`)

// Every key of .heraut.yml is described for the people writing the file: editors show the schema's
// descriptions on hover, and the configuration reference is generated from them.
func TestSchema_EveryKeyIsDescribedForUsers(t *testing.T) {
	raw, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(raw, &schema))
	defs, _ := schema["definitions"].(map[string]any)

	description := func(node map[string]any) string {
		if d, ok := node["description"].(string); ok && d != "" {
			return d
		}
		// draft-07 ignores a $ref's sibling keys, so a referenced key is described by its definition.
		ref, _ := node["$ref"].(string)
		if items, ok := node["items"].(map[string]any); ok && ref == "" {
			ref, _ = items["$ref"].(string)
		}
		if def, ok := defs[strings.TrimPrefix(ref, "#/definitions/")].(map[string]any); ok && ref != "" {
			d, _ := def["description"].(string)
			return d
		}
		return ""
	}
	check := func(path, d string) {
		t.Run(path, func(t *testing.T) {
			require.NotEmpty(t, d, "no description")
			assert.NotRegexp(t, internalReference, d, "cites an internal document")
			assert.LessOrEqual(t, len(d), maxDescription, "longer than %d characters: %q", maxDescription, d)
		})
	}

	var walk func(path string, node map[string]any)
	walk = func(path string, node map[string]any) {
		props, _ := node["properties"].(map[string]any)
		for name, p := range props {
			pm, _ := p.(map[string]any)
			check(path+name, description(pm))
			walk(path+name+".", pm)
		}
		for _, key := range []string{"items", "additionalProperties"} {
			if child, ok := node[key].(map[string]any); ok {
				walk(path, child)
			}
		}
	}
	walk("", schema)
	for name, d := range defs {
		dm, _ := d.(map[string]any)
		if desc, ok := dm["description"].(string); ok {
			check("definitions."+name, desc)
		}
		walk(name+".", dm)
	}
}
