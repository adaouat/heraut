package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

// The environment variables reference names every variable heraut reads, so a variable added to
// config.CIEnvVars cannot ship undocumented.
func TestEnvironmentVariablesReference_NamesEveryVariable(t *testing.T) {
	page, err := os.ReadFile("../../docs/reference/environment-variables.md")
	require.NoError(t, err)

	names := append([]string{config.EnvConfigFile, "HERAUT_SKIP_HOOKS", "HERAUT_CHECK_UPDATE"}, config.CIEnvVars...)
	for _, name := range names {
		assert.Contains(t, string(page), "`"+name+"`", "docs/reference/environment-variables.md must document %s", name)
	}
}
