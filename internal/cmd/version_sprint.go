package cmd

import (
	"fmt"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/spf13/cobra"
)

func newVersionSprintCmd() *cobra.Command {
	sprintCmd := &cobra.Command{
		Use:   "sprint",
		Short: "Sprint counter management",
	}
	sprintCmd.AddCommand(newVersionSprintBumpCmd())
	return sprintCmd
}

func newVersionSprintBumpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bump",
		Short: "Increment versioning.sprint in .heraut.yml",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			path := config.ResolvePath(cfgPath)
			out := cmd.OutOrStdout()

			if dryRun {
				cfg, err := config.Load(path)
				if err != nil {
					return exitcode.Wrap(exitcode.Config, err)
				}
				newSprint := cfg.Versioning.Sprint + 1
				_, _ = fmt.Fprintf(out, "[dry-run] would bump sprint %d -> %d in %s\n", cfg.Versioning.Sprint, newSprint, path)
				return nil
			}

			newSprint, err := config.IncrementSprint(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			_, _ = fmt.Fprintln(out, ui.Success(out, fmt.Sprintf("sprint bumped to %d", newSprint)))
			return nil
		},
	}
	cmd.Flags().Bool("dry-run", false, "print actions without executing them")
	return cmd
}
