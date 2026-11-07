package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/scaffold"
	"github.com/adaouat/heraut/internal/ui"
)

// NewInitCmd constructs the `heraut init` command.
func NewInitCmd(version string) *cobra.Command {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Generate .heraut.yml interactively (or with --defaults)",
		Long: "Create .heraut.yml by answering a few questions: versioning strategy, tag prefix, changelog and where\n" +
			"to publish. With an existing file it offers to update it, keeping every setting it does not ask about.\n" +
			"--defaults writes a default config without asking: SemVer, prefix v, CHANGELOG.md, and a release on\n" +
			"the forge found from CI or the git origin.",
		Example: "  heraut init\n" +
			"  heraut init --defaults\n" +
			"  heraut init --config .config/heraut.yml",
		RunE: func(cmd *cobra.Command, args []string) error { return runInit(cmd, version) },
	}

	initCmd.Flags().Bool("defaults", false, "write opinionated defaults non-interactively")
	initCmd.Flags().Bool("overwrite", false, "overwrite an existing config file without prompting")
	return initCmd
}

// runInit is the body of `heraut init`: pick the destination, collect answers from the wizard or
// the defaults, and write the file.
func runInit(cmd *cobra.Command, version string) error {
	flags := newFlagReader(cmd)
	cfgPath := flags.String("config")
	overwrite := flags.Bool("overwrite")
	defaults := flags.Bool("defaults")
	if err := flags.Err(); err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	if !defaults && (!ui.IsTTY(os.Stdin) || !ui.IsTTY(out)) {
		return exitcode.Wrap(exitcode.Usage, fmt.Errorf(
			"heraut init needs an interactive terminal: pass --defaults to write the default config (with --overwrite to replace an existing file), or write .heraut.yml by hand (docs/heraut.sample.yml lists every key)"))
	}

	// Write destination priority: --config flag → HERAUT_FILE env var → InitDest().
	var path string
	if cfgPath != "" {
		path = cfgPath
	} else if env := strings.TrimSpace(os.Getenv(config.EnvConfigFile)); env != "" {
		path = env
	} else {
		path = config.InitDest()
	}

	var answers scaffold.Answers
	existingLoaded := false
	var existingCfg *config.Config

	_, statErr := os.Stat(path)
	fileExists := statErr == nil

	// --defaults is the non-interactive path: it must not silently overwrite an
	// existing config (T227) — consistent with the rest of the CLI's
	// destructive-action posture (e.g. promotion guards require --force for
	// release/changelog). init has its own --overwrite (T264) rather than
	// borrowing root's --force, which means something unrelated (promotion guards).
	if defaults && fileExists && !overwrite {
		return fmt.Errorf("%s already exists (use --overwrite to overwrite with --defaults)", path)
	}

	if fileExists && !defaults {
		if !overwrite {
			var update bool
			prompt := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title(fmt.Sprintf("%s already exists. Update it?", path)).
						Value(&update),
				),
			).WithTheme(ui.HuhTheme())
			if err := prompt.Run(); err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			if !update {
				return nil
			}
		}
		// Pre-populate from existing config (best-effort; ignore load errors).
		if cfg, err := config.Load(path); err == nil {
			answers = scaffold.ConfigToAnswers(cfg)
			existingLoaded = true
			existingCfg = cfg
		}
	}

	if defaults {
		if !existingLoaded {
			answers = scaffold.Defaults()
		}
	} else {
		if err := scaffold.RunWizard(&answers); err != nil {
			return fmt.Errorf("wizard failed: %w", err)
		}
		// Post-wizard: warn about per-platform or per-env fields that couldn't
		// be matched to the rebuilt lists (add/remove/reorder/rename/type-change).
		printDroppedFieldsWarning(out, scaffold.DroppedPlatformFields(existingCfg, answers.Platforms))
		printDroppedFieldsWarning(out, scaffold.DroppedEnvFields(existingCfg, answers.Environments))
	}

	content, err := scaffold.GenerateUpdatedYAML(existingCfg, answers, version)
	if err != nil {
		return fmt.Errorf("generating config: %w", err)
	}

	if !defaults {
		_, _ = fmt.Fprintf(out, "\n%s\n", content)
		var confirmed bool
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Write this config to " + path + "?").
					Value(&confirmed),
			),
		).WithTheme(ui.HuhTheme()).Run(); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if !confirmed {
			_, _ = fmt.Fprintln(out, ui.Info(out, "init aborted"))
			return nil
		}
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return exitcode.Wrap(exitcode.Runtime, fmt.Errorf("writing config: %w", err))
	}

	_, _ = fmt.Fprintln(out, ui.Success(out, "config written to "+path))
	if defaults {
		printDetectedForge(out)
	}
	return nil
}

// printDetectedForge says which forge the runs of a --defaults config will publish to, since the
// file names none.
func printDetectedForge(out io.Writer) {
	if typ, project, ok := scaffold.DetectedForge(os.Getenv, scaffold.GitOrigin()); ok {
		_, _ = fmt.Fprintln(out, ui.Info(out, fmt.Sprintf("heraut release will publish to %s (%s): each run finds it from the CI job or the git origin", typ, project)))
		return
	}
	_, _ = fmt.Fprintln(out, ui.Warn(out, "no GitHub or GitLab forge detected from the CI job or the git origin: heraut release needs a forges: entry (see docs/heraut.sample.yml)"))
}

// printDroppedFieldsWarning warns that the listed per-platform or per-environment settings of the
// loaded config could not be matched to the edited platform or environment list, so the rewritten
// file drops them. No-op when dropped is empty.
func printDroppedFieldsWarning(out io.Writer, dropped []string) {
	if len(dropped) == 0 {
		return
	}
	_, _ = fmt.Fprintln(out, ui.Warn(out, "the following settings are not editable by the wizard and will be dropped if you continue:"))
	for _, d := range dropped {
		_, _ = fmt.Fprintf(out, "  - %s\n", d)
	}
}
