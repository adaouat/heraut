package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sprintBaseYAML = `version: "1"
versioning:
  strategy: calver
  format: "YYYY.SS.PATCH"
  sprint: 3
`

const sprintMissingYAML = `version: "1"
versioning:
  strategy: calver
  format: "YYYY.SS.PATCH"
`

func TestIncrementSprint_FastPath(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(sprintBaseYAML), 0o600))

	got, err := config.IncrementSprint(path)
	require.NoError(t, err)
	assert.Equal(t, 4, got)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "sprint: 4")
	assert.NotContains(t, string(data), "sprint: 3")
}

func TestIncrementSprint_SlowPath(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(sprintMissingYAML), 0o600))

	got, err := config.IncrementSprint(path)
	require.NoError(t, err)
	assert.Equal(t, 1, got)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "sprint: 1"), "expected sprint: 1 in file")
}

func TestIncrementSprint_FileNotFound(t *testing.T) {
	_, err := config.IncrementSprint("/nonexistent/path/.heraut.yml")
	require.Error(t, err)
}
