package cmd

import (
	"fmt"

	execadapter "github.com/adaouat/heraut/internal/adapter/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/spf13/cobra"
)

// NewReleaseCmd constructs the `heraut release` command.
func NewReleaseCmd() *cobra.Command {
	var versionOverride string

	releaseCmd := &cobra.Command{
		Use:   "release",
		Short: "Resolve next version, generate changelog, tag, and publish",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read persistent flags from root
			cfgPath, _ := cmd.Flags().GetString("config")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			verbose, _ := cmd.Flags().GetBool("verbose")
			env, _ := cmd.Flags().GetString("env")
			force, _ := cmd.Flags().GetBool("force")

			runner := execadapter.New(dryRun, verbose)

			path := config.ResolvePath(cfgPath)

			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			if errs := config.Validate(cfg); len(errs) > 0 {
				printConfigErrors(errs, cmd.ErrOrStderr())
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("configuration is invalid"))
			}

			resolver, err := app.NewResolver(cfg, env, force, versionOverride, runner)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			opts := app.PipelineOpts{
				DryRun:          dryRun,
				Force:           force,
				VersionOverride: versionOverride,
				Env:             env,
				Out:             cmd.OutOrStdout(),
			}
			pipe, err := app.BuildPipeline(runner, cfg, resolver, opts)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			if !dryRun {
				if err := app.PreflightCheck(runner); err != nil {
					return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("preflight check failed: %w", err))
				}
				if err := pipe.Check(); err != nil {
					return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("preflight check failed: %w", err))
				}
			}

			return wrapRunErr(pipe.Run())
		},
	}

	releaseCmd.Flags().StringVar(&versionOverride, "version", "", "override the resolved version (e.g. 1.2.3)")

	return releaseCmd
}
