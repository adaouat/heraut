package app

import (
	"fmt"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

// NewResolver builds the appropriate versioning.Resolver from config.
// env is the active environment name (empty for non-per-env strategies).
// force is the --force flag value.
// versionOverride is set when --version X.Y.Z is passed.
func NewResolver(cfg *config.Config, env string, force bool, versionOverride string, runner port.Runner) (versioning.Resolver, error) {
	switch cfg.Versioning.Strategy {
	case "semver":
		r := semver.New(runner, cfg)
		if versionOverride != "" {
			r.SetVersionOverride(versionOverride)
		}
		return r, nil
	default:
		return nil, fmt.Errorf("unknown versioning strategy %q (supported: semver)", cfg.Versioning.Strategy)
	}
}
