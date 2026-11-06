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
