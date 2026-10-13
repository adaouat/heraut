package cmd

import (
	"errors"
	"fmt"
	"os"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
)

// NewNotesCmd constructs the `heraut notes` command.
func NewNotesCmd() *cobra.Command {
	var (
		from     string
		format   string
		template string
		output   string
	)

	notesCmd := &cobra.Command{
		Use:   "notes <tag>",
		Short: "Print the release notes of a tag",
		Long: "Print the release notes of a tag: the changelog's groups and commit lines for the range from the tag's\n" +
			"previous release (or --from) to the tag. Read-only. Shape the output with the notes: block of .heraut.yml,\n" +
			"or --template for one run; --format text converts the Markdown to plain text.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			env, _ := cmd.Flags().GetString("env")
			force, _ := cmd.Flags().GetBool("force")

			runner := execadapter.New(false, verbose)

			cfg, err := config.Load(config.ResolvePath(cfgPath))
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}
			applyOfflineOverride(cmd, cfg)
			if errs := config.Validate(cfg); len(errs) > 0 {
				printConfigErrors(errs, cmd.ErrOrStderr())
				return exitcode.Wrap(exitcode.Config, fmt.Errorf("configuration is invalid"))
			}
			env, err = app.ResolveEnv(env, cfg, runner)
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}

			res, err := app.GenerateNotes(runner, cfg, app.NotesOpts{
				Tag: args[0], From: from, Env: env, TemplateFile: template, Format: format, Force: force,
			})
			switch {
			case errors.Is(err, app.ErrNotesFormat):
				return exitcode.Wrap(exitcode.Usage, err)
			case errors.Is(err, app.ErrNotesTagNotFound), errors.Is(err, app.ErrNotesRange):
				return exitcode.Wrap(exitcode.Runtime, err)
			case err != nil:
				return wrapRunErr(err)
			}

			for _, w := range res.Warnings {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+w)
			}
			if output != "" {
				if err := os.WriteFile(output, []byte(res.Text+"\n"), 0o644); err != nil {
					return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("writing notes: %w", err))
				}
				return nil
			}
			if res.Text != "" {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), res.Text); err != nil {
					return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("printing notes: %w", err))
				}
			}
			return nil
		},
	}

	notesCmd.Flags().StringVar(&from, "from", "", "lower bound of the range (default: the tag's previous release)")
	notesCmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown or text")
	notesCmd.Flags().StringVar(&template, "template", "", "template file for this run (replaces notes.template)")
	notesCmd.Flags().StringVar(&output, "output", "", "write the notes to this file instead of stdout")
	notesCmd.Flags().String("env", "", "target environment (for per-env strategies)")
	notesCmd.Flags().Bool("force", false, "downgrade commits.enrichment_policy: required to optional for this run")
	notesCmd.Flags().Bool("offline", false, "skip remote PR/MR metadata enrichment (forces enrichment_policy: disabled)")

	return notesCmd
}
