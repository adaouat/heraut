package config

// EffectivePlatforms returns the release platforms that apply for env:
//
//   - nil cfg, or cfg.Release == nil with no env override → nil.
//   - env == "" → cfg.Release.Platforms (root).
//   - env != "": if cfg.Environments[env].Release.Platforms is non-empty, it
//     replaces the root list entirely (not merged). Otherwise — an unknown
//     env, a nil env release, or an empty env platforms list — the root list
//     is inherited unchanged.
func EffectivePlatforms(cfg *Config, env string) []Platform {
	if cfg == nil {
		return nil
	}

	var platforms []Platform
	if cfg.Release != nil {
		platforms = cfg.Release.Platforms
	}

	if env != "" {
		if envCfg, ok := cfg.Environments[env]; ok && envCfg.Release != nil && len(envCfg.Release.Platforms) > 0 {
			platforms = envCfg.Release.Platforms
		}
	}

	return platforms
}

// EffectiveTargets resolves the publish targets for env: a non-empty per-environment list replaces
// the top-level one (it does not merge), mirroring EffectivePlatforms.
func EffectiveTargets(cfg *Config, env string) []Target {
	if cfg == nil {
		return nil
	}

	var targets []Target
	if cfg.Release != nil {
		targets = cfg.Release.Targets
	}

	if env != "" {
		if envCfg, ok := cfg.Environments[env]; ok && envCfg.Release != nil && len(envCfg.Release.Targets) > 0 {
			targets = envCfg.Release.Targets
		}
	}

	return targets
}
