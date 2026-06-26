package commitwizard

import (
	"testing"

	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssemble(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		c := Assemble(Answers{Type: "feat", Subject: "add wizard"})
		assert.Equal(t, "feat: add wizard", c.Format())
	})
	t.Run("scope + body", func(t *testing.T) {
		c := Assemble(Answers{Type: "fix", Scope: "cmd", Subject: "x", Body: "why"})
		assert.Equal(t, "fix(cmd): x\n\nwhy", c.Format())
	})
	t.Run("breaking adds bang and BREAKING CHANGE footer first", func(t *testing.T) {
		c := Assemble(Answers{
			Type: "feat", Scope: "cmd", Subject: "drop flag",
			Breaking: true, BreakingDesc: "old removed",
			Footers: []conventionalcommit.Footer{{Token: "Closes", Value: "#42"}},
		})
		assert.True(t, c.Breaking)
		assert.Equal(t, "feat(cmd)!: drop flag\n\nBREAKING CHANGE: old removed\nCloses: #42", c.Format())
	})
	t.Run("breaking bang without description adds no footer", func(t *testing.T) {
		c := Assemble(Answers{Type: "feat", Subject: "x", Breaking: true})
		assert.Equal(t, "feat!: x", c.Format())
	})
}

func TestParseFooterLines(t *testing.T) {
	t.Run("parses and skips blanks", func(t *testing.T) {
		got, err := parseFooterLines("Closes: #42\n\nRefs: PROJ-1\n")
		require.NoError(t, err)
		assert.Equal(t, []conventionalcommit.Footer{
			{Token: "Closes", Value: "#42"},
			{Token: "Refs", Value: "PROJ-1"},
		}, got)
	})
	t.Run("rejects a non-footer line", func(t *testing.T) {
		_, err := parseFooterLines("Closes: #42\nnot a footer")
		require.Error(t, err)
	})
	t.Run("empty input yields nil", func(t *testing.T) {
		got, err := parseFooterLines("")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}
