package cmd

import (
	"fmt"

	"github.com/adaouat/heraut/internal/config"
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
	return &cobra.Command{
		Use:   "bump",
		Short: "Increment versioning.sprint in .heraut.yml",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			path := config.ResolvePath(cfgPath)

			newSprint, err := config.IncrementSprint(path)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "sprint bumped to %d\n", newSprint)
			return nil
		},
	}
}
