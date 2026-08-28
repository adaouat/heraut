package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
	commitCmd.AddCommand(newCommitTicketsCmd())
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
					return exitcode.Wrap(exitcode.Config, err)
				}
				cfg = nil
			}
			if cfg != nil {
				if errs := config.Validate(cfg); len(errs) > 0 {
					printConfigErrors(errs, cmd.OutOrStdout())
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", len(errs)))
				}
			}

			summary, err := app.VerifyCommit(cfg, message)
			if err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}
			printCommitSummary(summary, cmd.OutOrStdout())
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
					return exitcode.Wrap(exitcode.Config, err)
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

func newCommitTicketsCmd() *cobra.Command {
	var fromLatestTag bool
	cmd := &cobra.Command{
		Use:   "tickets [rev-range]",
		Short: "Validate commits.tickets patterns against a range (or full history) of commits",
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
				return exitcode.Wrap(exitcode.Config, err)
			}
			if errs := config.Validate(cfg); len(errs) > 0 {
				printConfigErrors(errs, cmd.OutOrStdout())
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", len(errs)))
			}
			if len(cfg.Tickets()) == 0 {
				return exitcode.Wrap(exitcode.Usage, errors.New("no commits.tickets configured; nothing to check"))
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

			results, err := app.CheckTicketsInRange(runner, cfg, revRange)
			if err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}

			printTicketCheckResults(results, verbose, cmd.OutOrStdout())
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
					return exitcode.Wrap(exitcode.Config, err)
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
// printTicketCheckResults prints, for each commit with at least one ticket match, its
// matches (SHA, subject, then one line per match); with verbose, commits with no matches
// are also listed. Always prints a final summary line, and — since zero matches across an
// entire range is far more likely to mean "the pattern is broken" than "no work references
// tickets" — a warning when the total is zero, though this never fails the command: it's a
// diagnostic tool, not a gate.
func printTicketCheckResults(results []app.TicketCheckResult, verbose bool, out io.Writer) {
	var total int
	for _, r := range results {
		switch {
		case len(r.Matches) > 0:
			_, _ = fmt.Fprintln(out, ui.Success(out, fmt.Sprintf("%s  %s", r.SHA, r.Subject)))
			for _, m := range r.Matches {
				total++
				_, _ = fmt.Fprintf(out, "  → %s (%s)\n", m.Text, m.Href)
			}
		case verbose:
			_, _ = fmt.Fprintln(out, ui.Info(out, fmt.Sprintf("%s  %s  (no ticket references)", r.SHA, r.Subject)))
		}
	}
	if total == 0 {
		_, _ = fmt.Fprintln(out, ui.Warn(out, fmt.Sprintf("no ticket references found in %d commit(s) analysed — check your commits.tickets pattern(s)", len(results))))
		return
	}
	_, _ = fmt.Fprintf(out, "%d ticket reference(s) found across %d commit(s) analysed.\n", total, len(results))
}

// printCommitSummary prints a cocogitto-style recap of a successfully verified commit:
// type, scope, breaking, description, and any detected commits.tickets references. nil
// summary (a skipped merge/fixup commit) prints nothing, preserving the previous silent
// success behavior for those.
func printCommitSummary(s *app.CommitSummary, out io.Writer) {
	if s == nil {
		return
	}
	_, _ = fmt.Fprintln(out, ui.Success(out, "commit message is valid"))
	_, _ = fmt.Fprintf(out, "  type:        %s\n", s.Type)
	if s.Scope != "" {
		_, _ = fmt.Fprintf(out, "  scope:       %s\n", s.Scope)
	}
	_, _ = fmt.Fprintf(out, "  breaking:    %t\n", s.Breaking)
	_, _ = fmt.Fprintf(out, "  description: %s\n", s.Description)
	if len(s.Tickets) > 0 {
		refs := make([]string, len(s.Tickets))
		for i, t := range s.Tickets {
			refs[i] = fmt.Sprintf("%s (%s)", t.Text, t.Href)
		}
		_, _ = fmt.Fprintf(out, "  tickets:     %s\n", strings.Join(refs, ", "))
	}
}

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
