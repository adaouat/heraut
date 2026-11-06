package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/cmd"
)

func referenceTree() *cobra.Command {
	root := &cobra.Command{Use: "tool", Short: "Do things", Long: "BANNER", Example: "  tool run"}
	root.PersistentFlags().String("config", "", "path to the config")

	run := &cobra.Command{
		Use:     "run <target>",
		Short:   "Run a target",
		Long:    "Run the target.\nIt runs once in <dir>.",
		Example: "  # Run it\n  tool run app",
		RunE:    func(*cobra.Command, []string) error { return nil },
	}
	run.Flags().Bool("dry-run", false, "print actions | change nothing")
	run.Flags().StringP("dir", "d", ".cache", "where to run <path>")
	run.Flags().StringSlice("skip", nil, "steps to skip")
	run.Flags().Bool("secret", false, "hidden flag")
	_ = run.Flags().MarkHidden("secret")

	sub := &cobra.Command{Use: "sub", Short: "Sub <x>", Long: "Sub long.", Example: "  tool run sub", RunE: run.RunE}
	run.AddCommand(sub)
	root.AddCommand(run)
	root.AddCommand(&cobra.Command{Use: "ghost", Short: "hidden", Hidden: true, RunE: run.RunE})
	return root
}

func readPage(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return string(b)
}

func TestWriteCommandReference_Pages(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, cmd.WriteCommandReference(referenceTree(), dir))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{"README.md", "tool.md", "tool_run.md", "tool_run_sub.md"}, names,
		"one page per visible command, plus the index")

	page := readPage(t, dir, "tool_run.md")
	for _, want := range []string{
		"# tool run\n\nRun a target.\n\nRun the target.\nIt runs once in \\<dir\\>.\n",
		"## Usage\n\n```text\ntool run <target> [flags]\n```\n",
		"## Examples\n\n```bash\n# Run it\ntool run app\n```\n",
		"| `--dry-run` |  | print actions \\| change nothing |",
		"| `-d`, `--dir <string>` | `.cache` | where to run \\<path\\> |",
		"| `--skip <strings>` |  | steps to skip |",
		"## Global flags",
		"| `--config <string>` |  | path to the config |",
		"## Subcommands\n\n- [tool run sub](tool_run_sub.md): Sub \\<x\\>\n",
		"- [tool](tool.md): Do things",
		"[Exit codes](../exit-codes.md)",
		"[Environment variables](../environment-variables.md)",
	} {
		assert.Contains(t, page, want)
	}
	assert.NotContains(t, page, "secret", "hidden flags stay out")
	assert.NotContains(t, page, "ghost")

	rootPage := readPage(t, dir, "tool.md")
	assert.NotContains(t, rootPage, "BANNER", "the root's Long is the terminal banner, not text for a page")
	assert.Contains(t, rootPage, "# tool\n\nDo things.\n")

	index := readPage(t, dir, "README.md")
	for _, want := range []string{"- [tool](tool.md): Do things", "- [tool run](tool_run.md): Run a target", "- [tool run sub](tool_run_sub.md): Sub \\<x\\>"} {
		assert.Contains(t, index, want)
	}
	assert.NotContains(t, index, "ghost")
}

// The committed pages match what the commands say today; `mise run docs:gen` rewrites them.
func TestCommandReference_IsCurrent(t *testing.T) {
	want := t.TempDir()
	require.NoError(t, cmd.WriteCommandReference(cmd.NewRootCmd("dev"), want))

	committed := "../../docs/reference/commands"
	wantEntries, err := os.ReadDir(want)
	require.NoError(t, err)
	gotEntries, err := os.ReadDir(committed)
	require.NoError(t, err, "docs/reference/commands is missing: run mise run docs:gen")

	var wantNames, gotNames []string
	for _, e := range wantEntries {
		wantNames = append(wantNames, e.Name())
	}
	for _, e := range gotEntries {
		gotNames = append(gotNames, e.Name())
	}
	require.Equal(t, wantNames, gotNames, "the command pages differ from the commands: run mise run docs:gen")
	for _, name := range wantNames {
		assert.Equal(t, readPage(t, want, name), readPage(t, committed, name), "%s is stale: run mise run docs:gen", name)
	}
}
