package perenv

import (
	"errors"
	"fmt"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
)

// Sentinel errors for the three per-env promotion guards (ADR-0007).
var (
	// ErrTargetExists is returned when the candidate tag already exists (E001).
	// Bypassed by --force.
	ErrTargetExists = errors.New("E001: target tag already exists")

	// ErrDestinationAhead is returned when the destination environment is already
	// at a higher version than the promotion candidate (E002). Bypassed by --force.
	ErrDestinationAhead = errors.New("E002: destination version ahead of candidate")

	// ErrNoSourceTags is returned when the source environment has no tags to
	// promote from (E003). --force has no effect.
	ErrNoSourceTags = errors.New("E003: no source tags found")
)

// VersionCalculator is implemented by the underlying single-env resolvers.
// Both methods exist on the interface; only one is called per strategy:
// BumpAuto for semver-per-env, BumpFromDate for calver-per-env (ADR-0009).
type VersionCalculator interface {
	// BumpAuto computes the next version from pre-fetched bare versions and
	// conventional commits. Used by the semver-per-env auto path.
	BumpAuto(tags []string, commits []string) (string, error)

	// BumpFromDate computes the next version from pre-fetched bare versions and
	// the configured clock. Used by the calver-per-env auto path.
	BumpFromDate(tags []string) (string, error)
}

// Resolver is the generic per-environment version resolver.
type Resolver struct {
	runner port.Runner
	cfg    *config.Config
	env    string
	force  bool
	calc   VersionCalculator
}

// New constructs a per-environment Resolver.
func New(runner port.Runner, cfg *config.Config, env string, force bool, calc VersionCalculator) *Resolver {
	return &Resolver{runner: runner, cfg: cfg, env: env, force: force, calc: calc}
}

// tagFormat returns the effective tag format for env: the env-level override when
// set, otherwise the top-level versioning.tag_format. Both auto and promote paths
// use this so that a single top-level format covers all environments.
func tagFormat(cfg *config.Config, env string) string {
	if f := cfg.Versioning.Environments[env].TagFormat; f != "" {
		return f
	}
	return cfg.Versioning.TagFormat
}

// Resolve returns the next version for the active environment.
func (r *Resolver) Resolve() (versioning.Result, error) {
	envCfg, ok := r.cfg.Versioning.Environments[r.env]
	if !ok {
		return versioning.Result{}, fmt.Errorf("environment %q not found in config", r.env)
	}

	switch envCfg.Bump {
	case "auto":
		return resolveAuto(r.runner, r.cfg, r.env, r.calc)
	case "promote":
		return resolvePromote(r.runner, r.cfg, r.env, r.force)
	default:
		return versioning.Result{}, fmt.Errorf("unknown bump mode %q for environment %q", envCfg.Bump, r.env)
	}
}
