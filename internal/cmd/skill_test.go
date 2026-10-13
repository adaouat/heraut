package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/skill"
)

func TestSkillPrint_WritesTheEmbeddedSkill(t *testing.T) {
	out, err := executeRoot("skill", "print")

	require.NoError(t, err)
	assert.Equal(t, skill.Content(), out)
}

func TestSkillInstall(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "heraut", "SKILL.md")

	out, err := executeRoot("skill", "install", "--dir", dir)
	require.NoError(t, err)
	assert.Contains(t, out, target)
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, skill.Content(), string(got))

	_, err = executeRoot("skill", "install", "--dir", dir)
	require.NoError(t, err, "installing the same content again is a no-op")

	require.NoError(t, os.WriteFile(target, []byte("my edits"), 0o644))
	_, err = executeRoot("skill", "install", "--dir", dir)
	require.Error(t, err, "a changed file is not replaced silently")
	assert.Contains(t, err.Error(), "--force")
	got, _ = os.ReadFile(target)
	assert.Equal(t, "my edits", string(got))

	_, err = executeRoot("skill", "install", "--dir", dir, "--force")
	require.NoError(t, err)
	got, _ = os.ReadFile(target)
	assert.Equal(t, skill.Content(), string(got))
}

// TestSkill_NamesEveryCommand keeps the skill from drifting: each runnable command of the CLI
// appears in it by its full path.
func TestSkill_NamesEveryCommand(t *testing.T) {
	content := skill.Content()
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			if sub.Runnable() {
				path := strings.TrimPrefix(sub.CommandPath(), "heraut ")
				assert.Contains(t, content, "heraut "+path, "the skill does not mention `heraut %s`", path)
			}
			walk(sub)
		}
	}
	walk(cmd.NewRootCmd("dev"))
}
