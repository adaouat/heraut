package cmd

import (
	"github.com/spf13/cobra"

	"github.com/adaouat/forge/updatecheck"
	"github.com/adaouat/heraut"
	"github.com/adaouat/heraut/internal/ui"
)

// NewRootCmd constructs the root heraut command.
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "heraut",
		Short: "Release management for git-based projects",
		Long:  ui.HelpLong(),
	}
	root.SetVersionTemplate(ui.VersionTemplate())

	pf := root.PersistentFlags()
	pf.String("config", "", "path to .heraut.yml (default: auto-discover)")
	pf.Bool("dry-run", false, "print actions without executing them")
	pf.Bool("verbose", false, "echo each command and emit diagnostic logs")
	pf.String("env", "", "target environment (for per-env strategies)")
	pf.Bool("force", false, "bypass E001/E002 promotion errors")
	pf.Bool("offline", false, "skip remote PR/MR metadata enrichment (forces remote_metadata: disabled)")

	root.AddCommand(NewReleaseCmd(version))
	root.AddCommand(NewChangelogCmd(version))
	root.AddCommand(NewCheckCmd())
	root.AddCommand(NewCliffCmd())
	root.AddCommand(NewVersionCmd())
	root.AddCommand(NewCommitCmd())
	root.AddCommand(NewInitCmd(version))
	root.AddCommand(updatecheck.WhatsNewCommand(updatecheck.WhatsNewConfig{
		Repo:      "adaouat/heraut",
		Current:   version,
		CacheFile: updatecheck.CacheFile("heraut"),
		Changelog: heraut.Changelog,
	}))

	// After each command, print a one-line update hint if a newer release exists
	// (cached 24h, errors swallowed). Skipped for dev builds and via opt-out.
	root.PersistentPostRunE = updatecheck.Hinter{
		Repo:      "adaouat/heraut",
		Bin:       "heraut",
		Module:    "github.com/adaouat/heraut/cmd/heraut",
		Current:   version,
		CacheFile: updatecheck.CacheFile("heraut"),
		OptOutEnv: "HERAUT_CHECK_UPDATE",
	}.PostRun()

	return root
}
