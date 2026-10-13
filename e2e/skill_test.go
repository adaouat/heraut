package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestSkill(t *testing.T) {
	bin := harness.Binary(t)

	t.Run("print writes the skill to stdout and nothing else", func(t *testing.T) {
		repo := harness.NewRepo(t)

		res := repo.Run(bin, nil, "skill", "print")

		require.Equal(t, exitOK, res.ExitCode, res.Stderr)
		assert.True(t, strings.HasPrefix(res.Stdout, "---\nname: heraut\n"))
		assert.Contains(t, res.Stdout, "heraut release --dry-run --json")
	})

	t.Run("install writes into .claude/skills and a second run is a no-op", func(t *testing.T) {
		repo := harness.NewRepo(t)

		res := repo.Run(bin, nil, "skill", "install")
		require.Equal(t, exitOK, res.ExitCode, res.Stderr)
		skill := repo.ReadFile(filepath.Join(".claude", "skills", "heraut", "SKILL.md"))
		printed := repo.Run(bin, nil, "skill", "print")
		assert.Equal(t, printed.Stdout, skill)

		again := repo.Run(bin, nil, "skill", "install")
		require.Equal(t, exitOK, again.ExitCode)
		assert.Contains(t, again.Stdout, "up to date")
	})
}
