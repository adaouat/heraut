package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	forgeconfig "github.com/adaouat/forge/config"
	"gopkg.in/yaml.v3"
)

// ErrRemovedConfigKey reports a config key removed by the forge migration (ADR-0043).
var ErrRemovedConfigKey = errors.New("removed config key")

// removedKeys maps a removed config path to its replacement guidance.
var removedKeys = []struct{ path, hint string }{
	{"changelog.remote", "replace with a top-level `forges:` entry and point `commits.enrichment_forge` at it (this drives enrichment for `generator: native`; explicit remote pinning for `generator: git-cliff` is not carried over)"},
	{"commits.remote_metadata", "rename to `commits.enrichment_policy` (same values: disabled | optional | required)"},
}

// checkRemovedKeys reports the first removed key present in the raw YAML, with migration
// guidance. environments.<env>.changelog.remote is probed alongside the top-level keys: per-env
// remotes were explicitly supported before the forge migration (ADR-0043) removed changelog.remote,
// and without this probe they fail with a generic strict-decode error instead of the migration hint.
func checkRemovedKeys(raw []byte) error {
	var probe struct {
		Changelog struct {
			Remote any `yaml:"remote"`
		} `yaml:"changelog"`
		Commits struct {
			RemoteMetadata any `yaml:"remote_metadata"`
		} `yaml:"commits"`
		Environments map[string]struct {
			Changelog struct {
				Remote any `yaml:"remote"`
			} `yaml:"changelog"`
		} `yaml:"environments"`
	}
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return nil // malformed YAML surfaces from the strict parse with better context
	}
	present := map[string]bool{
		"changelog.remote":        probe.Changelog.Remote != nil,
		"commits.remote_metadata": probe.Commits.RemoteMetadata != nil,
	}
	for _, k := range removedKeys {
		if present[k.path] {
			return fmt.Errorf("%w: `%s` — %s", ErrRemovedConfigKey, k.path, k.hint)
		}
	}
	for _, env := range slices.Sorted(maps.Keys(probe.Environments)) {
		if probe.Environments[env].Changelog.Remote != nil {
			return fmt.Errorf("%w: `environments.%s.changelog.remote` — replace with a top-level `forges:` entry and point `commits.enrichment_forge` at it (this drives enrichment for `generator: native`; explicit remote pinning for `generator: git-cliff` is not carried over)", ErrRemovedConfigKey, env)
		}
	}
	return nil
}

// Load reads and strictly parses the config file at path, then applies heraut's
// post-parse defaults. Unknown YAML fields are rejected.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if err := checkRemovedKeys(raw); err != nil {
		return nil, err
	}
	var cfg Config
	if err := forgeconfig.Decode(bytes.NewReader(raw), &cfg); err != nil {
		return nil, err
	}
	normalize(&cfg)
	return &cfg, nil
}

// LoadFromReader strictly parses config from r, then applies heraut's defaults.
func LoadFromReader(r io.Reader) (*Config, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if err := checkRemovedKeys(raw); err != nil {
		return nil, err
	}
	var cfg Config
	if err := forgeconfig.Decode(bytes.NewReader(raw), &cfg); err != nil {
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
