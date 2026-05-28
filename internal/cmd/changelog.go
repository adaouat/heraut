package cmd

import (
	"fmt"

	execadapter "github.com/adaouat/heraut/internal/adapter/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/spf13/cobra"
)

// NewChangelogCmd constructs the `heraut changelog` command.
func NewChangelogCmd() *cobra.Command {
	var (
		commit          bool
		tag             bool
		versionOverride string
	)

	changelogCmd := &cobra.Command{
		Use:   "changelog",
		Short: "Generate changelog (optionally commit and tag)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			verbose, _ := cmd.Flags().GetBool("verbose")
			env, _ := cmd.Flags().GetString("env")
			force, _ := cmd.Flags().GetBool("force")

			runner := execadapter.New(dryRun, verbose)
			// Resolver only performs read-only git calls; use a real runner so
			// dry-run still shows the correct resolved version.
			readRunner := execadapter.New(false, verbose)
			path := config.ResolvePath(cfgPath)

			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			if errs := config.Validate(cfg); len(errs) > 0 {
				printConfigErrors(errs, cmd.ErrOrStderr())
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("configuration is invalid"))
			}

			resolver, err := app.NewResolver(cfg, env, force, versionOverride, readRunner)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			opts := app.PipelineOpts{
				DryRun:   dryRun,
				Env:      env,
				Out:      cmd.OutOrStdout(),
				Commit:   commit,
				Tag:      tag,
				SignTags: app.ReadGPGSign(readRunner),
			}
			if !dryRun {
				if err := app.PreflightCheck(runner); err != nil {
					return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("preflight check failed: %w", err))
				}
			}

			pipe, err := app.BuildChangelogPipeline(runner, cfg, resolver, opts)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			return wrapRunErr(pipe.Run())
		},
	}

	changelogCmd.Flags().BoolVar(&commit, "commit", false, "commit the generated changelog")
	changelogCmd.Flags().BoolVar(&tag, "tag", false, "tag after commit (implies --commit)")
	changelogCmd.Flags().StringVar(&versionOverride, "version", "", "override the resolved version — with or without tag prefix (e.g. 1.2.3 or v1.2.3)")

	return changelogCmd
}
