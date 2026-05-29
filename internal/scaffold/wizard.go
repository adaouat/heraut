package scaffold

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/huh/v2"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning/calver"
)

// Answers holds the user's responses from the init wizard.
type Answers struct {
	Strategy  string // semver, calver, semver-per-env, calver-per-env
	TagPrefix string // version prefix, e.g. "v" or ""
	Format    string // CalVer format string, e.g. "YYYY.MM.PATCH"

	ChangelogGenerator string // git-cliff, communique, cocogitto, or "" (none)
	ChangelogOutput    string // e.g. "CHANGELOG.md"

	NotesGenerator string // git-cliff, communique, cocogitto, or "" (none)

	Platforms []PlatformAnswer

	// Sprint is the current sprint counter, required when Format contains SPRINT.
	// Not clock-derived: advance it with `heraut version sprint bump`.
	Sprint int

	// Per-env strategies only.
	TagFormat    string
	Environments []EnvAnswer
}

// PlatformAnswer holds answers for one release platform.
type PlatformAnswer struct {
	Type       string // "github" or "gitlab"
	Repository string // github: "owner/repo"
	Project    string // gitlab: "namespace/project"
	TokenEnv   string
}

// EnvAnswer holds answers for one per-env environment.
type EnvAnswer struct {
	Name             string
	Bump             string // "auto" or "promote"
	TagFormat        string
	Source           string
	Branch           string
	DisableChangelog bool
	DisableNotes     bool
}

// calverPresets lists opinionated CalVer format choices.
var calverPresets = []struct {
	label  string
	format string
}{
	{"YYYY.MM.PATCH — Year + Month (e.g. 2026.01.0)", "YYYY.MM.PATCH"},
	{"YYYY.MM.DD.PATCH — Year + Month + Day (e.g. 2026.01.15.0)", "YYYY.MM.DD.PATCH"},
	{"YYYY.WW.PATCH — Year + Week number (e.g. 2026.04.0)", "YYYY.WW.PATCH"},
	{"YYYY.QQ.PATCH — Year + Quarter (e.g. 2026.1.0)", "YYYY.QQ.PATCH"},
	{"YYYY.SS.PATCH — Year + Semester (e.g. 2026.1.0)", "YYYY.SS.PATCH"},
	{"YYYY.SPRINT.PATCH — Year + Sprint counter (e.g. 2026.5.0)", "YYYY.SPRINT.PATCH"},
	{"YYYY.PATCH — Year only (e.g. 2026.0)", "YYYY.PATCH"},
	{"Custom format", "custom"},
}

// Defaults returns opinionated non-interactive defaults: semver, prefix "v",
// git-cliff changelog, gitlab platform.
func Defaults() Answers {
	return Answers{
		Strategy:           "semver",
		TagPrefix:          "v",
		ChangelogGenerator: "git-cliff",
		ChangelogOutput:    "CHANGELOG.md",
		NotesGenerator:     "git-cliff",
		Platforms:          []PlatformAnswer{{Type: "gitlab"}},
	}
}

// ConfigToAnswers populates an Answers from an existing Config for wizard pre-population.
func ConfigToAnswers(cfg *config.Config) Answers {
	a := Answers{
		Strategy:  cfg.Versioning.Strategy,
		Format:    cfg.Versioning.Format,
		TagFormat: cfg.Versioning.TagFormat,
	}

	if cfg.Versioning.TagPrefix != nil {
		a.TagPrefix = *cfg.Versioning.TagPrefix
	}

	a.Sprint = cfg.Versioning.Sprint

	if cfg.Changelog != nil {
		a.ChangelogGenerator = cfg.Changelog.Generator
		a.ChangelogOutput = cfg.Changelog.Output
		if a.ChangelogOutput == "" {
			a.ChangelogOutput = "CHANGELOG.md"
		}
	}

	if cfg.Release != nil {
		if cfg.Release.Notes != nil {
			a.NotesGenerator = cfg.Release.Notes.Generator
		}
		for _, p := range cfg.Release.Platforms {
			a.Platforms = append(a.Platforms, PlatformAnswer{
				Type:       p.Type,
				Repository: p.Repository,
				Project:    p.Project,
				TokenEnv:   p.TokenEnv,
			})
		}
	}

	for name, env := range cfg.Environments {
		a.Environments = append(a.Environments, EnvAnswer{
			Name:             name,
			Bump:             env.Bump,
			TagFormat:        env.TagFormat,
			Source:           env.Source,
			Branch:           env.Branch,
			DisableChangelog: env.DisableChangelog,
			DisableNotes:     env.DisableNotes,
		})
	}

	return a
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

	mainForm := huh.NewForm(
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
				Value(&a.TagFormat),
		).WithHideFunc(isNotPerEnv),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Changelog generator").
				Options(
					huh.NewOption("git-cliff", "git-cliff"),
					huh.NewOption("communique", "communique"),
					huh.NewOption("cocogitto", "cocogitto"),
					huh.NewOption("None", ""),
				).
				Value(&a.ChangelogGenerator),
			huh.NewInput().
				Title("Changelog output file").
				Description(`e.g. "CHANGELOG.md"`).
				Value(&a.ChangelogOutput),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Release notes generator").
				Options(
					huh.NewOption("git-cliff", "git-cliff"),
					huh.NewOption("communique", "communique"),
					huh.NewOption("cocogitto", "cocogitto"),
					huh.NewOption("None", ""),
				).
				Value(&a.NotesGenerator),
		),
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

	if a.NotesGenerator != "" {
		if err := runPlatformWizard(a); err != nil {
			return err
		}
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

	if err := huh.NewForm(
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

// platformTokenDefault returns the conventional token env var name for a platform type.
func platformTokenDefault(platformType string) string {
	switch platformType {
	case "github":
		return "GH_TOKEN"
	case "gitlab":
		return "GITLAB_TOKEN"
	default:
		return ""
	}
}

func runPlatformWizard(a *Answers) error {
	a.Platforms = nil
	var addPlatform bool
	first := true

	for {
		if !first {
			confirm := huh.NewForm(
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

		// Step 1: platform type.
		if err := huh.NewForm(
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

		// Step 2: platform-specific fields.
		switch p.Type {
		case "github":
			if err := huh.NewForm(
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
			if err := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Project (namespace/project)").
						Description("e.g. acme/widget").
						Value(&p.Project),
				),
			).Run(); err != nil {
				return err
			}
		}

		// Step 3: token env var, pre-filled with the platform convention.
		if p.TokenEnv == "" {
			p.TokenEnv = platformTokenDefault(p.Type)
		}
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Token environment variable").
					Description(`e.g. "GH_TOKEN" or "GITLAB_TOKEN"`).
					Value(&p.TokenEnv),
			),
		).Run(); err != nil {
			return err
		}

		a.Platforms = append(a.Platforms, p)
	}

	return nil
}

func runEnvWizard(a *Answers) error {
	a.Environments = nil
	var addEnv bool

	for {
		env := EnvAnswer{}

		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Environment name").
					Description(`e.g. "dev", "staging", "prod"`).
					Value(&env.Name),
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
					Value(&env.TagFormat),
				huh.NewInput().
					Title("Branch restriction").
					Description("Leave empty to skip branch check").
					Value(&env.Branch),
			),
			huh.NewGroup(
				huh.NewInput().
					Title("Source environment (promote mode)").
					Description("Environment to promote from").
					Value(&env.Source),
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

	return nil
}

// resolveFormatChoice maps an existing format string to a preset choice key and
// optional custom value. Returns the first preset when format is empty.
func resolveFormatChoice(format string) (choice string, custom string) {
	if format == "" {
		return calverPresets[0].format, ""
	}
	for _, p := range calverPresets {
		if p.format != "custom" && p.format == format {
			return format, ""
		}
	}
	return "custom", format
}

// suspiciousTokenRE finds uppercase-only substrings of 2+ chars in a literal —
// these are likely mistyped token names (e.g. "YYY", "OOOO").
var suspiciousTokenRE = regexp.MustCompile(`[A-Z]{2,}`)

// ValidateCalVerFormat validates a CalVer format string for use in .heraut.yml.
func ValidateCalVerFormat(format string) error {
	if strings.TrimSpace(format) == "" {
		return fmt.Errorf("format is required")
	}
	tokens, err := calver.ParseFormat(format)
	if err != nil {
		return err
	}
	hasCal := false
	for _, t := range tokens {
		switch t.Kind {
		case calver.KindYYYY, calver.KindMM, calver.KindDD,
			calver.KindWW, calver.KindQQ, calver.KindSS, calver.KindSPRINT:
			hasCal = true
		}
	}
	if !hasCal {
		return fmt.Errorf("format must contain at least one calendar token (YYYY, MM, DD, WW, QQ, SS, or SPRINT)")
	}
	for _, t := range tokens {
		if t.Kind == calver.KindLiteral {
			if m := suspiciousTokenRE.FindString(t.Literal); m != "" {
				return fmt.Errorf("unknown token %q; valid tokens: YYYY MM DD WW QQ SS SPRINT PATCH", m)
			}
		}
	}
	return nil
}
