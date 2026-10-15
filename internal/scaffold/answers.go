package scaffold

import (
	"github.com/adaouat/heraut/internal/config"
)

// Answers holds the user's responses from the init wizard.
type Answers struct {
	Strategy  string // semver, calver, semver-per-env, calver-per-env
	TagPrefix string // version prefix, e.g. "v" or ""
	Format    string // CalVer format string, e.g. "YYYY.MM.PATCH"

	EnableChangelog bool
	ChangelogOutput string // e.g. "CHANGELOG.md"

	Platforms []PlatformAnswer

	// PublishReleases answers the single collapsed "create a release?" question (T220,
	// release-atomicity design): release: presence always means "generate notes and publish,"
	// together, so there is no longer an independent notes toggle. PublishReleases gates both
	// whether runPlatformWizard runs and whether cfg.Release is emitted at all in generate.go. A
	// false answer clears Platforms (see applyPublishChoice) so a stale, pre-populated slice
	// from an edit-existing-config round trip can't survive into GenerateYAML.
	PublishReleases bool

	// Sprint is the current sprint counter, required when Format contains SPRINT.
	// Not clock-derived: advance it with `heraut version sprint bump`.
	Sprint int

	// Per-env strategies only.
	TagFormat    string
	Environments []EnvAnswer

	// Assets and Tickets are not wizard-editable; they are carried through verbatim from an
	// existing config on "Update it?" (T107).
	Assets  []string
	Tickets []config.Ticket

	// EnrichmentPolicy and EnrichmentForge are wizard-editable via runEnrichmentWizard (T201).
	EnrichmentPolicy string
	EnrichmentForge  string
}

// PlatformAnswer holds answers for one release platform.
type PlatformAnswer struct {
	Type       string // "github" or "gitlab"
	Repository string // github: "owner/repo"
	Project    string // gitlab: "namespace/project"
	TokenEnv   string
	APIMode    string // gitlab only: "rest" (default) or "graphql"

	// Passthrough fields: not wizard-editable, carried verbatim from existing config (T108).
	// There is no Prerelease field — GitHub's pre-release flag is derived from the resolved
	// version, not a per-target setting the wizard carries through at all (ADR-0064, T337).
	Name    string
	BaseURL string
	Draft   bool
}

// EnvAnswer holds answers for one per-env environment.
type EnvAnswer struct {
	Name             string
	Bump             string // "auto" or "promote"
	TagFormat        string
	Source           string
	Branch           string
	DisableChangelog bool
	// DisableRelease matches config.Environment.DisableRelease (T217): turns off the entire
	// release: behavior (notes and publish, together) for this environment, not just notes.
	DisableRelease bool

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
// changelog + release enabled, gitlab platform.
func Defaults() Answers {
	return Answers{
		Strategy:        "semver",
		TagPrefix:       "v",
		EnableChangelog: true,
		ChangelogOutput: "CHANGELOG.md",
		PublishReleases: true,
		Platforms:       []PlatformAnswer{{Type: "gitlab"}},
	}
}

// ConfigToAnswers populates an Answers from an existing Config for wizard pre-population.
func ConfigToAnswers(cfg *config.Config) Answers {
	a := Answers{
		Strategy:         cfg.Versioning.Strategy,
		Format:           cfg.Versioning.Format,
		TagFormat:        cfg.Versioning.TagFormat,
		Tickets:          cfg.Tickets(),
		EnrichmentPolicy: cfg.EnrichmentPolicy(),
	}

	if cfg.Commits != nil {
		a.EnrichmentForge = cfg.Commits.EnrichmentForge
	}

	if cfg.Versioning.TagPrefix != nil {
		a.TagPrefix = *cfg.Versioning.TagPrefix
	}

	a.Sprint = cfg.Versioning.Sprint

	if cfg.Changelog != nil {
		a.EnableChangelog = true
		a.ChangelogOutput = cfg.Changelog.Output
		if a.ChangelogOutput == "" {
			a.ChangelogOutput = "CHANGELOG.md"
		}
	}

	if cfg.Release != nil {
		// release: presence alone means "wanted" (T216 atomicity) — independent of whether any
		// target resolves to a known forge, since a zero-config release (no forges:, no
		// release.targets) legitimately has nothing to populate a.Platforms from.
		a.PublishReleases = true
		a.Assets = cfg.Release.Assets
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
				APIMode:    f.APIMode,
				Name:       f.Name,
				BaseURL:    f.BaseURL,
				Draft:      t.Draft,
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
			DisableRelease:   env.DisableRelease,
			Changelog:        env.Changelog,
			Release:          env.Release,
		})
	}

	return a
}
