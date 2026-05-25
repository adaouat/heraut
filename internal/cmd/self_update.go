package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/selfupdate"
)

// NewSelfUpdateCmd constructs the `heraut self-update` command.
func NewSelfUpdateCmd(version string, opts ...selfupdate.Option) *cobra.Command {
	var checkOnly bool

	c := &cobra.Command{
		Use:   "self-update",
		Short: "Download and install the latest heraut binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			u := selfupdate.New(version, opts...)

			if checkOnly {
				latest, available, err := u.Check(context.Background())
				if err != nil {
					return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("checking for updates: %w", err))
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "current: %s\nlatest:  %s\n", version, latest)
				if available {
					return errors.New("update available")
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "heraut is up to date")
				return nil
			}

			return exitcode.Wrap(exitcode.Runtime, u.Do(context.Background(), cmd.OutOrStdout()))
		},
	}

	c.Flags().BoolVar(&checkOnly, "check", false, "check for updates without downloading")
	return c
}
