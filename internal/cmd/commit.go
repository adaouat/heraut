package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
)

// NewCommitCmd constructs the `heraut commit` parent command and its subcommands.
func NewCommitCmd() *cobra.Command {
	commitCmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit message tooling",
	}
	commitCmd.AddCommand(newCommitVerifyCmd())
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
