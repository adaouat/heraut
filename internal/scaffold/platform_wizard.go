package scaffold

import (
	"os"

	"charm.land/huh/v2"
)

// runPlatformWizard asks for the release platforms, one after another, pre-filling each from the
// previous configuration (the snapshot) and from the CI environment or git origin.
func runPlatformWizard(a *Answers) error {
	snapshot := a.Platforms
	a.Platforms = nil
	var addPlatform bool

	for first := true; ; first = false {
		if !first {
			if err := confirmAnotherPlatform(&addPlatform); err != nil {
				return err
			}
			if !addPlatform {
				return nil
			}
		}
		p, err := askPlatform(snapshot, a.Platforms)
		if err != nil {
			return err
		}
		a.Platforms = append(a.Platforms, p)
	}
}

func confirmAnotherPlatform(addPlatform *bool) error {
	return themedForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Add another release platform?").
				Value(addPlatform),
		),
	).Run()
}

// askPlatform collects one platform: its type, where it lives, and the token to publish with.
// rebuiltSoFar are the entries already collected in this run, which decide which snapshot entry
// this one continues.
func askPlatform(snapshot, rebuiltSoFar []PlatformAnswer) (PlatformAnswer, error) {
	p := PlatformAnswer{}

	// Pre-fill type and project/repository path from CI env / git origin, when detectable.
	detectedType, detectedProject := detectPlatform(os.Getenv, gitRemoteOriginURL())
	p.Type = detectedType

	if err := askPlatformType(&p); err != nil {
		return p, err
	}

	// Re-seed the carried fields now that p.Type is known: TokenEnv and APIMode must be set before
	// the token question so it pre-fills correctly; the others are matched here too so every field
	// comes from the same original snapshot entry.
	if orig, ok := matchPlatformSnapshot(snapshot, rebuiltSoFar, p.Type); ok {
		applyPlatformSnapshot(&p, orig)
	}

	prefillPlatformLocation(&p, detectedProject)
	if err := askPlatformLocation(&p); err != nil {
		return p, err
	}
	err := askPlatformToken(&p)
	return p, err
}

func askPlatformType(p *PlatformAnswer) error {
	return themedForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Release platform").
				Options(
					huh.NewOption("GitLab", "gitlab"),
					huh.NewOption("GitHub", "github"),
				).
				Value(&p.Type),
		),
	).Run()
}

// askPlatformLocation asks for the GitHub repository or the GitLab project.
func askPlatformLocation(p *PlatformAnswer) error {
	switch p.Type {
	case "github":
		return themedForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Repository (owner/repo)").
					Description("e.g. acme/widget").
					Value(&p.Repository).
					Validate(requireGitHubRepository),
			),
		).Run()
	case "gitlab":
		return themedForm(
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
		).Run()
	}
	return nil
}

// askPlatformToken asks which token env var to publish with and, where it applies, the API mode.
func askPlatformToken(p *PlatformAnswer) error {
	tokenChoice, customToken := resolveTokenChoice(p.Type, p.TokenEnv)
	tokenOpts := make([]huh.Option[string], 0, 3)
	for _, k := range tokenSelectOptions(p.Type) {
		tokenOpts = append(tokenOpts, huh.NewOption(tokenOptionLabel(k), k))
	}
	apiMode := apiModeOrDefault(p.APIMode)

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
				Validate(requireTokenEnv),
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
	return nil
}
