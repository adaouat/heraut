package cmd

import (
	"fmt"
	"os"

	execadapter "github.com/adaouat/heraut/internal/adapter/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
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
				Env:             env,
				Out:             cmd.OutOrStdout(),
				Commit:          commit,
				Tag:             tag,
			}
			pipe, err := app.BuildChangelogPipeline(runner, cfg, resolver, opts)
			if err != nil {
				return err
			}

			return pipe.Run()
		},
	}

	changelogCmd.Flags().BoolVar(&commit, "commit", false, "commit the generated changelog")
	changelogCmd.Flags().BoolVar(&tag, "tag", false, "tag after commit (implies --commit)")
	changelogCmd.Flags().StringVar(&versionOverride, "version", "", "override the resolved version (e.g. 1.2.3)")

	return changelogCmd
}
