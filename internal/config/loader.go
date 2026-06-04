package config

import (
	"io"

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
}
