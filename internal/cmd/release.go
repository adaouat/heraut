package cmd

import (
	"fmt"
	"io"

	execadapter "github.com/adaouat/forge/exec"
	forgelog "github.com/adaouat/forge/log"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/spf13/cobra"
)

// releaseFlags holds the values of the flags bound to variables rather than read by name.
type releaseFlags struct {
	versionOverride     string
	buildID             string
	regenerateChangelog bool
	noHooks             bool
	skipHooks           []string
}

// NewReleaseCmd constructs the `heraut release` command.
func NewReleaseCmd(version string) *cobra.Command {
	rf := &releaseFlags{}

	releaseCmd := &cobra.Command{
		Use:   "release",
		Short: "Resolve next version, generate changelog, tag, and publish",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := newFlagReader(cmd)
			asJSON := flags.Bool("json")
			if err := flags.Err(); err != nil {
				return err
			}
			err := runRelease(cmd, version, rf, asJSON)
			if asJSON && err != nil {
				return writeJSON(cmd.OutOrStdout(), nil, nil, err)
			}
			return err
		},
	}

	addVersionOverrideFlags(releaseCmd, &rf.versionOverride, &rf.buildID)
	releaseCmd.Flags().BoolVar(&rf.regenerateChangelog, "regenerate-changelog", false,
		"rebuild the entire changelog and re-fetch PR attribution (needed once when migrating an existing changelog onto heraut)")
	releaseCmd.Flags().Bool("dry-run", false, "print actions without executing them")
	releaseCmd.Flags().Bool("json", false, "with --dry-run, print the plan as a JSON envelope")
	releaseCmd.Flags().String("env", "", "target environment (for per-env strategies)")
	releaseCmd.Flags().Bool("force", false, "override safety checks blocking tag promotion or missing PR/MR metadata")
	releaseCmd.Flags().Bool("allow-major", false, "lift versioning.bump.stay_at_v0 and the pre-release series major-escalation block for this run")
	releaseCmd.Flags().String("pre-release", "", "mint a pre-release <core>-<label>.<N> (plain semver only; never writes CHANGELOG.md)")
	releaseCmd.Flags().Bool("offline", false, "skip remote PR/MR metadata enrichment (forces enrichment_policy: disabled)")
	releaseCmd.Flags().BoolVar(&rf.noHooks, "no-hooks", false, "skip every configured hook (post_bump/pre_changelog/pre_tag/post_tag/pre_release/post_release) for this run")
	addSkipHookFlag(releaseCmd, &rf.skipHooks, app.ReleaseHookPoints())

	return releaseCmd
}

// runRelease is the body of `heraut release`: flags, config, resolver, pipeline, then the plan or the run.
func runRelease(cmd *cobra.Command, version string, rf *releaseFlags, asJSON bool) error {
	if err := validateVersionOverrideFlags(rf.versionOverride, rf.buildID); err != nil {
		return exitcode.Wrap(exitcode.Config, err)
	}

	skipHookPoints, err := resolveSkipHooks(cmd, rf.skipHooks, rf.noHooks, app.ReleaseHookPoints())
	if err != nil {
		return exitcode.Wrap(exitcode.Config, err)
	}

	flags := newFlagReader(cmd)
	cfgPath := flags.String("config")
	dryRun := flags.Bool("dry-run")
	if err := flags.Err(); err != nil {
		return err
	}
	if asJSON && !dryRun {
		return exitcode.Wrap(exitcode.Usage, fmt.Errorf("--json describes the plan of a run that changes nothing: pass --dry-run as well"))
	}
	flags = newFlagReader(cmd)
	verbose := flags.Bool("verbose")
	env := flags.String("env")
	force := flags.Bool("force")
	allowMajor := flags.Bool("allow-major")
	preRelease := flags.String("pre-release")
	if err := flags.Err(); err != nil {
		return err
	}

	releaseOut := cmd.OutOrStdout()
	if asJSON {
		releaseOut = io.Discard
	}
	runner := execadapter.New(dryRun, verbose)
	// Resolver only performs read-only git calls (tag list, log); use a real
	// runner so dry-run still shows the correct resolved version. Memoized so
	// HasResolvablePublishTarget's pre-flight forge resolution and BuildPipeline's
	// own (internal) resolution — both read-only, both reached before any write —
	// share one `git remote get-url origin` subprocess instead of spawning two (T173).
	readRunner := app.NewMemoizingRunner(execadapter.New(false, verbose))
	// interactiveRunner connects stdin/stdout/stderr directly to the terminal instead of
	// capturing them, so `git commit`/a signed tag can prompt through GPG's pinentry
	// instead of hanging until it times out (T260, needs forge v0.19.0+).
	interactiveRunner := execadapter.New(dryRun, verbose)
	interactiveRunner.Interactive = true

	logger := forgelog.New(cmd.ErrOrStderr(), forgelog.LevelFor(verbose))

	path := config.ResolvePath(cfgPath)

	cfg, err := config.Load(path)
	if err != nil {
		return exitcode.Wrap(exitcode.Config, err)
	}
	logger.Debug("config resolved", "path", path)
	if err := applyOfflineOverride(cmd, cfg); err != nil {
		return err
	}

	if errs := config.Validate(cfg); len(errs) > 0 {
		printConfigErrors(errs, cmd.ErrOrStderr())
		return exitcode.Wrap(exitcode.Config, fmt.Errorf("configuration is invalid"))
	}

	env, err = app.ResolveEnv(env, cfg, readRunner)
	if err != nil {
		return exitcode.Wrap(exitcode.Config, err)
	}

	if !app.HasResolvablePublishTarget(readRunner, cfg, env) {
		return exitcode.Wrap(exitcode.Config, fmt.Errorf(
			"heraut release requires at least one resolvable publish destination — declare release.targets (with a forges: entry) or run where a forge auto-detects (CI env or git origin); use 'heraut changelog' for tag-only workflows",
		))
	}

	resolver, err := app.NewResolver(cfg, env, force, rf.versionOverride, rf.buildID, readRunner, app.WithAllowMajor(allowMajor), app.WithPreRelease(preRelease))
	if err != nil {
		return exitcode.Wrap(exitcode.Config, err)
	}

	opts := app.PipelineOpts{
		DryRun:              dryRun,
		Force:               force,
		VersionOverride:     rf.versionOverride,
		Env:                 env,
		Out:                 releaseOut,
		SignTags:            app.ReadGPGSign(readRunner),
		HerautVersion:       version,
		RegenerateChangelog: rf.regenerateChangelog,
		Logger:              logger,
		ReadRunner:          readRunner,
		InteractiveRunner:   interactiveRunner,
		NoHooks:             rf.noHooks,
		SkipHooks:           skipHookPoints,
		PreReleaseLabel:     preRelease,
	}
	pipe, err := app.BuildPipeline(runner, cfg, resolver, opts)
	if err != nil {
		return exitcode.Wrap(exitcode.Config, err)
	}

	if !dryRun {
		if err := app.CheckBranch(readRunner, cfg, env, force); err != nil {
			return wrapBranchErr(err)
		}
		if err := app.CheckReleaseBranch(readRunner, cfg, force); err != nil {
			return exitcode.Wrap(exitcode.Config, err)
		}
		if err := app.PreflightCheck(runner); err != nil {
			return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("preflight check failed: %w", err))
		}
		if err := pipe.Check(); err != nil {
			return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("preflight check failed: %w", err))
		}
	}

	if asJSON {
		plan, err := pipe.Plan()
		if err != nil {
			return wrapRunErr(err)
		}
		return writeJSON(cmd.OutOrStdout(), planData(plan), plan.Warnings, nil)
	}
	return wrapRunErr(pipe.Run(cmd.Context()), "release failed")
}

// planData is the JSON data of `release --dry-run --json`; steps keep the order a real run follows.
func planData(plan app.ReleasePlan) any {
	steps := plan.Steps
	if steps == nil {
		steps = []app.PlanStep{}
	}
	return struct {
		Tag        string         `json:"tag"`
		Version    string         `json:"version"`
		CurrentTag string         `json:"current_tag,omitempty"`
		PreRelease bool           `json:"pre_release"`
		Steps      []app.PlanStep `json:"steps"`
	}{plan.Tag, plan.Version, plan.CurrentTag, plan.PreRelease, steps}
}
