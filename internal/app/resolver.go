package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/calver"
	"github.com/adaouat/heraut/internal/versioning/perenv"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

// NewResolver builds the appropriate versioning.Resolver from config.
// env is the active environment name (empty for non-per-env strategies).
// force is the --force flag value.
// versionOverride is set when --version X.Y.Z is passed; when non-empty a
// StaticResolver is returned for all strategies, bypassing git calls entirely.
// buildID is set when --build <id> is passed; requires versionOverride to be set.
func NewResolver(cfg *config.Config, env string, force bool, versionOverride, buildID string, runner port.Runner) (versioning.Resolver, error) {
	if buildID != "" && versionOverride == "" {
		return nil, fmt.Errorf("--build requires --version: build ID cannot be combined with automatic version resolution")
	}
	if versionOverride != "" {
		var tf string
		if buildID != "" {
			var err error
			tf, err = effectiveTagFmt(cfg, env)
			if err != nil {
				return nil, err
			}
		} else {
			tf = cfg.EffectiveTagFormat(env)
		}

		var tag, version string
		if tf != "" {
			// Strip any leading "v" to derive the bare version component fed into the
			// {version} token — a tag_format template has no single "prefix" to strip,
			// so this heuristic (not the configured tag_prefix) applies here.
			version = strings.TrimPrefix(versionOverride, "v")
			var err error
			tag, err = tagfmt.Render(tf, env, version, buildID)
			if err != nil {
				return nil, fmt.Errorf("rendering tag: %w", err)
			}
		} else {
			prefix := defaultTagPrefix(cfg.Versioning.Strategy)
			if cfg.Versioning.TagPrefix != nil {
				prefix = *cfg.Versioning.TagPrefix
			}
			version = strings.TrimPrefix(versionOverride, prefix)
			tag = prefix + version
		}
		return versioning.NewStaticResolver(tag, version), nil
	}

	switch cfg.Versioning.Strategy {
	case "semver":
		return semver.New(runner, cfg), nil
	case "calver":
		return calver.New(runner, cfg, time.Now), nil
	case "semver-per-env":
		calc := semver.New(nil, cfg)
		return perenv.New(runner, cfg, env, force, calc), nil
	case "calver-per-env":
		calc := calver.New(nil, cfg, time.Now)
		return perenv.New(runner, cfg, env, force, calc), nil
	default:
		return nil, fmt.Errorf("unknown versioning strategy %q (supported: semver, calver, semver-per-env, calver-per-env)", cfg.Versioning.Strategy)
	}
}

// ValidateBuildID reports whether a --build value is usable as a tag component.
// Delegates to tagfmt so cmd does not import the versioning layer directly.
func ValidateBuildID(build string) error {
	return tagfmt.ValidateBuildID(build)
}

// ValidateVersionOverride reports whether a --version value is usable as a
// tag/version override. Delegates to tagfmt so cmd does not import the
// versioning layer directly.
func ValidateVersionOverride(version string) error {
	return tagfmt.ValidateVersionOverride(version)
}

// defaultTagPrefix mirrors the unexported default each strategy resolver falls back to when
// versioning.tag_prefix is unset (semver.Resolver.prefix, calver.Resolver.prefix): "v" for
// SemVer-based strategies, "" for CalVer-based ones, where a version like 2026.05.3 already
// reads as a tag without help from a prefix.
func defaultTagPrefix(strategy string) string {
	switch strategy {
	case "semver", "semver-per-env":
		return "v"
	default:
		return ""
	}
}

// effectiveTagFmt returns the tag format to use for build ID rendering and
// validates that {build} is present (required when --build is passed). The
// env-override → top-level resolution lives in config.EffectiveTagFormat.
func effectiveTagFmt(cfg *config.Config, env string) (string, error) {
	tf := cfg.EffectiveTagFormat(env)
	if tf == "" {
		return "", fmt.Errorf("--build requires versioning.tag_format to contain a {build} token, but tag_format is not set")
	}
	if !strings.Contains(tf, "{build}") {
		return "", fmt.Errorf("--build requires a {build} token in versioning.tag_format (got %q)", tf)
	}
	return tf, nil
}
