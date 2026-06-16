package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// ResolveEnv resolves the --env flag value before any pipeline work. When env
// is "auto", it reads the current branch and matches it against configured
// environments' branch fields. All other values (including "") are returned
// as-is without calling git.
func ResolveEnv(env string, cfg *config.Config, runner port.Runner) (string, error) {
	if env != "auto" {
		return env, nil
	}

	if cfg == nil {
		return "", fmt.Errorf("--env auto requires a config file to detect the target environment")
	}

	strategy := cfg.Versioning.Strategy
	if strategy != "semver-per-env" && strategy != "calver-per-env" {
		return "", fmt.Errorf("--env auto requires a per-env strategy; current strategy is %q", strategy)
	}

	out, _, err := runner.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("determining current git branch: %w", err)
	}
	branch := strings.TrimSpace(out)

	if branch == "HEAD" {
		return "", fmt.Errorf("cannot auto-detect env: HEAD is detached — pass --env explicitly")
	}

	var matches []string
	for name, e := range cfg.Environments {
		if e.Branch == branch {
			matches = append(matches, name)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no env is linked to branch %q — pass --env explicitly", branch)
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		quoted := make([]string, len(matches))
		for i, m := range matches {
			quoted[i] = fmt.Sprintf("%q", m)
		}
		return "", fmt.Errorf("envs %s all use branch %q — pass --env explicitly",
			strings.Join(quoted, ", "), branch)
	}
}
