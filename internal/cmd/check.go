package cmd

import (
	"fmt"
	"io"
	"strings"

	execadapter "github.com/adaouat/heraut/internal/adapter/exec"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
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

			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			var failed bool

			// config check
			if errs := config.Validate(cfg); len(errs) > 0 {
				printConfigErrors(errs, out)
				failed = true
			} else {
				_, _ = fmt.Fprintln(out, "config: ok")
			}

			// runtime check
			if items := app.RuntimeCheck(runner, cfg); len(items) > 0 {
				if f := printRuntimeItems(items, out); f {
					failed = true
				}
			}

			// cliff check (best-effort; skip if no git-cliff generators configured)
			if f := runCliffChecks(runner, cfg, out); f {
				failed = true
			}

			if failed {
				return fmt.Errorf("one or more checks failed")
			}
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

			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			errs := config.Validate(cfg)
			if len(errs) > 0 {
				printConfigErrors(errs, out)
				return fmt.Errorf("%d error(s) in config", len(errs))
			}
			_, _ = fmt.Fprintln(out, "config: ok")
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
				return fmt.Errorf("loading config: %w", err)
			}

			items := app.RuntimeCheck(runner, cfg)
			if failed := printRuntimeItems(items, out); failed {
				return fmt.Errorf("one or more runtime checks failed")
			}
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
				return fmt.Errorf("loading config: %w", err)
			}

			if failed := runCliffChecks(runner, cfg, out); failed {
				return fmt.Errorf("git-cliff config validation failed")
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
				return fmt.Errorf("loading config: %w", err)
			}

			return checkCliffDriver(runner, cfg.Changelog, "changelog", out)
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
				return fmt.Errorf("loading config: %w", err)
			}

			var notesDriver *config.ContentDriver
			if cfg.Release != nil {
				notesDriver = cfg.Release.Notes
			}
			return checkCliffDriver(runner, notesDriver, "release-notes", out)
		},
	}
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
	return failed
}

// checkCliffDriver validates one git-cliff config. Returns nil if skipped (non-gitcliff
// generator) or if git-cliff accepts the config.
func checkCliffDriver(runner port.Runner, driver *config.ContentDriver, mode string, out io.Writer) error {
	if driver == nil {
		_, _ = fmt.Fprintf(out, "cliff %s: skip (not configured)\n", mode)
		return nil
	}
	if !strings.EqualFold(driver.Generator, "git-cliff") {
		_, _ = fmt.Fprintf(out, "cliff %s: skip (generator is %s, not git-cliff)\n", mode, driver.Generator)
		return nil
	}
	if err := app.CheckCliff(runner, driver, mode); err != nil {
		_, _ = fmt.Fprintf(out, "cliff %s: ✗ %s\n", mode, err)
		return err
	}
	_, _ = fmt.Fprintf(out, "cliff %s: ok\n", mode)
	return nil
}

// printConfigErrors writes validation errors to out.
func printConfigErrors(errs config.ValidationErrors, out io.Writer) {
	for _, e := range errs {
		_, _ = fmt.Fprintf(out, "✗ %s: %s\n", e.Path, e.Message)
		if e.Hint != "" {
			_, _ = fmt.Fprintf(out, "  hint: %s\n", e.Hint)
		}
	}
	_, _ = fmt.Fprintf(out, "%d error(s)\n", len(errs))
}

// printRuntimeItems writes runtime check results to out.
// Returns true if any item failed.
func printRuntimeItems(items []app.RuntimeCheckItem, out io.Writer) bool {
	var failed bool
	for _, item := range items {
		if item.Err != nil {
			_, _ = fmt.Fprintf(out, "✗ %s: %s\n", item.Name, item.Err)
			failed = true
		} else {
			_, _ = fmt.Fprintf(out, "✓ %s\n", item.Name)
		}
	}
	return failed
}
