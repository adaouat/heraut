package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	execadapter "github.com/adaouat/forge/exec"
	forgeui "github.com/adaouat/forge/ui"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/spf13/cobra"
)

// NewCheckCmd constructs the `heraut check` parent command.
// When called without a subcommand it runs config + runtime checks.
func NewCheckCmd() *cobra.Command {
	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Run preflight validations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			env, _ := cmd.Flags().GetString("env")
			runner := execadapter.New(false, verbose)
			out := cmd.OutOrStdout()

			path, source := config.ResolvePathWithSource(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return exitcode.Wrap(exitcode.Config, err)
				}
				cfg = nil
			}
			if cfg != nil {
				applyOfflineOverride(cmd, cfg)
			}

			var failed, configFailed int

			// Config section
			ui.Header(out, "Config")
			if cfg == nil {
				_, _ = fmt.Fprintln(out, ui.Warn(out, fmt.Sprintf("no config found at %s — skipping config validation", path)))
			} else {
				_, _ = fmt.Fprintln(out, ui.Info(out, fmt.Sprintf("%s  (from %s)", path, source)))
				if errs := config.Validate(cfg); len(errs) > 0 {
					printConfigErrors(errs, out)
					configFailed = len(errs)
					failed += configFailed
				} else {
					_, _ = fmt.Fprintln(out, ui.Success(out, "config: ok"))
				}
			}

			env, err = app.ResolveEnv(env, cfg, runner)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			// Runtime section (Git / Platforms — headers emitted by RuntimeCheck)
			failed += runRuntimeCheck(runner, cfg, env, out)

			// Summary
			_, _ = fmt.Fprintln(out)
			if failed > 0 {
				_, _ = fmt.Fprintln(out, ui.Err(out, fmt.Sprintf("%d check(s) failed — fix the issues above before running heraut release", failed)))
				if configFailed > 0 {
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("one or more checks failed"))
				}
				return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("one or more checks failed"))
			}
			_, _ = fmt.Fprintln(out, ui.Success(out, "all checks passed"))
			return nil
		},
	}

	checkCmd.AddCommand(newCheckConfigCmd())
	checkCmd.AddCommand(newCheckRuntimeCmd())

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
				return exitcode.Wrap(exitcode.Config, err)
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
			env, _ := cmd.Flags().GetString("env")
			runner := execadapter.New(false, verbose)
			out := cmd.OutOrStdout()

			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return exitcode.Wrap(exitcode.Config, err)
				}
				_, _ = fmt.Fprintln(out, ui.Warn(out, fmt.Sprintf("no config found at %s — all tools checked as required", path)))
				cfg = nil
			}

			env, err = app.ResolveEnv(env, cfg, runner)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			if failed := runRuntimeCheck(runner, cfg, env, out); failed > 0 {
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

// runRuntimeCheck dispatches each runtime check with a spinner and returns
// the number of hard failures (warnings do not count). env selects the
// effective release.targets list (root or env override) for the Platforms
// section; pass "" to check the root list.
func runRuntimeCheck(runner port.Runner, cfg *config.Config, env string, out io.Writer) int {
	var failed int
	sp := forgeui.NewSpinner(out, forgeui.Human)
	app.RuntimeCheck(runner, cfg, env,
		func(title string) { ui.Header(out, title) },
		func(name string, run func() app.RuntimeCheckItem) {
			err := sp.Run(name, func() (forgeui.Result, error) {
				item := run()
				switch {
				case item.IsWarn:
					return forgeui.Result{}, forgeui.Skip(item.Err.Error())
				case item.Err != nil:
					return forgeui.Result{}, item.Err
				default:
					return forgeui.Result{Detail: item.Value}, nil
				}
			})
			if err != nil {
				failed++
			}
		},
	)
	return failed
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
