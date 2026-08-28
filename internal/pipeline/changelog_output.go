package pipeline

import "github.com/adaouat/heraut/internal/port"

// outputPathReporter is implemented by a port.Generator wrapper that knows the concrete file path
// its last Generate call actually wrote. Needed when changelog.output contains rotation tokens
// (e.g. "CHANGELOG_{YYYY}.md"): the concrete path can only be resolved once the tag is known,
// which is after ChangelogFile is set at config-build time — see
// docs/superpowers/specs/2026-08-28-changelog-rotation-design.md §3. Structural: internal/pipeline
// never imports internal/app, the package that implements this.
type outputPathReporter interface {
	LastOutputPath() string
}

// resolvedChangelogFile returns the concrete changelog path for commit/display purposes: gen's
// own last-resolved path when it implements outputPathReporter, falling back to configured
// (defaulting to "CHANGELOG.md" when unset) otherwise. Every config without rotation tokens is
// unaffected — gen never implements outputPathReporter in that case, and dry-run output (which
// never calls Generate) also falls back here since LastOutputPath has nothing to report yet.
func resolvedChangelogFile(gen port.Generator, configured string) string {
	file := configured
	if file == "" {
		file = "CHANGELOG.md"
	}
	if r, ok := gen.(outputPathReporter); ok {
		if path := r.LastOutputPath(); path != "" {
			file = path
		}
	}
	return file
}
