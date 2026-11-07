package branch

import (
	"fmt"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// BranchMismatchError is returned by CheckBranch when the current branch is not the one the
// environment is linked to.
type BranchMismatchError struct {
	Env, Want, Current string
}

func (e *BranchMismatchError) Error() string {
	return fmt.Sprintf("environment %q must be operated from branch %q, but the current branch is %q (use --force to override)",
		e.Env, e.Want, e.Current)
}

// CheckBranch enforces the per-environment branch guard: when the active environment
// declares a branch, the current git branch must match it. This prevents operating on
// an environment (e.g. releasing prod) from the wrong branch. It is a no-op when env is
// empty or the environment has no branch set. force bypasses the check entirely. On a detached
// checkout the branch is the one the CI job runs for (CurrentBranch).
func CheckBranch(runner port.Runner, cfg *config.Config, env string, force bool) error {
	if env == "" || force {
		return nil
	}
	envCfg, ok := cfg.Environments[env]
	if !ok || envCfg.Branch == "" {
		return nil
	}
	current, ok, err := CurrentBranch(runner)
	if err != nil {
		return err
	}
	if !ok {
		current = "HEAD"
	}
	if current != envCfg.Branch {
		return &BranchMismatchError{Env: env, Want: envCfg.Branch, Current: current}
	}
	return nil
}
