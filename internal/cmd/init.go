package cmd

import (
	"fmt"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			force, _ := cmd.Flags().GetBool("force")
			defaults, _ := cmd.Flags().GetBool("defaults")
			out := cmd.OutOrStdout()

			// Write destination priority: --config flag → HERAUT_FILE env var → InitDest().
			var path string
			if cfgPath != "" {
				path = cfgPath
			} else if env := strings.TrimSpace(os.Getenv("HERAUT_FILE")); env != "" {
				path = env
			} else {
				path = config.InitDest()
			}

			var answers scaffold.Answers
			existingLoaded := false

			_, statErr := os.Stat(path)
			fileExists := statErr == nil

			if fileExists && !defaults {
				if !force {
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
			}

			content, err := scaffold.GenerateYAML(answers, version)
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
			return nil
		},
	}

	initCmd.Flags().Bool("defaults", false, "write opinionated defaults non-interactively")
	return initCmd
}
