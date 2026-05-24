package cmd

import (
	"fmt"

	execadapter "github.com/adaouat/heraut/internal/adapter/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/spf13/cobra"
)

// NewVersionCmd constructs the `heraut version` parent command and its subcommands.
func NewVersionCmd() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
	}
	versionCmd.AddCommand(newVersionNextCmd())
	versionCmd.AddCommand(newVersionCurrentCmd())
	versionCmd.AddCommand(newVersionSprintCmd())
	return versionCmd
}

func newVersionNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Compute and print the next version without side effects",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			env, _ := cmd.Flags().GetString("env")
			force, _ := cmd.Flags().GetBool("force")

			runner := execadapter.New(false, verbose)
			path := config.ResolvePath(cfgPath)

			cfg, err := config.Load(path)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			resolver, err := app.NewResolver(cfg, env, force, "", runner)
			if err != nil {
				return err
			}

			result, err := resolver.Resolve()
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), result.Tag)
			return nil
		},
	}
}

func newVersionCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print the latest released tag",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			env, _ := cmd.Flags().GetString("env")

			runner := execadapter.New(false, verbose)
			path := config.ResolvePath(cfgPath)

			cfg, err := config.Load(path)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			tag, err := app.CurrentTag(runner, cfg, env)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), tag)
			return nil
		},
	}
}
