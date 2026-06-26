package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/commitwizard"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/ui"
)

// NewCommitCmd constructs the `heraut commit` parent command and its subcommands.
func NewCommitCmd() *cobra.Command {
	commitCmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit message tooling",
	}
	commitCmd.AddCommand(newCommitVerifyCmd())
	commitCmd.AddCommand(newCommitCheckCmd())
	commitCmd.AddCommand(newCommitCreateCmd())
	return commitCmd
}

func newCommitVerifyCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "verify [message]",
		Short: "Validate a commit message against the conventional-commit grammar",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			message, err := readCommitMessage(cmd, args, file)
			if err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}

			cfgPath, _ := cmd.Flags().GetString("config")
			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
				}
				cfg = nil
			}
			if cfg != nil {
				if errs := config.Validate(cfg); len(errs) > 0 {
					printConfigErrors(errs, cmd.OutOrStdout())
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", len(errs)))
				}
			}

			if err := app.VerifyCommit(cfg, message); err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "read the commit message from a file (use - for stdin)")
	return cmd
}

func newCommitCheckCmd() *cobra.Command {
	var fromLatestTag bool
	cmd := &cobra.Command{
		Use:   "check [rev-range]",
		Short: "Validate every commit in a range (or full history) against the conventional-commit grammar",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromLatestTag && len(args) == 1 {
				return exitcode.Wrap(exitcode.Usage, errors.New("cannot use both --from-latest-tag and a rev-range argument"))
			}

			var revRange string
			if len(args) == 1 {
				revRange = args[0]
			}

			cfgPath, _ := cmd.Flags().GetString("config")
			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
				}
				cfg = nil
			}
			if cfg != nil {
				if errs := config.Validate(cfg); len(errs) > 0 {
					printConfigErrors(errs, cmd.OutOrStdout())
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", len(errs)))
				}
			}

			verbose, _ := cmd.Flags().GetBool("verbose")
			runner := execadapter.New(false, verbose)

			if fromLatestTag {
				env, _ := cmd.Flags().GetString("env")
				resolved, noTags, err := app.ResolveFromLatestTag(runner, cfg, env)
				if err != nil {
					return exitcode.Wrap(exitcode.Usage, err)
				}
				if noTags {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Warn(cmd.OutOrStdout(), "no tags found — checking full history"))
				} else {
					revRange = resolved
				}
			}

			results, err := app.CheckCommitRange(runner, cfg, revRange)
			if err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}

			failed := printCommitCheckResults(results, verbose, cmd.OutOrStdout())
			if failed > 0 {
				return exitcode.Wrap(exitcode.Usage, fmt.Errorf("%d of %d commits invalid", failed, len(results)))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fromLatestTag, "from-latest-tag", false, "check commits since the latest tag (mutually exclusive with rev-range)")
	return cmd
}

func newCommitCreateCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Interactively author a Conventional Commits message and commit it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
				}
				cfg = nil
			}
			if cfg != nil {
				if errs := config.Validate(cfg); len(errs) > 0 {
					printConfigErrors(errs, cmd.OutOrStdout())
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", len(errs)))
				}
			}

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			verbose, _ := cmd.Flags().GetBool("verbose")
			// Always a real runner: the wizard's only mutation (git commit) is gated by
			// Options.DryRun in finalize, and read-only staging checks must really run.
			runner := execadapter.New(false, verbose)

			opts := commitwizard.Options{All: all, DryRun: dryRun, Out: cmd.OutOrStdout()}
			if err := commitwizard.Run(runner, cfg, opts); err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "stage all tracked modifications before committing (git commit -a)")
	return cmd
}

// printCommitCheckResults renders results to out: failing commits always print
// (SHA, subject, reason); verbose additionally prints every valid/skipped commit.
// Returns the number of invalid commits.
func printCommitCheckResults(results []app.CommitCheckResult, verbose bool, out io.Writer) int {
	var failed int
	for _, r := range results {
		switch {
		case r.Err != nil:
			failed++
			_, _ = fmt.Fprintln(out, ui.Err(out, fmt.Sprintf("%s  %s — %s", r.SHA, r.Subject, r.Err)))
		case verbose:
			_, _ = fmt.Fprintln(out, ui.Success(out, fmt.Sprintf("%s  %s", r.SHA, r.Subject)))
		}
	}
	if failed == 0 {
		_, _ = fmt.Fprintln(out, ui.Success(out, "all commits follow conventional commits!"))
		_, _ = fmt.Fprintf(out, "  %d commits analysed\n", len(results))
	} else {
		_, _ = fmt.Fprintf(out, "%d of %d commits invalid\n", failed, len(results))
	}
	return failed
}

func readCommitMessage(cmd *cobra.Command, args []string, file string) (string, error) {
	if file != "" && len(args) == 1 {
		return "", errors.New("provide a commit message as an argument or via --file, not both")
	}
	if file != "" {
		if file == "-" {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return "", fmt.Errorf("reading commit message from stdin: %w", err)
			}
			return string(data), nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading commit message from %s: %w", file, err)
		}
		return string(data), nil
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return "", errors.New("provide a commit message as an argument or via --file")
}
