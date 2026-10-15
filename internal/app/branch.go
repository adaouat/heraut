package app

import (
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning/branch"
)

// The branch rules live in internal/versioning/branch; cmd reaches them through app.
var (
	ErrUnlistedBranch   = branch.ErrUnlistedBranch
	ErrAmbiguousBranch  = branch.ErrAmbiguousBranch
	ErrUnderivableRange = branch.ErrUnderivableRange
)

// CheckBranch enforces the per-environment branch guard (see branch.CheckBranch).
func CheckBranch(runner port.Runner, cfg *config.Config, env string, force bool) error {
	return branch.CheckBranch(runner, cfg, env, force)
}

// CheckReleaseBranch enforces versioning.branches for a release (see branch.CheckReleaseBranch).
func CheckReleaseBranch(runner port.Runner, cfg *config.Config, force bool) error {
	return branch.CheckReleaseBranch(runner, cfg, force)
}
