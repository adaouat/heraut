package config

// MergeContentDriver returns the effective ContentDriver for a per-environment override
// applied over a top-level base, per ADR-0019:
//
//   - nil base → override (nothing to inherit); nil override → base.
//   - If override sets a generator that differs from base, the override fully replaces base
//     (generator-specific fields like config/template do not carry across generators).
//   - Otherwise field-by-field merge: a non-empty override field wins; an empty field
//     inherits from base.
//
// Neither argument is mutated; the result is always a fresh value (or the sole non-nil
// argument when one side is nil).
func MergeContentDriver(base, override *ContentDriver) *ContentDriver {
	if override == nil {
		return base
	}
	if base == nil {
		return override
	}
	// Generator switch → full replacement (no inheritance of generator-specific fields).
	if override.Generator != "" && override.Generator != base.Generator {
		return override
	}

	merged := *base
	if override.Generator != "" {
		merged.Generator = override.Generator
	}
	if override.Config != "" {
		merged.Config = override.Config
	}
	if override.Output != "" {
		merged.Output = override.Output
	}
	if override.TagPattern != "" {
		merged.TagPattern = override.TagPattern
	}
	if override.Template != "" {
		merged.Template = override.Template
	}
	return &merged
}
