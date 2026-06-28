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
	t.Run("trims surrounding whitespace from subject and breaking description", func(t *testing.T) {
		c := Assemble(Answers{Type: "feat", Subject: "  add x  ", Breaking: true, BreakingDesc: "  gone  "})
		assert.Equal(t, "feat!: add x\n\nBREAKING CHANGE: gone", c.Format())
	})
	t.Run("breaking description of only whitespace adds no footer", func(t *testing.T) {
		c := Assemble(Answers{Type: "feat", Subject: "x", Breaking: true, BreakingDesc: "   "})
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

func TestFinalize_GuardRunsBeforeDryRun(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer
	// The guard must run before the dry-run branch: an invalid type fails even
	// in dry-run, rather than being printed as a "valid" message.
	err := finalize(mr, nil, Answers{Type: "wip", Subject: "x"},
		Options{DryRun: true, Out: &out}, alwaysConfirm)
	require.Error(t, err)
	assert.Empty(t, mr.Calls)
	assert.NotContains(t, out.String(), "dry-run", "guard should fail before the dry-run print")
}

func TestRun_NonTTYErrors(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer // a *bytes.Buffer is never a TTY
	err := Run(mr, nil, Options{Out: &out})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interactive terminal")
	assert.Empty(t, mr.Calls, "no git calls when there is no TTY")
}

func TestOptionLabel(t *testing.T) {
	assert.Equal(t, "custom", optionLabel("custom", ""), "no description → bare name")
	got := optionLabel("feat", "A new feature")
	assert.Contains(t, got, "feat")
	assert.Contains(t, got, "A new feature")
}

func TestResolveStaging(t *testing.T) {
	confirmYes := func() (bool, error) { return true, nil }
	confirmNo := func() (bool, error) { return false, nil }

	t.Run("already staged proceeds without prompting", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("internal/x.go\n", "", nil) // git diff --cached: staged
		var out bytes.Buffer
		called := false
		proceed, err := resolveStaging(mr, Options{Out: &out}, func() (bool, error) {
			called = true
			return true, nil
		})
		require.NoError(t, err)
		assert.True(t, proceed)
		assert.False(t, called, "must not prompt to stage when something is already staged")
		require.Len(t, mr.Calls, 1)
		assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[0].Args)
	})

	t.Run("clean working tree aborts without prompting", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil) // git diff --cached: nothing staged
		mr.QueueResponse("", "", nil) // git status --porcelain: clean
		var out bytes.Buffer
		called := false
		proceed, err := resolveStaging(mr, Options{Out: &out}, func() (bool, error) {
			called = true
			return true, nil
		})
		require.NoError(t, err)
		assert.False(t, proceed)
		assert.False(t, called, "must not prompt to stage when there is nothing to stage")
		assert.Contains(t, out.String(), "nothing to commit")
		assert.Equal(t, []string{"status", "--porcelain"}, mr.Calls[1].Args)
		for _, c := range mr.Calls {
			assert.NotEqual(t, []string{"add", "-A"}, c.Args, "must not stage on a clean tree")
		}
	})

	t.Run("dirty tree, user stages, proceeds", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil)                   // diff --cached: nothing staged
		mr.QueueResponse(" M internal/x.go\n", "", nil) // status --porcelain: dirty
		mr.QueueResponse("", "", nil)                   // add -A
		var out bytes.Buffer
		proceed, err := resolveStaging(mr, Options{Out: &out}, confirmYes)
		require.NoError(t, err)
		assert.True(t, proceed)
		require.Len(t, mr.Calls, 3)
		assert.Equal(t, []string{"add", "-A"}, mr.Calls[2].Args)
	})

	t.Run("dirty tree, user cancels, aborts", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil)            // diff --cached: nothing staged
		mr.QueueResponse("?? new.go\n", "", nil) // status --porcelain: untracked
		var out bytes.Buffer
		proceed, err := resolveStaging(mr, Options{Out: &out}, confirmNo)
		require.NoError(t, err)
		assert.False(t, proceed)
		assert.Contains(t, out.String(), "cancelled")
		for _, c := range mr.Calls {
			assert.NotEqual(t, []string{"add", "-A"}, c.Args, "must not stage when cancelled")
		}
	})
}
