package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/adaouat/heraut/internal/config"
)

// A loaded config encodes back to YAML that loads to the same config: heraut init writes configs this
// way when it updates one, so no key, nesting or explicit empty list may be lost.
func TestConfig_EncodesBackToTheSameConfig(t *testing.T) {
	fixtures, err := filepath.Glob("../../testdata/config/valid/*.yml")
	require.NoError(t, err)
	fixtures = append(fixtures, "../../docs/heraut.sample.yml")
	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			want, err := config.Load(fixture)
			require.NoError(t, err)

			out, err := yaml.Marshal(want)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), ".heraut.yml")
			require.NoError(t, os.WriteFile(path, out, 0o600))
			got, err := config.Load(path)
			require.NoError(t, err, "encoded:\n%s", out)

			assert.Equal(t, want, got, "encoded:\n%s", out)
		})
	}
}
