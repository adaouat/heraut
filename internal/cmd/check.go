package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	execadapter "github.com/adaouat/heraut/internal/adapter/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/spf13/cobra"
)

// NewCheckCmd constructs the `heraut check` parent command.
// When called without a subcommand it runs config + runtime + cliff checks.
func NewCheckCmd() *cobra.Command {
	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Run preflight validations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			runner := execadapter.New(false, verbose)
			out := cmd.OutOrStdout()

			path, source := config.ResolvePathWithSource(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			var failed int

			// Config section
			ui.Header(out, "Config")
			_, _ = fmt.Fprintln(out, ui.Info(out, fmt.Sprintf("%s  (from %s)", path, source)))
			if errs := config.Validate(cfg); len(errs) > 0 {
				printConfigErrors(errs, out)
				failed += len(errs)
			} else {
				_, _ = fmt.Fprintln(out, ui.Success(out, "config: ok"))
			}

			// Runtime section (Git / Platforms / Generators — headers emitted by RuntimeCheck)
			failed += runRuntimeCheck(runner, cfg, out)

			// Cliff section (best-effort; skip if no git-cliff generators configured)
			ui.Header(out, "Cliff")
			if f := runCliffChecks(runner, cfg, out); f {
				failed++
			}

			// Summary
			_, _ = fmt.Fprintln(out)
			if failed > 0 {
				_, _ = fmt.Fprintln(out, ui.Err(out, fmt.Sprintf("%d check(s) failed — fix the issues above before running heraut release", failed)))
				code := exitcode.Runtime
				return exitcode.Wrap(code, fmt.Errorf("one or more checks failed"))
			}
			_, _ = fmt.Fprintln(out, ui.Success(out, "all checks passed"))
			return nil
		},
	}

	checkCmd.AddCommand(newCheckConfigCmd())
	checkCmd.AddCommand(newCheckRuntimeCmd())
	checkCmd.AddCommand(newCheckCliffCmd())

	return checkCmd
}

func newCheckConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Validate .heraut.yml (offline)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			out := cmd.OutOrStdout()

			path, source := config.ResolvePathWithSource(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			_, _ = fmt.Fprintln(out, ui.Info(out, fmt.Sprintf("%s  (from %s)", path, source)))
			errs := config.Validate(cfg)
			if len(errs) > 0 {
				printConfigErrors(errs, out)
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", len(errs)))
			}
			_, _ = fmt.Fprintln(out, ui.Success(out, "config: ok"))
			return nil
		},
	}
}

func newCheckRuntimeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "runtime",
		Short: "Check binaries on PATH, token env vars, and git user config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			runner := execadapter.New(false, verbose)
			out := cmd.OutOrStdout()

			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
				}
				_, _ = fmt.Fprintln(out, ui.Warn(out, fmt.Sprintf("no config found at %s — all tools checked as required", path)))
				cfg = nil
			}

			if failed := runRuntimeCheck(runner, cfg, out); failed > 0 {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, ui.Err(out, fmt.Sprintf("%d check(s) failed", failed)))
				return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("one or more runtime checks failed"))
			}
			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintln(out, ui.Success(out, "all checks passed"))
			return nil
		},
	}
}

func newCheckCliffCmd() *cobra.Command {
	cliffCmd := &cobra.Command{
		Use:   "cliff",
		Short: "Validate the effective git-cliff config(s)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			runner := execadapter.New(false, verbose)
			out := cmd.OutOrStdout()

			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			ui.Header(out, "Cliff")
			if failed := runCliffChecks(runner, cfg, out); failed {
				return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("git-cliff config validation failed"))
			}
			return nil
		},
	}

	cliffCmd.AddCommand(newCheckCliffChangelogCmd())
	cliffCmd.AddCommand(newCheckCliffReleaseNotesCmd())

	return cliffCmd
}

func newCheckCliffChangelogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "changelog",
		Short: "Validate the effective git-cliff changelog config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			runner := execadapter.New(false, verbose)
			out := cmd.OutOrStdout()

			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			return exitcode.Wrap(exitcode.Runtime, checkCliffDriver(runner, cfg.Changelog, "changelog", out))
		},
	}
}

func newCheckCliffReleaseNotesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "release-notes",
		Short: "Validate the effective git-cliff release-notes config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			runner := execadapter.New(false, verbose)
			out := cmd.OutOrStdout()

			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
			}

			var notesDriver *config.ContentDriver
			if cfg.Release != nil {
				notesDriver = cfg.Release.Notes
			}
			return exitcode.Wrap(exitcode.Runtime, checkCliffDriver(runner, notesDriver, "release-notes", out))
		},
	}
}

// runRuntimeCheck dispatches each runtime check with a spinner and returns
// the number of hard failures (warnings do not count).
func runRuntimeCheck(runner port.Runner, cfg *config.Config, out io.Writer) int {
	var failed int
	app.RuntimeCheck(runner, cfg,
		func(title string) { ui.Header(out, title) },
		func(name string, run func() app.RuntimeCheckItem) {
			step := ui.StartStep(out, name)
			item := run()
			switch {
			case item.IsWarn:
				step.Skip(item.Err.Error())
			case item.Err != nil:
				step.Fail(item.Err.Error())
				failed++
			default:
				step.Done(item.Value)
			}
		},
	)
	return failed
}

// runCliffChecks checks all configured git-cliff generators and reports results.
// Returns true if any check failed.
func runCliffChecks(runner port.Runner, cfg *config.Config, out io.Writer) bool {
	var failed bool
	if cfg.Changelog != nil {
		if err := checkCliffDriver(runner, cfg.Changelog, "changelog", out); err != nil {
			failed = true
		}
	}
	if cfg.Release != nil && cfg.Release.Notes != nil {
		if err := checkCliffDriver(runner, cfg.Release.Notes, "release-notes", out); err != nil {
			failed = true
		}
	}
	if cfg.Changelog == nil && (cfg.Release == nil || cfg.Release.Notes == nil) {
		_, _ = fmt.Fprintln(out, ui.Info(out, "no git-cliff generators configured"))
	}
	return failed
}

// checkCliffDriver validates one git-cliff config. Returns nil if skipped (non-gitcliff
// generator) or if git-cliff accepts the config.
func checkCliffDriver(runner port.Runner, driver *config.ContentDriver, mode string, out io.Writer) error {
	if driver == nil {
		_, _ = fmt.Fprintln(out, ui.Warn(out, fmt.Sprintf("cliff %s: skip (not configured)", mode)))
		return nil
	}
	if !strings.EqualFold(driver.Generator, "git-cliff") {
		_, _ = fmt.Fprintln(out, ui.Warn(out, fmt.Sprintf("cliff %s: skip (generator is %s, not git-cliff)", mode, driver.Generator)))
		return nil
	}
	step := ui.StartStep(out, fmt.Sprintf("cliff %s", mode))
	if err := app.CheckCliff(runner, driver, mode); err != nil {
		step.Fail(err.Error())
		return err
	}
	step.Done("valid")
	return nil
}

// printConfigErrors writes validation errors to out.
func printConfigErrors(errs config.ValidationErrors, out io.Writer) {
	for _, e := range errs {
		_, _ = fmt.Fprintln(out, ui.Err(out, fmt.Sprintf("%s: %s", e.Path, e.Message)))
		if e.Hint != "" {
			_, _ = fmt.Fprintln(out, ui.Info(out, "hint: "+e.Hint))
		}
	}
	_, _ = fmt.Fprintf(out, "%d error(s)\n", len(errs))
}
