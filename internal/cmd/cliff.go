package cmd

import (
	"fmt"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/spf13/cobra"
)

// NewCliffCmd constructs the `heraut cliff` parent command.
func NewCliffCmd() *cobra.Command {
	cliffCmd := &cobra.Command{
		Use:   "cliff",
		Short: "Print the effective merged git-cliff TOML",
	}
	cliffCmd.AddCommand(newCliffChangelogCmd())
	cliffCmd.AddCommand(newCliffReleaseNotesCmd())
	return cliffCmd
}

func newCliffChangelogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "changelog",
		Short: "Print the effective git-cliff TOML for changelog mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			env, _ := cmd.Flags().GetString("env")
			path := config.ResolvePath(cfgPath)

			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			toml, err := app.EffectiveCliffConfig(cfg, cfg.Changelog, "changelog", env)
			if err != nil {
				return exitcode.Wrap(exitcode.Runtime, err)
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), toml)
			return nil
		},
	}
}

func newCliffReleaseNotesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "release-notes",
		Short: "Print the effective git-cliff TOML for release-notes mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			env, _ := cmd.Flags().GetString("env")
			path := config.ResolvePath(cfgPath)

			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			var notesDriver *config.ContentDriver
			if cfg.Release != nil {
				notesDriver = cfg.Release.Notes
			}

			toml, err := app.EffectiveCliffConfig(cfg, notesDriver, "release-notes", env)
			if err != nil {
				return exitcode.Wrap(exitcode.Runtime, err)
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), toml)
			return nil
		},
	}
}
