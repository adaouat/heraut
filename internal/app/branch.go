package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// CheckBranch enforces the per-environment branch guard: when the active environment
// declares a branch, the current git branch must match it. This prevents operating on
// an environment (e.g. releasing prod) from the wrong branch. It is a no-op when env is
// empty or the environment has no branch set. force bypasses the check entirely.
func CheckBranch(runner port.Runner, cfg *config.Config, env string, force bool) error {
	if env == "" || force {
		return nil
	}
	envCfg, ok := cfg.Environments[env]
	if !ok || envCfg.Branch == "" {
		return nil
	}
	out, _, err := runner.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("determining current git branch: %w", err)
	}
	current := strings.TrimSpace(out)
	if current != envCfg.Branch {
		return fmt.Errorf("environment %q must be operated from branch %q, but the current branch is %q (use --force to override)",
			env, envCfg.Branch, current)
	}
	return nil
}
