package config

import (
	"io"
	"strings"

	forgeconfig "github.com/adaouat/forge/config"
)

// Load reads and strictly parses the config file at path, then applies heraut's
// post-parse defaults. Unknown YAML fields are rejected.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := forgeconfig.Load(path, &cfg); err != nil {
		return nil, err
	}
	normalize(&cfg)
	return &cfg, nil
}

// LoadFromReader strictly parses config from r, then applies heraut's defaults.
func LoadFromReader(r io.Reader) (*Config, error) {
	var cfg Config
	if err := forgeconfig.Decode(r, &cfg); err != nil {
		return nil, err
	}
	normalize(&cfg)
	return &cfg, nil
}

// normalize applies post-parse defaults that cannot be expressed in the YAML schema.
func normalize(cfg *Config) {
	if cfg.Changelog != nil && cfg.Changelog.Output == "" {
		cfg.Changelog.Output = "CHANGELOG.md"
	}
	if cfg.Release != nil {
		normalizePlatforms(cfg.Release.Platforms)
	}
	for _, env := range cfg.Environments {
		if env.Release != nil {
			normalizePlatforms(env.Release.Platforms)
		}
	}
}

// normalizePlatforms trims a trailing slash from each platform's base_url and fills in
// the per-type default when it is empty, so cfg.Platform.BaseURL is the single
// trailing-slash-free source of truth for that platform's host (ADR-0020).
func normalizePlatforms(plats []Platform) {
	for i := range plats {
		plats[i].BaseURL = strings.TrimRight(plats[i].BaseURL, "/")
		if plats[i].BaseURL == "" {
			plats[i].BaseURL = DefaultBaseURL(plats[i].Type)
		}
	}
}
