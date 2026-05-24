package cmd

import (
	"github.com/adaouat/heraut/internal/ui"
	"github.com/spf13/cobra"
)

// NewRootCmd constructs the root heraut command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "heraut",
		Short: "Release management for git-based projects",
		Long:  ui.HelpLong(),
	}
	root.SetVersionTemplate(ui.VersionTemplate())

	pf := root.PersistentFlags()
	pf.String("config", "", "path to .heraut.yml (default: auto-discover)")
	pf.Bool("dry-run", false, "print actions without executing them")
	pf.Bool("verbose", false, "log each command before executing it")
	pf.String("env", "", "target environment (for per-env strategies)")
	pf.Bool("force", false, "bypass E001/E002 promotion errors")

	root.AddCommand(NewReleaseCmd())
	root.AddCommand(NewChangelogCmd())
	root.AddCommand(NewCheckCmd())
	root.AddCommand(NewVersionCmd())

	return root
}
