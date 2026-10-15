package scaffold

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"charm.land/huh/v2"

	"github.com/adaouat/heraut/internal/config"
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

// gitRemoteOriginURL runs `git remote get-url origin` and returns its raw output, or "" when not
// in a git repo or on any error.
func gitRemoteOriginURL() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

func runPlatformWizard(a *Answers) error {
	snapshot := a.Platforms
	a.Platforms = nil
	var addPlatform bool
	first := true

	for {
		if !first {
			confirm := themedForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Add another release platform?").
						Value(&addPlatform),
				),
			)
			if err := confirm.Run(); err != nil {
				return err
			}
			if !addPlatform {
				break
			}
		}
		first = false

		p := PlatformAnswer{}

		// Pre-fill type and project/repository path from CI env / git origin, when detectable.
		origin := gitRemoteOriginURL()
		detectedType, detectedProject := detectPlatform(os.Getenv, origin)
		p.Type = detectedType

		// Step 1: platform type.
		if err := themedForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Release platform").
					Options(
						huh.NewOption("GitLab", "gitlab"),
						huh.NewOption("GitHub", "github"),
					).
					Value(&p.Type),
			),
		).Run(); err != nil {
			return err
		}

		// Re-seed the full set of snapshot fields now that p.Type is known: TokenEnv/APIMode must
		// be set before Step 3 below so its prompts pre-fill correctly; Name/BaseURL/Draft
		// aren't read by any prompt but are matched here too so every field comes from
		// the same original snapshot entry.
		if orig, ok := matchPlatformSnapshot(snapshot, a.Platforms, p.Type); ok {
			p.Name = orig.Name
			p.BaseURL = orig.BaseURL
			p.Draft = orig.Draft
			p.TokenEnv = orig.TokenEnv
			p.APIMode = orig.APIMode
		}

		// Step 2: platform-specific fields.
		switch p.Type {
		case "github":
			if p.Repository == "" {
				p.Repository = detectedProject
			}
			if err := themedForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Repository (owner/repo)").
						Description("e.g. acme/widget").
						Value(&p.Repository).
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("repository is required for GitHub (e.g. org/repo)")
							}
							return nil
						}),
				),
			).Run(); err != nil {
				return err
			}
		case "gitlab":
			if p.Project == "" {
				p.Project = detectedProject
			}
			if err := themedForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Project (namespace/project)").
						Description("e.g. acme/widget").
						Value(&p.Project),
				),
				huh.NewGroup(
					huh.NewNote().
						Title("Running in GitLab CI/CD?").
						Description(
							"Use `CI_JOB_TOKEN` instead of `GITLAB_TOKEN` and enable "+
								`"Allow Git push requests to the repository" `+
								"in your project settings "+
								"(Settings › CI/CD › Job token permissions).",
						).
						Next(true),
				),
			).Run(); err != nil {
				return err
			}
		}

		// Step 3: token env var — select from known names or enter custom.
		tokenChoice, customToken := resolveTokenChoice(p.Type, p.TokenEnv)
		knownOpts := platformTokenOptions(p.Type)
		tokenOpts := make([]huh.Option[string], 0, len(knownOpts)+1)
		for _, k := range knownOpts {
			tokenOpts = append(tokenOpts, huh.NewOption(k, k))
		}
		tokenOpts = append(tokenOpts, huh.NewOption("Custom", "custom"))

		apiMode := p.APIMode
		if apiMode == "" {
			apiMode = "rest"
		}

		if err := themedForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Token environment variable").
					Options(tokenOpts...).
					Value(&tokenChoice),
			),
			huh.NewGroup(
				huh.NewInput().
					Title("Custom token environment variable").
					Value(&customToken).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("token env var is required")
						}
						return nil
					}),
			).WithHideFunc(func() bool { return tokenChoice != "custom" }),
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("API mode").
					Description("graphql renders linked @usernames but needs a read_api token — CI_JOB_TOKEN cannot use it").
					Options(
						huh.NewOption("rest", "rest"),
						huh.NewOption("graphql", "graphql"),
					).
					Value(&apiMode),
			).WithHideFunc(func() bool { return hideAPIMode(p.Type, tokenChoice) }),
		).Run(); err != nil {
			return err
		}

		p.TokenEnv, p.APIMode = finishPlatformToken(p.Type, tokenChoice, customToken, apiMode)

		a.Platforms = append(a.Platforms, p)
	}

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
