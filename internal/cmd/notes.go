package cmd

import (
	"errors"
	"fmt"
	"io/fs"
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
		from      string
		format    string
		template  string
		output    string
		overwrite bool
	)

	notesCmd := &cobra.Command{
		Use:   "notes <tag>",
		Short: "Print the release notes of a tag",
		Long: "Print the release notes of a tag: the changelog's groups and commit lines for the range from the tag's\n" +
			"previous release (or --from) to the tag. Read-only. Shape the output with the notes: block of .heraut.yml,\n" +
			"or --template for one run; --format text converts the Markdown to plain text.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := newFlagReader(cmd)
			cfgPath := flags.String("config")
			verbose := flags.Bool("verbose")
			env := flags.String("env")
			force := flags.Bool("force")
			if err := flags.Err(); err != nil {
				return err
			}

			runner := execadapter.New(false, verbose)

			cfg, err := config.Load(config.ResolvePath(cfgPath))
			if err != nil {
				return exitcode.Wrap(exitcode.Config, err)
			}
			if err := applyOfflineOverride(cmd, cfg); err != nil {
				return err
			}
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
			case errors.Is(err, app.ErrNotesTagNotFound), errors.Is(err, app.ErrNotesNotReleaseTag), errors.Is(err, app.ErrNotesRange):
				return exitcode.Wrap(exitcode.Runtime, err)
			case err != nil:
				return wrapRunErr(err)
			}

			for _, w := range res.Warnings {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+w)
			}
			if output != "" {
				content := res.Text
				if content != "" {
					content += "\n"
				}
				return writeNotes(output, content, overwrite)
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
	notesCmd.Flags().BoolVar(&overwrite, "overwrite", false, "replace the --output file if it already exists")
	notesCmd.Flags().String("env", "", "target environment (for per-env strategies)")
	notesCmd.Flags().Bool("force", false, "downgrade commits.enrichment_policy: required to optional for this run")
	notesCmd.Flags().Bool("offline", false, "skip remote PR/MR metadata enrichment (forces enrichment_policy: disabled)")

	return notesCmd
}

// writeNotes writes content to path. Without overwrite an existing path, a symlink included, is
// refused (SEC-002): the existence check and the create are one O_EXCL open, so nothing can be put
// there in between.
func writeNotes(path, content string, overwrite bool) error {
	flag := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if overwrite {
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flag, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return exitcode.Wrap(exitcode.Usage, fmt.Errorf("%s already exists: pass --overwrite to replace it", path))
	}
	if err != nil {
		return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("writing notes: %w", err))
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("writing notes: %w", err))
	}
	if err := f.Close(); err != nil {
		return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("writing notes: %w", err))
	}
	return nil
}
