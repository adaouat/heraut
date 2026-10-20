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

func bumpSprint(t *testing.T, src string) (int, string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(src), 0o600))
	got, err := config.IncrementSprint(path)
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	return got, string(data), err
}

func TestIncrementSprint_LeavesSprintLinesOutsideTheVersioningBlock(t *testing.T) {
	src := `version: "1"
hooks:
  pre_tag:
    - run: |
        sprint: 7
versioning:
  strategy: calver
  format: "YYYY.SS.PATCH"
  sprint: 3
changelog:
  output: CHANGELOG.md
`
	got, data, err := bumpSprint(t, src)
	require.NoError(t, err)
	assert.Equal(t, 4, got)
	assert.Contains(t, data, "        sprint: 7\n", "a sprint: line inside a hook script is not the config key")
	assert.Contains(t, data, "  sprint: 4\n")
	assert.Equal(t, 1, strings.Count(data, "sprint: 4"))
}

func TestIncrementSprint_KeepsTheValuesTrailingComment(t *testing.T) {
	src := "version: \"1\"\nversioning:\n  strategy: calver\n  format: \"YYYY.SS.PATCH\"\n  sprint: 3 # current sprint\n"
	got, data, err := bumpSprint(t, src)
	require.NoError(t, err)
	assert.Equal(t, 4, got)
	assert.Contains(t, data, "  sprint: 4 # current sprint\n")
}

func TestIncrementSprint_ToleratesACommentOnTheVersioningLine(t *testing.T) {
	t.Run("replace", func(t *testing.T) {
		src := "version: \"1\"\nversioning: # how versions are cut\n  strategy: calver\n  format: \"YYYY.SS.PATCH\"\n  sprint: 3\n"
		got, data, err := bumpSprint(t, src)
		require.NoError(t, err)
		assert.Equal(t, 4, got)
		assert.Contains(t, data, "versioning: # how versions are cut\n")
		assert.Contains(t, data, "  sprint: 4\n")
	})
	t.Run("insert", func(t *testing.T) {
		src := "version: \"1\"\nversioning: # how versions are cut\n  strategy: calver\n  format: \"YYYY.SS.PATCH\"\n"
		got, data, err := bumpSprint(t, src)
		require.NoError(t, err)
		assert.Equal(t, 1, got)
		assert.Contains(t, data, "versioning: # how versions are cut\n  sprint: 1\n  strategy: calver\n")
	})
}

func TestIncrementSprint_InsertUsesTheBlocksIndentation(t *testing.T) {
	src := "version: \"1\"\nversioning:\n    strategy: calver\n    format: \"YYYY.SS.PATCH\"\n"
	_, data, err := bumpSprint(t, src)
	require.NoError(t, err)
	assert.Contains(t, data, "versioning:\n    sprint: 1\n    strategy: calver\n")
}

func TestIncrementSprint_FlowStyleVersioningIsRefusedNotMangled(t *testing.T) {
	src := "version: \"1\"\nversioning: {strategy: calver, format: \"YYYY.SS.PATCH\"}\n"
	_, data, err := bumpSprint(t, src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "block")
	assert.Equal(t, src, data, "the file must be left untouched")
}
