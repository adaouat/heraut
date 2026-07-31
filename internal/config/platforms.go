package config

// EffectiveTargets resolves the publish targets for env: a non-empty per-environment list replaces
// the top-level one (it does not merge).
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
