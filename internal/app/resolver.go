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
		// Strip any leading "v" to derive the bare version component used in commit
		// messages and changelog templates. The full tag is used as-is.
		version := strings.TrimPrefix(versionOverride, "v")
		tag := versionOverride
		if buildID != "" {
			tf, err := effectiveTagFmt(cfg, env)
			if err != nil {
				return nil, err
			}
			tag, err = tagfmt.Render(tf, env, version, buildID)
			if err != nil {
				return nil, fmt.Errorf("rendering tag with build ID: %w", err)
			}
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
