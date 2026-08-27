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
// per-environment override merges onto the top-level config (ADR-0019, MergeContentDriver).
// release: presence — top-level or per-environment — always default-populates Notes to a
// non-nil, empty ContentDriver when neither level sets one explicitly (ADR-0046 release-block
// atomicity applies per-environment exactly like it does at the root; the loader's normalize()
// only reaches the top-level block, so the per-environment default has to happen here). A nil
// result means release: was not configured at all for this environment, at either level.
func EffectiveReleaseNotes(cfg *Config, env string) *ContentDriver {
	if cfg == nil {
		return nil
	}

	var notes *ContentDriver
	released := cfg.Release != nil
	if released {
		notes = cfg.Release.Notes
	}

	if env != "" {
		if envCfg, ok := cfg.Environments[env]; ok && envCfg.Release != nil {
			released = true
			if envCfg.Release.Notes != nil {
				notes = MergeContentDriver(notes, envCfg.Release.Notes)
			}
		}
	}

	if released && notes == nil {
		notes = &ContentDriver{}
	}

	return notes
}
