package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

// errNoTagsFound is the sentinel CurrentTag returns when no tags match the resolved glob.
var errNoTagsFound = errors.New("no tags found")

// CurrentTag returns the latest existing git tag for the given strategy and environment.
// For single-env strategies, env is ignored. For per-env strategies, env is required.
func CurrentTag(runner port.Runner, cfg *config.Config, env string) (string, error) {
	glob, err := currentTagGlob(cfg, env)
	if err != nil {
		return "", err
	}

	stdout, _, err := runner.Run("git", "tag", "-l", glob, "--sort=-version:refname")
	if err != nil {
		return "", fmt.Errorf("listing git tags: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("%w for %q", errNoTagsFound, glob)
}

// CurrentVersion returns the bare semantic version of the latest tag (the tag with
// any prefix / env / build components stripped). For per-env strategies the version is
// parsed via the effective tag_format; for single-env strategies the tag prefix is
// stripped.
func CurrentVersion(runner port.Runner, cfg *config.Config, env string) (string, error) {
	tag, err := CurrentTag(runner, cfg, env)
	if err != nil {
		return "", err
	}
	switch cfg.Versioning.Strategy {
	case "semver":
		prefix := "v"
		if cfg.Versioning.TagPrefix != nil {
			prefix = *cfg.Versioning.TagPrefix
		}
		return strings.TrimPrefix(tag, prefix), nil
	case "calver":
		prefix := ""
		if cfg.Versioning.TagPrefix != nil {
			prefix = *cfg.Versioning.TagPrefix
		}
		return strings.TrimPrefix(tag, prefix), nil
	case "semver-per-env", "calver-per-env":
		v, err := tagfmt.ParseVersion(cfg.EffectiveTagFormat(env), tag)
		if err != nil {
			return "", fmt.Errorf("parsing version from tag %q: %w", tag, err)
		}
		return v, nil
	default:
		return "", fmt.Errorf("unknown versioning strategy %q", cfg.Versioning.Strategy)
	}
}

func currentTagGlob(cfg *config.Config, env string) (string, error) {
	prefix := func() string {
		if cfg.Versioning.TagPrefix != nil {
			return *cfg.Versioning.TagPrefix
		}
		return ""
	}

	switch cfg.Versioning.Strategy {
	case "semver":
		p := "v"
		if cfg.Versioning.TagPrefix != nil {
			p = *cfg.Versioning.TagPrefix
		}
		return p + "*", nil
	case "calver":
		return prefix() + "*", nil
	case "semver-per-env", "calver-per-env":
		if env == "" {
			return "", fmt.Errorf("--env is required for %s strategy", cfg.Versioning.Strategy)
		}
		if _, ok := cfg.Environments[env]; !ok {
			return "", fmt.Errorf("environment %q not found in config", env)
		}
		return tagfmt.GlobPattern(cfg.EffectiveTagFormat(env), env)
	default:
		return "", fmt.Errorf("unknown versioning strategy %q", cfg.Versioning.Strategy)
	}
}
