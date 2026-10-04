package app

import (
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

// isPreReleaseRun reports whether this run publishes a SemVer pre-release, decided before
// resolution so step totals and the changelog skip are fixed at build time (ADR-0064).
func isPreReleaseRun(cfg *config.Config, env string, opts PipelineOpts) bool {
	if opts.PreReleaseLabel != "" {
		return true
	}
	strategy := cfg.Versioning.Strategy
	if opts.VersionOverride == "" || (strategy != "semver" && strategy != "semver-per-env") {
		return false
	}
	version := opts.VersionOverride
	if cfg.EffectiveTagFormat(env) != "" {
		version = strings.TrimPrefix(version, "v")
	} else {
		version = strings.TrimPrefix(version, configuredTagPrefix(cfg))
	}
	v, err := semver.Parse(version)
	return err == nil && v.IsPreRelease()
}
