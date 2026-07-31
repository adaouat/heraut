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

// releasePlatformsHint is the migration guidance for release.platforms, shared by the top-level
// and per-env probes: declare a forges: entry carrying base_url/token_env/repository-or-project,
// then reference it from release.targets[].forge, keeping draft/prerelease/assets on the target.
const releasePlatformsHint = "declare a `forges:` entry carrying `base_url` / `token_env` / `repository`-or-`project`, then reference it from `release.targets[].forge`, keeping `draft` / `prerelease` / `assets` on the target"

// removedKeys maps a removed config path to its replacement guidance.
var removedKeys = []struct{ path, hint string }{
	{"changelog.remote", "replace with a top-level `forges:` entry and point `commits.enrichment_forge` at it (this drives enrichment for `generator: native`; explicit remote pinning for `generator: git-cliff` is not carried over)"},
	{"commits.remote_metadata", "rename to `commits.enrichment_policy` (same values: disabled | optional | required)"},
	{"release.platforms", releasePlatformsHint},
}

// checkRemovedKeys reports the first removed key present in the raw YAML, with migration
// guidance. environments.<env>.changelog.remote and environments.<env>.release.platforms are
// probed alongside the top-level keys: both were explicitly supported before the forge migration
// (ADR-0043) removed their top-level counterparts, and without this probe they fail with a
// generic strict-decode error instead of the migration hint.
func checkRemovedKeys(raw []byte) error {
	var probe struct {
		Changelog struct {
			Remote any `yaml:"remote"`
		} `yaml:"changelog"`
		Commits struct {
			RemoteMetadata any `yaml:"remote_metadata"`
		} `yaml:"commits"`
		Release struct {
			Platforms any `yaml:"platforms"`
		} `yaml:"release"`
		Environments map[string]struct {
			Changelog struct {
				Remote any `yaml:"remote"`
			} `yaml:"changelog"`
			Release struct {
				Platforms any `yaml:"platforms"`
			} `yaml:"release"`
		} `yaml:"environments"`
	}
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return nil // malformed YAML surfaces from the strict parse with better context
	}
	present := map[string]bool{
		"changelog.remote":        probe.Changelog.Remote != nil,
		"commits.remote_metadata": probe.Commits.RemoteMetadata != nil,
		"release.platforms":       probe.Release.Platforms != nil,
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
		if probe.Environments[env].Release.Platforms != nil {
			return fmt.Errorf("%w: `environments.%s.release.platforms` — %s", ErrRemovedConfigKey, env, releasePlatformsHint)
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
	normalizeForges(cfg.Forges)
}

// normalizeForges trims a trailing slash from each forge's base_url and api_url, mirroring
// the deleted normalizePlatforms (ADR-0020/ADR-0043). Unlike normalizePlatforms, it does NOT
// fill a per-type default host when base_url is empty: internal/forge.Resolve already applies
// defaultHostFor(f.Type) as the last-resort fallback in its resolution precedence
// (explicit config -> CI env -> git origin -> type default), so filling it here would just be
// a redundant, earlier application of the same default with none of Resolve's other sources.
func normalizeForges(forges []Forge) {
	for i := range forges {
		forges[i].BaseURL = strings.TrimRight(forges[i].BaseURL, "/")
		forges[i].APIURL = strings.TrimRight(forges[i].APIURL, "/")
	}
}
