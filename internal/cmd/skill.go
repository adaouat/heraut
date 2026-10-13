package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/skill"
)

// NewSkillCmd constructs the `heraut skill` parent command: the agent skill bundled in the binary.
func NewSkillCmd() *cobra.Command {
	skillCmd := &cobra.Command{
		Use:   "skill",
		Short: "Install or print the agent skill that teaches an AI agent to use heraut",
	}
	skillCmd.AddCommand(newSkillPrintCmd(), newSkillInstallCmd())
	return skillCmd
}

func newSkillPrintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "print",
		Short: "Print the agent skill (SKILL.md) to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := fmt.Fprint(cmd.OutOrStdout(), skill.Content()); err != nil {
				return exitcode.Wrap(exitcode.Runtime, err)
			}
			return nil
		},
	}
}

func newSkillInstallCmd() *cobra.Command {
	var dir string
	var force bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Write the agent skill to <dir>/heraut/SKILL.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := filepath.Join(dir, "heraut", "SKILL.md")
			want := []byte(skill.Content())
			switch have, err := os.ReadFile(target); {
			case err == nil && string(have) == string(want):
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s is up to date\n", target)
				return nil
			case err == nil && !force:
				return exitcode.Wrap(exitcode.Usage, fmt.Errorf("%s differs from the bundled skill: pass --force to replace it", target))
			case err != nil && !os.IsNotExist(err):
				return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("reading %s: %w", target, err))
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("creating skill directory: %w", err))
			}
			if err := os.WriteFile(target, want, 0o644); err != nil {
				return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("writing skill: %w", err))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "skill written to %s\n", target)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".claude/skills", "skills directory to install into")
	cmd.Flags().BoolVar(&force, "force", false, "replace an installed skill that differs from the bundled one")
	return cmd
}
