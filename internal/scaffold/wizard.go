package scaffold

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/huh/v2"

	execadapter "github.com/adaouat/forge/exec"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/ui"
)

// themedForm builds a huh form with heraut's branded theme (the Heraldic accent).
func themedForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(ui.HuhTheme())
}

// RunWizard runs the interactive huh-based wizard and populates a.
func RunWizard(a *Answers) error {
	formatChoice, customFormat := resolveFormatChoice(a.Format)

	isNotCalVer := func() bool {
		return a.Strategy != "calver" && a.Strategy != "calver-per-env"
	}
	isNotPerEnv := func() bool {
		return a.Strategy != "semver-per-env" && a.Strategy != "calver-per-env"
	}
	isNotCustomFormat := func() bool {
		return isNotCalVer() || formatChoice != "custom"
	}

	presetOpts := make([]huh.Option[string], len(calverPresets))
	for i, p := range calverPresets {
		presetOpts[i] = huh.NewOption(p.label, p.format)
	}

	mainForm := themedForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Versioning strategy").
				Options(
					huh.NewOption("SemVer (conventional commits)", "semver"),
					huh.NewOption("CalVer (calendar versioning)", "calver"),
					huh.NewOption("SemVer per-environment", "semver-per-env"),
					huh.NewOption("CalVer per-environment", "calver-per-env"),
				).
				Value(&a.Strategy),
			huh.NewInput().
				Title("Version prefix").
				Description(`e.g. "v" for v1.2.3, leave empty for no prefix`).
				Value(&a.TagPrefix),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("CalVer format").
				Options(presetOpts...).
				Value(&formatChoice),
		).WithHideFunc(isNotCalVer),
		huh.NewGroup(
			huh.NewInput().
				Title("Custom CalVer format").
				Description("Tokens: YYYY MM DD WW QQ SS SPRINT PATCH (PATCH required, must be last)").
				Value(&customFormat).
				Validate(ValidateCalVerFormat),
		).WithHideFunc(isNotCustomFormat),
		huh.NewGroup(
			huh.NewInput().
				Title("Common tag format (per-env)").
				Description(`e.g. "{env}/{version}"`).
				Value(&a.TagFormat).
				Validate(config.ValidateTagFormatForWizard),
		).WithHideFunc(isNotPerEnv),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Generate a changelog?").
				Value(&a.EnableChangelog),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Changelog output file").
				Description(`e.g. "CHANGELOG.md"`).
				Value(&a.ChangelogOutput).
				Validate(func(output string) error {
					format := formatChoice
					if format == "custom" {
						format = customFormat
					}
					return config.ValidateChangelogRotationForWizard(a.Strategy, format, output)
				}),
		).WithHideFunc(func() bool { return !a.EnableChangelog }),
	)

	if err := mainForm.Run(); err != nil {
		return err
	}

	if a.Strategy == "calver" || a.Strategy == "calver-per-env" {
		if formatChoice != "custom" {
			a.Format = formatChoice
		} else {
			a.Format = customFormat
		}
		if strings.Contains(a.Format, "SPRINT") {
			if err := runSprintWizard(a); err != nil {
				return err
			}
		}
	}

	if err := themedForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Create a release (generate notes and publish) on your forge?").
				Value(&a.PublishReleases),
		),
	).Run(); err != nil {
		return err
	}

	if err := applyPublishChoice(a); err != nil {
		return err
	}

	if err := runEnrichmentWizard(a); err != nil {
		return err
	}

	if a.Strategy == "semver-per-env" || a.Strategy == "calver-per-env" {
		if err := runEnvWizard(a); err != nil {
			return err
		}
	}

	return nil
}

func runSprintWizard(a *Answers) error {
	initial := "1"
	if a.Sprint > 0 {
		initial = strconv.Itoa(a.Sprint)
	}
	sprintStr := initial

	if err := themedForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Current sprint number").
				Description("SPRINT is not clock-derived — set it here and run 'heraut version sprint bump' to advance it").
				Value(&sprintStr).
				Validate(func(s string) error {
					n, err := strconv.Atoi(strings.TrimSpace(s))
					if err != nil || n < 1 {
						return fmt.Errorf("sprint must be a positive integer")
					}
					return nil
				}),
		),
	).Run(); err != nil {
		return err
	}

	n, _ := strconv.Atoi(strings.TrimSpace(sprintStr))
	a.Sprint = n
	return nil
}

// gitRemoteOriginURL returns the origin remote URL, or "" when not in a git repo or on any error.
// The lookup is forge.GitOriginURL, the one place that knows the command.
func gitRemoteOriginURL() string {
	return forge.GitOriginURL(execadapter.New(false, false))
}

// applyPublishChoice enforces the invariant that a.Platforms reflects the user's actual publish
// choice: a false PublishReleases must not leave a stale, pre-populated Platforms slice (from an
// edit-existing-config ConfigToAnswers round trip) surviving into GenerateYAML.
func applyPublishChoice(a *Answers) error {
	if a.PublishReleases {
		return runPlatformWizard(a)
	}
	a.Platforms = nil
	return nil
}

// runEnrichmentWizard asks the PR/MR enrichment policy whenever changelog generation or a release
// (notes + publish, together — T216 atomicity) is wanted; enrichment can work off a zero-config
// auto-detected forge even with no explicit forges: block, and only prompts which one supplies the
// data when the choice is actually ambiguous (2+ configured platforms).
func runEnrichmentWizard(a *Answers) error {
	if !a.EnableChangelog && !a.PublishReleases {
		return nil
	}

	a.EnrichmentPolicy = enrichmentPolicyOrDefault(a.EnrichmentPolicy)

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("PR/MR enrichment policy").
				Options(
					huh.NewOption("optional (fetch when possible, else warn)", "optional"),
					huh.NewOption("required (fail if unavailable)", "required"),
					huh.NewOption("disabled (never fetch)", "disabled"),
				).
				Value(&a.EnrichmentPolicy),
		),
	}

	if shouldPromptEnrichmentForge(a.Platforms) {
		names := platformDisplayNames(a.Platforms)
		forgeOpts := make([]huh.Option[string], len(names))
		for i, name := range names {
			forgeOpts[i] = huh.NewOption(name, name)
		}
		a.EnrichmentForge = enrichmentForgeChoice(names, a.EnrichmentForge)
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Enrichment forge").
				Description("Which platform supplies PR/MR metadata?").
				Options(forgeOpts...).
				Value(&a.EnrichmentForge),
		))
	}

	return themedForm(groups...).Run()
}

func runEnvWizard(a *Answers) error {
	snapshot := a.Environments
	a.Environments = nil
	var addEnv bool

	for {
		env := EnvAnswer{}

		if err := themedForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Environment name").
					Description(`e.g. "dev", "staging", "prod"`).
					Value(&env.Name).
					Validate(func(s string) error {
						return validateEnvName(a.Environments, s)
					}),
				huh.NewSelect[string]().
					Title("Bump mode").
					Options(
						huh.NewOption("auto (resolve from commits)", "auto"),
						huh.NewOption("promote (copy from source env)", "promote"),
					).
					Value(&env.Bump),
				huh.NewInput().
					Title("Tag format override").
					Description("Leave empty to use the common tag format").
					Value(&env.TagFormat).
					Validate(config.ValidateTagFormatForWizard),
				huh.NewInput().
					Title("Branch restriction").
					Description("Leave empty to skip branch check").
					Value(&env.Branch),
			),
			huh.NewGroup(
				huh.NewInput().
					Title("Source environment (promote mode)").
					Description("Environment to promote from").
					Value(&env.Source).
					Validate(func(s string) error {
						return validateEnvSource(a.Environments, env.Name, s)
					}),
			).WithHideFunc(func() bool { return env.Bump != "promote" }),
			huh.NewGroup(
				huh.NewConfirm().
					Title("Add another environment?").
					Value(&addEnv),
			),
		).Run(); err != nil {
			return err
		}

		a.Environments = append(a.Environments, env)
		if !addEnv {
			break
		}
	}

	a.Environments = matchEnvSnapshot(snapshot, a.Environments)
	return nil
}
