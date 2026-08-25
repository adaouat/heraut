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

// EffectiveReleaseNotes resolves the release-notes ContentDriver for env: a non-empty
// per-environment override merges onto the top-level config (ADR-0019, MergeContentDriver). A nil
// result means release-notes generation was not configured at all — the signal T214 uses to tell
// "notes only, no publish" apart from zero-config publishing.
func EffectiveReleaseNotes(cfg *Config, env string) *ContentDriver {
	if cfg == nil {
		return nil
	}

	var notes *ContentDriver
	if cfg.Release != nil {
		notes = cfg.Release.Notes
	}

	if env != "" {
		if envCfg, ok := cfg.Environments[env]; ok && envCfg.Release != nil && envCfg.Release.Notes != nil {
			notes = MergeContentDriver(notes, envCfg.Release.Notes)
		}
	}

	return notes
}
