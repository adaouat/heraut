package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

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
	return "", fmt.Errorf("no tags found for %q", glob)
}

func currentTagGlob(cfg *config.Config, env string) (string, error) {
	prefix := func() string {
		if cfg.Versioning.Prefix != nil {
			return *cfg.Versioning.Prefix
		}
		return ""
	}

	switch cfg.Versioning.Strategy {
	case "semver":
		p := "v"
		if cfg.Versioning.Prefix != nil {
			p = *cfg.Versioning.Prefix
		}
		return p + "*", nil
	case "calver":
		return prefix() + "*", nil
	case "semver-per-env", "calver-per-env":
		if env == "" {
			return "", fmt.Errorf("--env is required for %s strategy", cfg.Versioning.Strategy)
		}
		envCfg, ok := cfg.Environments[env]
		if !ok {
			return "", fmt.Errorf("environment %q not found in config", env)
		}
		return tagfmt.GlobPattern(envCfg.TagFormat, env)
	default:
		return "", fmt.Errorf("unknown versioning strategy %q", cfg.Versioning.Strategy)
	}
}
