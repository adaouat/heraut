package cmd

import (
	"fmt"
	"os"

	execadapter "github.com/adaouat/heraut/internal/adapter/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
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
				return fmt.Errorf("loading config: %w", err)
			}

			if errs := config.Validate(cfg); len(errs) > 0 {
				for _, e := range errs {
					_, _ = fmt.Fprintf(os.Stderr, "config error [%s]: %s\n", e.Path, e.Message)
					if e.Hint != "" {
						_, _ = fmt.Fprintf(os.Stderr, "  hint: %s\n", e.Hint)
					}
				}
				return fmt.Errorf("configuration is invalid")
			}

			resolver, err := app.NewResolver(cfg, env, force, versionOverride, runner)
			if err != nil {
				return err
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
				return err
			}

			if !dryRun {
				if err := app.PreflightCheck(runner); err != nil {
					return fmt.Errorf("preflight check failed: %w", err)
				}
				if err := pipe.Check(); err != nil {
					return fmt.Errorf("preflight check failed: %w", err)
				}
			}

			return pipe.Run()
		},
	}

	releaseCmd.Flags().StringVar(&versionOverride, "version", "", "override the resolved version (e.g. 1.2.3)")

	return releaseCmd
}
