package cmd

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/selfupdate"
	"github.com/adaouat/heraut/internal/ui"
)

// NewRootCmd constructs the root heraut command. opts are forwarded to the
// self-update subsystem (e.g. selfupdate.WithLatestURL for tests).
func NewRootCmd(version string, opts ...selfupdate.Option) *cobra.Command {
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
	root.AddCommand(NewCliffCmd())
	root.AddCommand(NewVersionCmd())
	root.AddCommand(NewInitCmd(version))
	root.AddCommand(NewSelfUpdateCmd(version, opts...))

	root.PersistentPostRunE = func(c *cobra.Command, args []string) error {
		if c.Name() == "self-update" {
			return nil
		}
		if version == "dev" || os.Getenv("HERAUT_NO_UPDATE_CHECK") == "1" {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		selfupdate.New(version, opts...).Hint(ctx, c.ErrOrStderr())
		return nil
	}

	return root
}
