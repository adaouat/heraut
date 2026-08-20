package scaffold

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"charm.land/huh/v2"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/adaouat/heraut/internal/versioning/calver"
)

// themedForm builds a huh form with heraut's branded theme (the Heraldic accent).
func themedForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(ui.HuhTheme())
}

// Answers holds the user's responses from the init wizard.
type Answers struct {
	Strategy  string // semver, calver, semver-per-env, calver-per-env
	TagPrefix string // version prefix, e.g. "v" or ""
	Format    string // CalVer format string, e.g. "YYYY.MM.PATCH"

	ChangelogGenerator string // git-cliff, communique, or "" (none)
	ChangelogOutput    string // e.g. "CHANGELOG.md"

	NotesGenerator string // git-cliff, communique, or "" (none)

	Platforms []PlatformAnswer

	// Sprint is the current sprint counter, required when Format contains SPRINT.
	// Not clock-derived: advance it with `heraut version sprint bump`.
	Sprint int

	// Per-env strategies only.
	TagFormat    string
	Environments []EnvAnswer

	// Assets, Tickets, RemoteMetadata, and EnrichmentForge are not wizard-editable; they are
	// carried through verbatim from an existing config on "Update it?" (T107). EnrichmentForge
	// references a forges[].name (by the pre-rebuild name, matched positionally like Platforms'
	// passthrough fields — see matchPlatformSnapshot); forge selection has no wizard prompt yet
	// (that redesign is T164/P4).
	Assets          []string
	Tickets         []config.Ticket
	RemoteMetadata  string
	EnrichmentForge string
}

// PlatformAnswer holds answers for one release platform.
type PlatformAnswer struct {
	Type       string // "github" or "gitlab"
	Repository string // github: "owner/repo"
	Project    string // gitlab: "namespace/project"
	TokenEnv   string

	// Passthrough fields: not wizard-editable, carried verbatim from existing config (T108).
	Name       string
	BaseURL    string
	Draft      bool
	Prerelease bool
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

	// Passthrough fields: not wizard-editable, carried verbatim from existing config (T109).
	Changelog *config.ContentDriver
	Release   *config.EnvRelease
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
		Strategy:       cfg.Versioning.Strategy,
		Format:         cfg.Versioning.Format,
		TagFormat:      cfg.Versioning.TagFormat,
		Tickets:        cfg.Tickets(),
		RemoteMetadata: cfg.EnrichmentPolicy(),
	}

	if cfg.Commits != nil {
		a.EnrichmentForge = cfg.Commits.EnrichmentForge
	}

	if cfg.Versioning.TagPrefix != nil {
		a.TagPrefix = *cfg.Versioning.TagPrefix
	}

	a.Sprint = cfg.Versioning.Sprint

	if cfg.Changelog != nil {
		// cfg.Changelog.Generator is always "" post-T177 (config.Load hard-rejects a present
		// generator: key) — presence of the block is what the wizard's generator prompt must
		// encode, so pre-select a non-empty sentinel rather than propagating the now-meaningless
		// empty string (which would match the "None" option and drop the block on re-run).
		a.ChangelogGenerator = "git-cliff"
		a.ChangelogOutput = cfg.Changelog.Output
		if a.ChangelogOutput == "" {
			a.ChangelogOutput = "CHANGELOG.md"
		}
	}

	if cfg.Release != nil {
		a.Assets = cfg.Release.Assets
		if cfg.Release.Notes != nil {
			a.NotesGenerator = "git-cliff"
		}
		forgesByName := make(map[string]config.Forge, len(cfg.Forges))
		for _, f := range cfg.Forges {
			forgesByName[f.Name] = f
		}
		for _, t := range cfg.Release.Targets {
			name := t.Forge
			if name == "" && len(cfg.Forges) == 1 {
				name = cfg.Forges[0].Name
			}
			f, ok := forgesByName[name]
			if !ok {
				continue // target references a forge that does not resolve; nothing to round-trip
			}
			a.Platforms = append(a.Platforms, PlatformAnswer{
				Type:       f.Type,
				Repository: f.Repository,
				Project:    f.Project,
				TokenEnv:   f.TokenEnv,
				Name:       f.Name,
				BaseURL:    f.BaseURL,
				Draft:      t.Draft,
				Prerelease: t.Prerelease,
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
			Changelog:        env.Changelog,
			Release:          env.Release,
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
				Value(&a.TagFormat),
		).WithHideFunc(isNotPerEnv),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Changelog generator").
				Options(
					huh.NewOption("git-cliff", "git-cliff"),
					huh.NewOption("communique", "communique"),
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

// detectPlatform pre-fills a new platform's type and project/repository path: CI
// environment/known-host detection (via internal/forge) identifies the type when possible; the
// project/repository path falls back to parseRemoteProject's any-host parsing when forge
// detection doesn't apply (self-hosted instances, no CI markers) or names a type the wizard's
// platform Select doesn't offer (only "github"/"gitlab" are wizard-supported today).
func detectPlatform(getenv func(string) string, gitOrigin string) (typ, projectOrRepo string) {
	if t, p, ok := forge.DetectForWizard(getenv, gitOrigin); ok && (t == "github" || t == "gitlab") {
		if p == "" {
			p = parseRemoteProject(gitOrigin)
		}
		return t, p
	}
	return "", parseRemoteProject(gitOrigin)
}

// parseRemoteProject extracts "namespace/project" (or "owner/repo") from a git
// remote URL. Handles SSH (git@host:path.git), ssh:// and HTTPS schemes.
func parseRemoteProject(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	// Strip any scheme prefix (https://, ssh://, git+ssh://, …)
	if i := strings.Index(rawURL, "://"); i != -1 {
		rawURL = rawURL[i+3:]
		// Now looks like "host/path.git" — strip the host.
		if i := strings.Index(rawURL, "/"); i != -1 {
			rawURL = rawURL[i+1:]
		} else {
			return ""
		}
	} else if i := strings.Index(rawURL, ":"); i != -1 {
		// SCP-style SSH: git@host:path.git — everything after the colon is the path.
		rawURL = rawURL[i+1:]
	}
	return strings.TrimSuffix(rawURL, ".git")
}

// platformTokenOptions returns the known token env var names for a platform,
// in display order. The first entry is the recommended default.
func platformTokenOptions(platformType string) []string {
	switch platformType {
	case "github":
		return []string{"GH_TOKEN"}
	case "gitlab":
		return []string{"GITLAB_TOKEN", "CI_JOB_TOKEN"}
	default:
		return nil
	}
}

// resolveTokenChoice maps an existing token env var to a wizard select choice
// and optional custom value. When existing is empty the first known token is
// returned as the default. Unrecognised values map to ("custom", existing).
func resolveTokenChoice(platformType, existing string) (choice, custom string) {
	opts := platformTokenOptions(platformType)
	if existing == "" {
		if len(opts) > 0 {
			return opts[0], ""
		}
		return "custom", ""
	}
	if slices.Contains(opts, existing) {
		return existing, ""
	}
	return "custom", existing
}

// matchPlatformSnapshot applies passthrough fields (Name, BaseURL, Draft, Prerelease)
// from the pre-rebuild snapshot to the rebuilt entries using type-scoped positional
// matching: the n-th rebuilt entry of type T gets the fields from the n-th original
// entry of type T. Rebuilt entries with no corresponding original get zero values.
func matchPlatformSnapshot(original, rebuilt []PlatformAnswer) []PlatformAnswer {
	byType := make(map[string][]PlatformAnswer)
	for _, p := range original {
		byType[p.Type] = append(byType[p.Type], p)
	}
	consumed := make(map[string]int)
	result := make([]PlatformAnswer, len(rebuilt))
	for i, p := range rebuilt {
		if idx := consumed[p.Type]; idx < len(byType[p.Type]) {
			orig := byType[p.Type][idx]
			p.Name = orig.Name
			p.BaseURL = orig.BaseURL
			p.Draft = orig.Draft
			p.Prerelease = orig.Prerelease
		}
		consumed[p.Type]++
		result[i] = p
	}
	return result
}

// matchEnvSnapshot carries passthrough fields (Changelog, Release) from the pre-wizard
// snapshot to the rebuilt environment list using name-based matching. Rebuilt entries
// with no name match in original are returned unchanged.
func matchEnvSnapshot(original, rebuilt []EnvAnswer) []EnvAnswer {
	byName := make(map[string]EnvAnswer, len(original))
	for _, e := range original {
		byName[e.Name] = e
	}
	result := make([]EnvAnswer, len(rebuilt))
	for i, e := range rebuilt {
		if orig, ok := byName[e.Name]; ok {
			e.Changelog = orig.Changelog
			e.Release = orig.Release
		}
		result[i] = e
	}
	return result
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
		).Run(); err != nil {
			return err
		}

		if tokenChoice == "custom" {
			p.TokenEnv = strings.TrimSpace(customToken)
		} else {
			p.TokenEnv = tokenChoice
		}

		a.Platforms = append(a.Platforms, p)
	}

	a.Platforms = matchPlatformSnapshot(snapshot, a.Platforms)
	return nil
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

	a.Environments = matchEnvSnapshot(snapshot, a.Environments)
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
