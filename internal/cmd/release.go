package cmd

import (
	"fmt"
	"regexp"

	execadapter "github.com/adaouat/heraut/internal/adapter/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/spf13/cobra"
)

// versionPattern matches a valid semver version with or without "v" prefix.
var versionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)

// NewReleaseCmd constructs the `heraut release` command.
func NewReleaseCmd() *cobra.Command {
	var versionOverride string

	releaseCmd := &cobra.Command{
		Use:   "release",
		Short: "Resolve next version, generate changelog, tag, and publish",
		RunE: func(cmd *cobra.Command, args []string) error {
			if versionOverride != "" && !versionPattern.MatchString(versionOverride) {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf(
					"invalid --version %q: expected vMAJOR.MINOR.PATCH or MAJOR.MINOR.PATCH (e.g. v1.2.3)",
					versionOverride,
				))
			}

			// Read persistent flags from root
			cfgPath, _ := cmd.Flags().GetString("config")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			verbose, _ := cmd.Flags().GetBool("verbose")
			env, _ := cmd.Flags().GetString("env")
			force, _ := cmd.Flags().GetBool("force")

			runner := execadapter.New(dryRun, verbose)
			// Resolver only performs read-only git calls (tag list, log); use a real
			// runner so dry-run still shows the correct resolved version.
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

			if !hasEffectivePlatforms(cfg, env) {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf(
					"heraut release requires at least one entry in release.platforms — use 'heraut changelog' for tag-only workflows",
				))
			}

			resolver, err := app.NewResolver(cfg, env, force, versionOverride, "", readRunner)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			opts := app.PipelineOpts{
				DryRun:          dryRun,
				Force:           force,
				VersionOverride: versionOverride,
				Env:             env,
				Out:             cmd.OutOrStdout(),
				SignTags:        app.ReadGPGSign(readRunner),
			}
			pipe, err := app.BuildPipeline(runner, cfg, resolver, opts)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			if !dryRun {
				if err := app.CheckBranch(readRunner, cfg, env, force); err != nil {
					return exitcode.Wrap(exitcode.Runtime, err)
				}
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

	releaseCmd.Flags().StringVar(&versionOverride, "version", "", "override the resolved version — with or without tag prefix (e.g. 1.2.3 or v1.2.3)")

	return releaseCmd
}

// hasEffectivePlatforms reports whether cfg has at least one release platform
// after applying env overrides. Env platforms replace root platforms entirely.
func hasEffectivePlatforms(cfg *config.Config, env string) bool {
	if env != "" {
		if envCfg, ok := cfg.Environments[env]; ok && envCfg.Release != nil {
			return len(envCfg.Release.Platforms) > 0
		}
	}
	return cfg.Release != nil && len(cfg.Release.Platforms) > 0
}
