package cmd

import (
	"fmt"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
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
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			if err := app.CheckBranch(runner, cfg, env, force); err != nil {
				return exitcode.Wrap(exitcode.Runtime, err)
			}

			resolver, err := app.NewResolver(cfg, env, force, "", "", runner)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			result, err := resolver.Resolve()
			if err != nil {
				return wrapRunErr(err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.Tag)
			return nil
		},
	}
}

func newVersionCurrentCmd() *cobra.Command {
	var bare bool
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Print the latest released tag",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			env, _ := cmd.Flags().GetString("env")
			force, _ := cmd.Flags().GetBool("force")

			runner := execadapter.New(false, verbose)
			path := config.ResolvePath(cfgPath)

			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			if err := app.CheckBranch(runner, cfg, env, force); err != nil {
				return exitcode.Wrap(exitcode.Runtime, err)
			}

			out := app.CurrentTag
			if bare {
				out = app.CurrentVersion
			}
			value, err := out(runner, cfg, env)
			if err != nil {
				return exitcode.Wrap(exitcode.Runtime, err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	}
	cmd.Flags().BoolVar(&bare, "bare", false, "print the bare semantic version (strip prefix/env/build), not the raw tag")
	return cmd
}
