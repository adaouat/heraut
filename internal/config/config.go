package config

// Config is the parsed .heraut.yml.
type Config struct {
	Version      string                 `yaml:"version"`
	Versioning   Versioning             `yaml:"versioning"`
	Changelog    *ContentDriver         `yaml:"changelog,omitempty"`
	Release      *Release               `yaml:"release,omitempty"`
	Environments map[string]Environment `yaml:"environments,omitempty"`
	// Commits is the single source of truth for commit semantics and enrichment (ADR-0033):
	// the conventional-commit type set (consumed by both `heraut commit verify`/`create` and
	// the native renderer's section taxonomy), scope rules, ticket links, and the
	// remote-metadata policy. Replaces the former commit_lint block and the top-level
	// tickets/remote_metadata keys.
	Commits *Commits `yaml:"commits,omitempty"`
	// Rendering configures content output (ADR-0033): the exclude rules that drop matched
	// commits from the rendered changelog/release-notes.
	Rendering *Rendering `yaml:"rendering,omitempty"`
}

// Tickets returns the configured ticket-link patterns (commits.tickets), or nil. Nil-safe.
func (c *Config) Tickets() []Ticket {
	if c.Commits == nil {
		return nil
	}
	return c.Commits.Tickets
}

// RemoteMetadata returns the configured remote-metadata policy (commits.remote_metadata), or
// "" (which the generators treat as "optional"). Nil-safe.
func (c *Config) RemoteMetadata() string {
	if c.Commits == nil {
		return ""
	}
	return c.Commits.RemoteMetadata
}

// Ticket maps a commit ticket-ID pattern to a URL template. {ticket} in URL is the first
// capture group of Pattern (or the full match if Pattern has no group); the link label is
// always the full match.
type Ticket struct {
	Pattern string `yaml:"pattern"`
	URL     string `yaml:"url"`
}

// Versioning holds version resolution settings.
type Versioning struct {
	Strategy       string  `yaml:"strategy"`
	TagPrefix      *string `yaml:"tag_prefix,omitempty"`
	InitialVersion string  `yaml:"initial_version,omitempty"`
	Bump           string  `yaml:"bump,omitempty"`
	Format         string  `yaml:"format,omitempty"`
	Sprint         int     `yaml:"sprint,omitempty"`
	TagFormat      string  `yaml:"tag_format,omitempty"`
	TagType        string  `yaml:"tag_type,omitempty"`
}

// Environment holds all per-environment configuration under the root environments map.
// Versioning fields drive version resolution; content fields override changelog/release.
type Environment struct {
	// Versioning fields
	Bump      string `yaml:"bump"`
	TagFormat string `yaml:"tag_format,omitempty"`
	Branch    string `yaml:"branch,omitempty"`
	Source    string `yaml:"source,omitempty"`

	// Content fields
	DisableChangelog bool           `yaml:"disable_changelog,omitempty"`
	DisableNotes     bool           `yaml:"disable_notes,omitempty"`
	Changelog        *ContentDriver `yaml:"changelog,omitempty"`
	Release          *EnvRelease    `yaml:"release,omitempty"`
}

// EnvRelease holds per-environment release overrides.
// Nil fields mean "inherit from root release", which differs from root Release
// where nil means "disabled".
type EnvRelease struct {
	Notes     *ContentDriver `yaml:"notes,omitempty"`
	Platforms []Platform     `yaml:"platforms,omitempty"`
}

// ContentDriver configures a content generator (git-cliff, communique).
type ContentDriver struct {
	Generator  string `yaml:"generator"`
	Config     string `yaml:"config,omitempty"`
	Output     string `yaml:"output,omitempty"`
	TagPattern string `yaml:"tag_pattern,omitempty"`
	Template   string `yaml:"template,omitempty"`
	// Rendering holds per-driver rendering overrides (template snippets + excludes),
	// deep-merged over the global rendering block. native only (ADR-0037).
	Rendering *Rendering `yaml:"rendering,omitempty"`
	// HeadingVersionPattern is set by the app layer when the effective tag_format contains
	// {env} or {build}. It is injected into the effective git-cliff TOML as a postprocessor
	// that strips the env prefix/suffix and build ID from version headings (leaving just the
	// version). Not user-configurable.
	HeadingVersionPattern string `yaml:"-"`
	// RemoteMetadata is the effective top-level Config.RemoteMetadata policy, propagated onto
	// the driver by the app layer so the generator can honour it. Empty means "optional".
	// Not user-configurable at the driver level (the user sets it once at the top level).
	RemoteMetadata string `yaml:"-"`
	// Tickets is the effective top-level Config.Tickets, propagated onto the driver by the
	// app layer so the generator can inject link_parsers. Not user-configurable per-driver.
	Tickets []Ticket `yaml:"-"`
	// Types is the effective commits.types, propagated by the app layer so the native generator
	// drives its section taxonomy from config. Not user-configurable per-driver. (native only.)
	Types []TypeRule `yaml:"-"`
	// Excludes is the effective rendering.excludes, propagated by the app layer so the native
	// generator filters commits from the output. Not user-configurable per-driver. (native only.)
	Excludes []Exclude `yaml:"-"`
	// EffectiveTemplates is the app-computed template-block override map (global rendering.templates
	// overlaid by this driver's rendering.templates), propagated for the native generator. Not
	// user-configurable directly — the knobs are rendering.templates. (native only.)
	EffectiveTemplates map[string]string `yaml:"-"`
	// HerautVersion is the running heraut binary's version, propagated by the app layer so the
	// native generator can expose it to templates as .Heraut.Version. Not user-configurable.
	// (native only.)
	HerautVersion string `yaml:"-"`
	// TypesHeadingLevel is the effective commits.types_heading_level, propagated by the app
	// layer for the native generator's section heading depth. (native only.)
	TypesHeadingLevel int `yaml:"-"`
	// TagGlob is the app-computed git tag glob scoping the native generator to the active
	// environment's tags under a per-env strategy (tagfmt.GlobPattern). Empty means "all tags"
	// (non-per-env). Not user-configurable — the user-facing knob is TagPattern. (native only.)
	TagGlob string `yaml:"-"`
	// Remote configures an explicit, metadata-only remote for git-cliff PR/author
	// enrichment. Only valid on the changelog driver — release.notes already resolves
	// this from release.platforms and rejects Remote. git-cliff only. See ADR-0026.
	Remote *Remote `yaml:"remote,omitempty"`
}

// Remote configures an explicit metadata-only remote for git-cliff PR/author enrichment
// on the changelog content driver. Unlike release.platforms (which models where heraut
// publishes a release), Remote never grants publish capability — it only tells git-cliff
// where to fetch PR/MR metadata from. See ADR-0026.
type Remote struct {
	// Type is the remote discriminator: "github", "gitlab", or "azure_devops".
	Type string `yaml:"type"`
	// Project is required for type: gitlab ("namespace[/subgroup]/repo") and
	// type: azure_devops ("organization/project" — git-cliff's own azure_devops
	// "owner" shape; not just the project name alone).
	Project string `yaml:"project,omitempty"`
	// Repository is required for type: github ("owner/repo") and type: azure_devops
	// (repository name only).
	Repository string `yaml:"repository,omitempty"`
	// TokenEnv overrides the default token env var read for this remote's type
	// (GITHUB_TOKEN, GITLAB_TOKEN, or AZURE_DEVOPS_TOKEN).
	TokenEnv string `yaml:"token_env,omitempty"`
	// APIURL overrides the remote's default API host. Only meaningful for
	// type: azure_devops (Azure DevOps Server / on-prem).
	APIURL string `yaml:"api_url,omitempty"`
}

// Release holds release notes and platform settings.
type Release struct {
	Notes     *ContentDriver `yaml:"notes,omitempty"`
	Platforms []Platform     `yaml:"platforms,omitempty"`
	// Assets lists glob patterns for files to attach to the GitHub/GitLab release.
	// Globs are expanded at release time; a pattern matching nothing emits a warning
	// but does not abort the release. Applied to all configured platforms.
	Assets []string `yaml:"assets,omitempty"`
}

// Platform holds settings for one release platform (github or gitlab).
// Type is the platform discriminator ("github" or "gitlab") mapping to yaml:"platform";
// using Type avoids the Platform.Platform self-reference (ADR-0006).
type Platform struct {
	// Name uniquely identifies this platform entry within its release.platforms list
	// (top-level or a single environment override). Required — see ADR-0025.
	Name string `yaml:"name"`
	Type string `yaml:"platform"`
	// GitHub-specific
	Repository string `yaml:"repository,omitempty"`
	Draft      bool   `yaml:"draft,omitempty"`
	Prerelease bool   `yaml:"prerelease,omitempty"`
	// GitLab-specific
	Project string `yaml:"project,omitempty"`
	// Shared
	BaseURL  string   `yaml:"base_url,omitempty"`
	TokenEnv string   `yaml:"token_env,omitempty"`
	Assets   []string `yaml:"assets,omitempty"`
	// LenientAssets is set programmatically when assets come from release.assets (top-level).
	// When true, a glob pattern that matches nothing emits a warning instead of an error.
	LenientAssets bool `yaml:"-"`
}

const (
	defaultGitHubBaseURL = "https://github.com"
	defaultGitLabBaseURL = "https://gitlab.com"
)

// DefaultBaseURL returns the default web base URL for a platform type, or "" when the
// type has no known default (e.g. an invalid platform — the type error is raised
// separately by the validator).
func DefaultBaseURL(platformType string) string {
	switch platformType {
	case "github":
		return defaultGitHubBaseURL
	case "gitlab":
		return defaultGitLabBaseURL
	default:
		return ""
	}
}
