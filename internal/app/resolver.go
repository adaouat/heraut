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
)

// NewResolver builds the appropriate versioning.Resolver from config.
// env is the active environment name (empty for non-per-env strategies).
// force is the --force flag value.
// versionOverride is set when --version X.Y.Z is passed; when non-empty a
// StaticResolver is returned for all strategies, bypassing git calls entirely.
func NewResolver(cfg *config.Config, env string, force bool, versionOverride string, runner port.Runner) (versioning.Resolver, error) {
	if versionOverride != "" {
		// Strip any leading "v" to derive the bare version component used in commit
		// messages and changelog templates. The full tag is used as-is.
		version := strings.TrimPrefix(versionOverride, "v")
		return versioning.NewStaticResolver(versionOverride, version), nil
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
