package config

// Config is the parsed .heraut.yml.
type Config struct {
	Version      string                 `yaml:"version"`
	Versioning   Versioning             `yaml:"versioning"`
	Changelog    *ContentDriver         `yaml:"changelog,omitempty"`
	Release      *Release               `yaml:"release,omitempty"`
	Environments map[string]Environment `yaml:"environments,omitempty"`
	// RemoteMetadata controls whether content generators fetch PR/MR metadata from the
	// platform API (author handle, PR number) to enrich changelog/release-notes:
	// "required" (fetch, fail if unavailable), "optional" (fetch when possible, else warn +
	// skip), "disabled" (never fetch). Empty resolves to "optional". Governs both changelog
	// and release-notes generation (T78).
	RemoteMetadata string `yaml:"remote_metadata,omitempty"`
	// Tickets configures issue-tracker links: each entry's regex is matched in commit
	// messages (subject/body/footer) and rendered as a link in the changelog and release
	// notes. git-cliff only (T79 / ADR-0024).
	Tickets []Ticket `yaml:"tickets,omitempty"`
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

// ContentDriver configures a content generator (git-cliff, communique, cocogitto).
type ContentDriver struct {
	Generator  string `yaml:"generator"`
	Config     string `yaml:"config,omitempty"`
	Output     string `yaml:"output,omitempty"`
	TagPattern string `yaml:"tag_pattern,omitempty"`
	Template   string `yaml:"template,omitempty"`
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
