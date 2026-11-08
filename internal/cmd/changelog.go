package cmd

import (
	"fmt"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/spf13/cobra"
)

// NewChangelogCmd constructs the `heraut changelog` command.
func NewChangelogCmd(version string) *cobra.Command {
	var (
		commit          bool
		tag             bool
		noPush          bool
		versionOverride string
		buildID         string
		regenerate      bool
		noHooks         bool
		skipHooks       []string
	)

	changelogCmd := &cobra.Command{
		Use:   "changelog",
		Short: "Generate changelog (optionally commit and tag)",
		Long: "Write the changelog for the next version without publishing anything. On its own it only updates\n" +
			"the changelog file; --commit also commits and pushes it, --tag also creates and pushes the tag.\n" +
			"Use it for projects that tag but publish no forge release, or to read the changelog before releasing.",
		Example: "  # Update CHANGELOG.md, commit nothing\n" +
			"  heraut changelog\n" +
			"\n" +
			"  # Commit, tag and push, without a forge release\n" +
			"  heraut changelog --tag\n" +
			"\n" +
			"  # Commit and tag locally only\n" +
			"  heraut changelog --tag --no-push",
		RunE: func(cmd *cobra.Command, args []string) error {
			skipHookPoints, err := resolveSkipHooks(cmd, skipHooks, noHooks, app.ChangelogHookPoints())
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			flags := newFlagReader(cmd)
			cfgPath := flags.String("config")
			dryRun := flags.Bool("dry-run")
			verbose := flags.Bool("verbose")
			env := flags.String("env")
			force := flags.Bool("force")
			allowMajor := flags.Bool("allow-major")
			if err := flags.Err(); err != nil {
				return err
			}

			if err := validateVersionOverrideFlags(versionOverride, buildID); err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			runner := execadapter.New(dryRun, verbose)
			// Resolver only performs read-only git calls; use a real runner so
			// dry-run still shows the correct resolved version.
			readRunner := execadapter.New(false, verbose)
			// interactiveRunner connects stdin/stdout/stderr directly to the terminal instead of
			// capturing them, so `git commit`/a signed tag can prompt through GPG's pinentry
			// instead of hanging until it times out (T260, needs forge v0.19.0+).
			interactiveRunner := execadapter.New(dryRun, verbose)
			interactiveRunner.Interactive = true
			path := config.ResolvePath(cfgPath)

			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}
			if err := applyOfflineOverride(cmd, cfg); err != nil {
				return err
			}

			if errs := config.Validate(cfg); len(errs) > 0 {
				printConfigErrors(errs, cmd.ErrOrStderr())
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("configuration is invalid"))
			}

			if err := app.CheckFullHistory(readRunner); err != nil {
				return exitcode.Wrap(exitcode.Runtime, err)
			}

			env, err = app.ResolveEnv(env, cfg, readRunner)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			resolverOpts := []app.ResolverOption{app.WithAllowMajor(allowMajor)}
			if !tag {
				resolverOpts = append(resolverOpts, app.WithoutCollisionGuard())
			}
			resolver, err := app.NewResolver(cfg, env, force, versionOverride, buildID, readRunner, resolverOpts...)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			opts := app.PipelineOpts{
				DryRun:              dryRun,
				VersionOverride:     versionOverride,
				Env:                 env,
				Out:                 cmd.OutOrStdout(),
				Commit:              commit,
				Tag:                 tag,
				NoPush:              noPush,
				SignTags:            app.ReadGPGSign(readRunner),
				HerautVersion:       version,
				RegenerateChangelog: regenerate,
				ReadRunner:          readRunner,
				InteractiveRunner:   interactiveRunner,
				NoHooks:             noHooks,
				SkipHooks:           skipHookPoints,
			}
			if !dryRun {
				if err := app.CheckBranch(readRunner, cfg, env, force); err != nil {
					return wrapBranchErr(err)
				}
				if tag {
					if err := app.CheckReleaseBranch(readRunner, cfg, force); err != nil {
						return exitcode.Wrap(exitcode.Config, err)
					}
				}
				if err := app.PreflightCheck(runner); err != nil {
					return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("preflight check failed: %w", err))
				}
			}

			pipe, err := app.BuildChangelogPipeline(runner, cfg, resolver, opts)
			if err != nil {
				return wrapBuildErr(err)
			}

			return wrapRunErr(pipe.Run(cmd.Context()), "changelog failed")
		},
	}

	changelogCmd.Flags().BoolVar(&commit, "commit", false, "commit the generated changelog")
	changelogCmd.Flags().BoolVar(&tag, "tag", false, "tag after commit (implies --commit)")
	changelogCmd.Flags().BoolVar(&noPush, "no-push", false, "commit and tag locally without pushing (only meaningful with --commit/--tag)")
	addVersionOverrideFlags(changelogCmd, &versionOverride, &buildID)
	changelogCmd.Flags().BoolVar(&regenerate, "regenerate", false,
		"rebuild the entire changelog and re-fetch PR attribution (instead of incrementally adding the new section)")
	changelogCmd.Flags().Bool("dry-run", false, "print actions without executing them")
	changelogCmd.Flags().String("env", "", "target environment (for per-env strategies)")
	changelogCmd.Flags().Bool("force", false, "release from here: lift the branch checks and E002 (destination ahead); E001 and missing PR data are never forced")
	changelogCmd.Flags().Bool("allow-major", false, "confirm a major bump for this run: lift versioning.bump.stay_at_v0 and versioning.bump.confirm_major")
	changelogCmd.Flags().Bool("offline", false, "skip remote PR/MR metadata enrichment (forces enrichment_policy: disabled); outside CI, commit links go too")
	changelogCmd.Flags().BoolVar(&noHooks, "no-hooks", false, "skip every configured hook (post_bump/pre_changelog/pre_tag/post_tag) for this run")
	addSkipHookFlag(changelogCmd, &skipHooks, app.ChangelogHookPoints())

	return changelogCmd
}
