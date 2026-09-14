package config

import "maps"

// MergeContentDriver returns the effective ContentDriver for a per-environment override applied
// over a top-level base, per ADR-0019: nil base -> override (nothing to inherit); nil override ->
// base; otherwise field-by-field merge — a non-empty override field wins, an empty field inherits
// from base. (Prior to T177/T182, a generator switch between base and override triggered a full
// replacement instead of a field merge; that branch is gone now that Generator can never be
// non-empty — native is the only generator.)
//
// Neither argument is mutated; the result is always a fresh value (or the sole non-nil argument
// when one side is nil).
func MergeContentDriver(base, override *ContentDriver) *ContentDriver {
	if override == nil {
		return base
	}
	if base == nil {
		return override
	}
	merged := *base
	if override.Output != "" {
		merged.Output = override.Output
	}
	if override.TagPattern != "" {
		merged.TagPattern = override.TagPattern
	}
	if override.Template != "" {
		merged.Template = override.Template
	}
	merged.Rendering = mergeRendering(base.Rendering, override.Rendering)
	return &merged
}

// mergeRendering deep-merges a per-env rendering override over a base: Excludes are replaced
// wholesale when the override sets them; Templates merges key-by-key and Commit.Trailers merges
// by token (override wins per key/token, unset keys/tokens inherit — ADR-0057 for Trailers,
// relocated under Commit by ADR-0060). A nil side contributes nothing; both nil yields nil.
func mergeRendering(base, override *Rendering) *Rendering {
	if override == nil {
		return base
	}
	if base == nil {
		return override
	}
	merged := *base
	if len(override.Excludes) > 0 {
		merged.Excludes = override.Excludes
	}
	if len(override.Templates) > 0 {
		templates := make(TemplateOverrides, len(base.Templates)+len(override.Templates))
		maps.Copy(templates, base.Templates)
		maps.Copy(templates, override.Templates)
		merged.Templates = templates
	}
	merged.Commit = mergeRenderingCommit(base.Commit, override.Commit)
	return &merged
}

// mergeRenderingCommit deep-merges a per-env rendering.commit override over a base: Trailers
// merges by token (ADR-0057), the same override-wins-per-key shape as Templates. A nil side
// contributes nothing; both nil yields nil.
func mergeRenderingCommit(base, override *RenderingCommit) *RenderingCommit {
	if override == nil {
		return base
	}
	if base == nil {
		return override
	}
	merged := *base
	if len(override.Trailers) > 0 {
		merged.Trailers = MergeFooterRules(base.Trailers, override.Trailers)
	}
	return &merged
}
