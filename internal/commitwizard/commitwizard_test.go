package commitwizard

import (
	"bytes"
	"io"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
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

func alwaysConfirm(_ io.Writer, _ string) (bool, error) { return true, nil }
func neverConfirm(_ io.Writer, _ string) (bool, error)  { return false, nil }

func TestFinalize_CommitsOnConfirm(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git commit
	var out bytes.Buffer
	err := finalize(mr, nil, Answers{Type: "feat", Subject: "x"},
		Options{Out: &out}, alwaysConfirm)
	require.NoError(t, err)
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "commit", mr.Calls[0].Args[0])
}

func TestFinalize_CancelOnDecline(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer
	err := finalize(mr, nil, Answers{Type: "feat", Subject: "x"},
		Options{Out: &out}, neverConfirm)
	require.NoError(t, err)
	assert.Empty(t, mr.Calls, "no git commit on decline")
	assert.Contains(t, out.String(), "feat: x")
}

func TestFinalize_DryRunPrintsAndSkipsCommit(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer
	err := finalize(mr, nil, Answers{Type: "feat", Subject: "x"},
		Options{DryRun: true, Out: &out}, alwaysConfirm)
	require.NoError(t, err)
	assert.Empty(t, mr.Calls, "no git commit in dry-run")
	assert.Contains(t, out.String(), "feat: x")
	assert.Contains(t, out.String(), "dry-run")
}

func TestFinalize_GuardBlocksInvalidType(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer
	// "wip" is not in DefaultCommitTypes → VerifyCommit fails → no commit.
	err := finalize(mr, nil, Answers{Type: "wip", Subject: "x"},
		Options{Out: &out}, alwaysConfirm)
	require.Error(t, err)
	assert.Empty(t, mr.Calls)
}

func TestRun_NonTTYErrors(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer // a *bytes.Buffer is never a TTY
	err := Run(mr, nil, Options{Out: &out})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interactive terminal")
	assert.Empty(t, mr.Calls, "no git calls when there is no TTY")
}

func TestTypeOptionLabel(t *testing.T) {
	assert.Equal(t, "feat    A new feature", typeOptionLabel("feat"))
	assert.Equal(t, "custom", typeOptionLabel("custom")) // unknown type → bare label
}
