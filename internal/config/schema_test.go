package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const schemaPath = "../../schema.json"

func loadSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(schemaPath)
	require.NoError(t, err, "schema.json must be a valid JSON Schema")
	return sch
}

// yamlFileToJSON reads a YAML file and returns it as a JSON-compatible value.
func yamlFileToJSON(t *testing.T, path string) any {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var v any
	require.NoError(t, yaml.Unmarshal(raw, &v))

	// Round-trip through JSON to normalise map key types.
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var out any
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

func TestSchema_ValidJSON(t *testing.T) {
	raw, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	var v any
	assert.NoError(t, json.Unmarshal(raw, &v), "schema.json must be valid JSON")
}

func TestSchema_ValidFixtures(t *testing.T) {
	sch := loadSchema(t)

	fixtures, err := filepath.Glob("../../testdata/config/valid/*.yml")
	require.NoError(t, err)
	require.NotEmpty(t, fixtures)

	for _, fix := range fixtures {
		t.Run(filepath.Base(fix), func(t *testing.T) {
			v := yamlFileToJSON(t, fix)
			assert.NoError(t, sch.Validate(v))
		})
	}
}

func TestSchema_InvalidFixtures(t *testing.T) {
	sch := loadSchema(t)

	tests := []struct {
		file   string
		reason string
	}{
		{"missing_version.yml", "missing required field version"},
		{"invalid_strategy.yml", "strategy enum violation"},
		{"invalid_generator.yml", "generator enum violation"},
		{"invalid_enrichment_policy.yml", "enrichment_policy enum violation"},
		{"invalid_tickets.yml", "tickets item missing required url"},
		{"rendering_unknown_template_block.yml", "rendering.templates unknown block additionalProperties violation"},
		{"unknown_key.yml", "additionalProperties violation"},
		{"perenv_no_environments.yml", "per-env strategy requires environments"},
	}

	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join("../../testdata/config/invalid", tc.file)
			v := yamlFileToJSON(t, path)
			err := sch.Validate(v)
			assert.Error(t, err, "expected schema to reject %s (%s)", tc.file, tc.reason)
		})
	}
}

func TestSchema_SemanticOnlyFixturesPassSchema(t *testing.T) {
	sch := loadSchema(t)

	// These fixtures are structurally valid — schema must accept them.
	// Semantic errors (cycles, ambiguous source) are caught by config.Validate only.
	for _, name := range []string{"source_ambiguous.yml", "source_cycle.yml"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("../../testdata/config/invalid", name)
			v := yamlFileToJSON(t, path)
			assert.NoError(t, sch.Validate(v))
		})
	}
}
