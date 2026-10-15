package cmd

import (
	"errors"
	"fmt"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/ui"
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
	var versionOverride, buildID string

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Compute and print the next version without side effects",
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			next, err := resolveNext(cmd, versionOverride, buildID)
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), next.data(), next.warnings, err)
			}
			if err != nil {
				return err
			}
			for _, w := range next.warnings {
				ui.WarnLines(cmd.ErrOrStderr(), w)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), next.Tag)
			return nil
		},
	}
	cmd.Flags().String("env", "", "target environment (for per-env strategies)")
	cmd.Flags().Bool("json", false, "print the result as a JSON envelope")
	cmd.Flags().Bool("force", false, "override safety checks blocking tag promotion or missing PR/MR metadata")
	cmd.Flags().Bool("allow-major", false, "lift versioning.bump.stay_at_v0 and the pre-release series major-escalation block for this run")
	cmd.Flags().String("pre-release", "", "mint a pre-release <core>-<label>.<N> (plain semver only; never writes CHANGELOG.md)")
	addVersionOverrideFlags(cmd, &versionOverride, &buildID)
	return cmd
}

// nextResult is what `version next` resolved; Tag is the printed value, data() the JSON object.
type nextResult struct {
	Tag, Version, CurrentTag, bump string
	warnings                       []string
}

func (n nextResult) data() any {
	return struct {
		Tag        string `json:"tag"`
		Version    string `json:"version"`
		CurrentTag string `json:"current_tag,omitempty"`
		Bump       string `json:"bump,omitempty"`
	}{n.Tag, n.Version, n.CurrentTag, n.bump}
}

func resolveNext(cmd *cobra.Command, versionOverride, buildID string) (nextResult, error) {
	if err := validateVersionOverrideFlags(versionOverride, buildID); err != nil {
		return nextResult{}, exitcode.Wrap(exitcode.Config, err)
	}

	cfgPath, _ := cmd.Flags().GetString("config")
	verbose, _ := cmd.Flags().GetBool("verbose")
	env, _ := cmd.Flags().GetString("env")
	force, _ := cmd.Flags().GetBool("force")
	allowMajor, _ := cmd.Flags().GetBool("allow-major")
	preRelease, _ := cmd.Flags().GetString("pre-release")

	runner := execadapter.New(false, verbose)
	cfg, err := loadValidatedConfig(cmd, config.ResolvePath(cfgPath))
	if err != nil {
		return nextResult{}, err
	}

	env, err = app.ResolveEnv(env, cfg, runner)
	if err != nil {
		return nextResult{}, exitcode.Wrap(exitcode.Config, err)
	}

	if err := app.CheckBranch(runner, cfg, env, force); err != nil {
		return nextResult{}, wrapBranchErr(err)
	}

	resolver, err := app.NewResolver(cfg, env, force, versionOverride, buildID, runner, app.WithAllowMajor(allowMajor), app.WithPreRelease(preRelease), app.WithBranchesOnlyCollisionGuard())
	if err != nil {
		return nextResult{}, exitcode.Wrap(exitcode.Config, err)
	}

	result, err := resolver.Resolve()
	if err != nil {
		return nextResult{}, wrapRunErr(err)
	}
	next := nextResult{Tag: result.Tag, Version: result.Version, CurrentTag: result.CurrentTag, warnings: result.Warnings}
	if result.Bump != 0 {
		next.bump = result.Bump.String()
	}
	return next, nil
}

// loadValidatedConfig loads and semantically validates the config, printing the findings to stderr
// and returning a Config-class error when there are any.
func loadValidatedConfig(cmd *cobra.Command, path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.Config, err)
	}
	if errs := config.Validate(cfg); len(errs) > 0 {
		printConfigErrors(errs, cmd.ErrOrStderr())
		return nil, exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", len(errs)))
	}
	return cfg, nil
}

func newVersionCurrentCmd() *cobra.Command {
	var bare, includePreRelease bool
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Print the latest released tag",
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			tag, version, err := resolveCurrent(cmd, asJSON || !bare, asJSON || bare, includePreRelease)
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), currentData{Tag: tag, Version: version}, nil, err)
			}
			if err != nil {
				return err
			}
			value := tag
			if bare {
				value = version
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	}
	cmd.Flags().BoolVar(&bare, "bare", false, "print the bare semantic version (strip prefix/env/build), not the raw tag")
	cmd.Flags().BoolVar(&includePreRelease, "include-pre-release", false, "print the highest SemVer tag including pre-releases (SemVer strategies only; default: latest release)")
	cmd.Flags().String("env", "", "target environment (for per-env strategies)")
	cmd.Flags().Bool("json", false, "print the result as a JSON envelope (tag and version)")
	cmd.Flags().Bool("force", false, "override safety checks blocking tag promotion or missing PR/MR metadata")
	return cmd
}

type currentData struct {
	Tag     string `json:"tag"`
	Version string `json:"version"`
}

// resolveCurrent returns the latest released tag and its bare version, each only when asked for:
// every one is its own tag lookup, and the text mode wants exactly one of them.
func resolveCurrent(cmd *cobra.Command, withTag, withVersion, includePreRelease bool) (tag, version string, err error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	verbose, _ := cmd.Flags().GetBool("verbose")
	env, _ := cmd.Flags().GetString("env")
	force, _ := cmd.Flags().GetBool("force")

	runner := execadapter.New(false, verbose)
	cfg, err := loadValidatedConfig(cmd, config.ResolvePath(cfgPath))
	if err != nil {
		return "", "", err
	}

	env, err = app.ResolveEnv(env, cfg, runner)
	if err != nil {
		return "", "", exitcode.Wrap(exitcode.Config, err)
	}

	if err := app.CheckBranch(runner, cfg, env, force); err != nil {
		return "", "", wrapBranchErr(err)
	}

	classify := func(err error) error {
		if errors.Is(err, app.ErrAmbiguousBranch) || errors.Is(err, app.ErrUnderivableRange) || app.IsEnvSelection(err) {
			return exitcode.Wrap(exitcode.Config, err)
		}
		return exitcode.Wrap(exitcode.Runtime, err)
	}
	if withTag {
		if tag, err = app.CurrentTag(runner, cfg, env, includePreRelease); err != nil {
			return "", "", classify(err)
		}
	}
	if withVersion {
		if version, err = app.CurrentVersion(runner, cfg, env, includePreRelease); err != nil {
			return "", "", classify(err)
		}
	}
	return tag, version, nil
}
