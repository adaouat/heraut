package config

// Config is the parsed .heraut.yml.
type Config struct {
	Version      string                 `yaml:"version"`
	Versioning   Versioning             `yaml:"versioning"`
	Changelog    *ContentDriver         `yaml:"changelog,omitempty"`
	Release      *Release               `yaml:"release,omitempty"`
	Environments map[string]EnvOverride `yaml:"environments,omitempty"`
}

// Versioning holds version resolution settings.
type Versioning struct {
	Strategy       string                   `yaml:"strategy"`
	Prefix         *string                  `yaml:"prefix,omitempty"`
	InitialVersion string                   `yaml:"initial_version,omitempty"`
	Bump           string                   `yaml:"bump,omitempty"`
	Format         string                   `yaml:"format,omitempty"`
	Sprint         int                      `yaml:"sprint,omitempty"`
	TagFormat      string                   `yaml:"tag_format,omitempty"`
	Environments   map[string]EnvVersioning `yaml:"environments,omitempty"`
}

// EnvVersioning is the per-environment versioning config inside versioning.environments.
type EnvVersioning struct {
	Bump             string `yaml:"bump"`
	TagFormat        string `yaml:"tag_format,omitempty"`
	Branch           string `yaml:"branch,omitempty"`
	Source           string `yaml:"source,omitempty"`
	DisableChangelog bool   `yaml:"disable_changelog,omitempty"`
	DisableNotes     bool   `yaml:"disable_notes,omitempty"`
}

// ContentDriver configures a content generator (git-cliff, communique, cocogitto).
type ContentDriver struct {
	Generator  string `yaml:"generator"`
	Config     string `yaml:"config,omitempty"`
	Output     string `yaml:"output,omitempty"`
	TagPattern string `yaml:"tag_pattern,omitempty"`
	Template   string `yaml:"template,omitempty"`
}

// Release holds release notes and platform settings.
type Release struct {
	Notes     *ContentDriver `yaml:"notes,omitempty"`
	Platforms []Platform     `yaml:"platforms,omitempty"`
}

// Platform holds settings for one release platform (github or gitlab).
// Type is the platform discriminator ("github" or "gitlab") mapping to yaml:"platform";
// using Type avoids the Platform.Platform self-reference (ADR-0006).
type Platform struct {
	Type string `yaml:"platform"`
	// GitHub-specific
	Repository string `yaml:"repository,omitempty"`
	Draft      bool   `yaml:"draft,omitempty"`
	Prerelease bool   `yaml:"prerelease,omitempty"`
	// GitLab-specific
	Project string `yaml:"project,omitempty"`
	Catalog bool   `yaml:"catalog,omitempty"`
	// Shared
	TokenEnv string   `yaml:"token_env,omitempty"`
	Assets   []string `yaml:"assets,omitempty"`
}

// EnvOverride holds per-environment changelog and release overrides (top-level environments).
type EnvOverride struct {
	Changelog *ContentDriver `yaml:"changelog,omitempty"`
	Release   *Release       `yaml:"release,omitempty"`
}
