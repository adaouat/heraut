package semver

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/versioning"
)

// SetFixedVersion pins resolution to core (MAJOR.MINOR.PATCH), the version a version branch
// carries in its name (ADR-0065): commits never bump it, only the pre-release label or the
// final decides the tag. "" restores normal resolution. --set-version and manual bump mode still
// win; Resolve never reaches this path for them.
func (r *Resolver) SetFixedVersion(core string) {
	r.fixedCore = core
}

func (r *Resolver) resolveFixed() (versioning.Result, error) {
	core, err := Parse(r.fixedCore)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("parsing version branch version %q: %w", r.fixedCore, err)
	}
	prefix, label := r.prefix(), r.preReleaseLabel
	extract := func(tag string) (string, bool) { return strings.CutPrefix(tag, prefix) }

	stdout, _, err := r.runner.Run("git", "tag", "-l", prefix+"*", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing git tags: %w", err)
	}
	all := SortTags(parseTags(stdout), extract)
	stdout, _, err = r.runner.Run("git", "tag", "-l", prefix+"*", "--merged", "HEAD", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing merged git tags: %w", err)
	}
	merged := SortTags(parseTags(stdout), extract)

	version := core.Core()
	if label == "" {
		for _, t := range all {
			// A build-metadata-only tag is the release of its core (ADR-0064).
			if Compare(coreVersion(t.Version), core) == 0 && !t.Version.IsPreRelease() {
				return versioning.Result{}, fmt.Errorf("%w: %s — branch version %s is already released; pass --pre-release only before the final, or --set-version %s --set-build-id <id> for a repeat build",
					ErrTagExists, t.Tag, version, version)
			}
		}
	} else {
		version = fmt.Sprintf("%s-%s.%d", version, label, nextCounter(all, core, label))
	}
	candidate, err := Parse(version)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building version %q: %w", version, err)
	}
	if label != "" {
		for _, t := range all {
			if Compare(coreVersion(t.Version), core) == 0 && Compare(candidate, t.Version) <= 0 {
				return versioning.Result{}, &sentinelError{
					sentinel: ErrPreReleaseRegression,
					msg: fmt.Sprintf("%s%s would sort below existing %s — a pre-release of branch version %s cannot follow it (already released or a higher label exists)",
						prefix, version, t.Tag, core.Core()),
				}
			}
		}
	}

	var previous string
	for _, t := range merged {
		if Compare(t.Version, candidate) < 0 {
			previous = t.Tag
			break
		}
	}
	if latest, ok := Latest(merged, false); ok && Compare(coreVersion(latest.Version), core) > 0 {
		r.warnings = append(r.warnings, fmt.Sprintf("branch version %s is lower than %s, already in this branch's history", core.Core(), latest.Tag))
	}
	return versioning.Result{
		Version:    version,
		Tag:        prefix + version,
		CurrentTag: previous,
		Bump:       versioning.BumpNone,
	}, nil
}
