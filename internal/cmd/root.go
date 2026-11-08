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
		Example: "  # Write a config, check it, rehearse, release\n" +
			"  heraut init\n" +
			"  heraut check\n" +
			"  heraut release --dry-run\n" +
			"  heraut release",
		Long: ui.HelpLong(),
	}
	root.SetVersionTemplate(ui.VersionTemplate())

	pf := root.PersistentFlags()
	pf.String("config", "", "path to .heraut.yml (default: auto-discover)")
	pf.Bool("verbose", false, "echo each command and emit diagnostic logs")

	root.AddCommand(NewReleaseCmd(version))
	root.AddCommand(NewChangelogCmd(version))
	root.AddCommand(NewNotesCmd())
	root.AddCommand(NewCheckCmd())
	root.AddCommand(NewVersionCmd())
	root.AddCommand(NewCommitCmd())
	root.AddCommand(NewInitCmd(version))
	root.AddCommand(NewSkillCmd())
	whatsNew := updatecheck.WhatsNewCommand(updatecheck.WhatsNewConfig{
		Repo:      "adaouat/heraut",
		Current:   version,
		CacheFile: updatecheck.CacheFile("heraut"),
		Changelog: heraut.Changelog,
	})
	whatsNew.Long = "Show the release notes of every heraut version newer than the one running. When GitHub can't be\n" +
		"reached it reads a local cache, then the changelog built into the binary, so it works offline too."
	whatsNew.Example = "  heraut whatsnew"
	root.AddCommand(whatsNew)

	// After each command, print a one-line update hint if a newer release exists
	// (cached 24h, errors swallowed). Skipped for dev builds, via opt-out, and where heraut promises
	// no network or machine output (withUpdateHint).
	root.PersistentPostRunE = withUpdateHint(updatecheck.Hinter{
		Repo:      "adaouat/heraut",
		Bin:       "heraut",
		Module:    "github.com/adaouat/heraut/cmd/heraut",
		Current:   version,
		CacheFile: updatecheck.CacheFile("heraut"),
		OptOutEnv: "HERAUT_CHECK_UPDATE",
	}.PostRun())

	return root
}

// withUpdateHint runs hint after a command, except on --dry-run, --offline and --json runs and on
// heraut check config: the hint may call GitHub, and those runs promise no network (a dry run, offline,
// a config-only check) or output a script reads.
func withUpdateHint(hint func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		for _, name := range []string{"dry-run", "offline", "json"} {
			if f := cmd.Flags().Lookup(name); f != nil && f.Value.String() == "true" {
				return nil
			}
		}
		if cmd.CommandPath() == cmd.Root().Name()+" check config" {
			return nil
		}
		return hint(cmd, args)
	}
}
