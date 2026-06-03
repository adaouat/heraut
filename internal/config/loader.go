package config

import (
	"fmt"
	"io"
	"os"

	forgeconfig "github.com/adaouat/forge/config"
)

// Load reads and strictly parses the config file at path.
// Unknown YAML fields are rejected.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close() //nolint:errcheck
	return LoadFromReader(f)
}

// LoadFromReader strictly parses config from r via forge's loader, then applies
// heraut's post-parse defaults. Unknown YAML fields are rejected.
func LoadFromReader(r io.Reader) (*Config, error) {
	var cfg Config
	if err := forgeconfig.Decode(r, &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
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
