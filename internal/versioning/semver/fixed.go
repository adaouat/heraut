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

// SetAllowReleased lets a final whose version is already tagged resolve to that version again,
// for runs that never create the tag (changelog without --tag).
func (r *Resolver) SetAllowReleased(allow bool) {
	r.allowReleased = allow
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
			if !r.allowReleased && Compare(coreVersion(t.Version), core) == 0 && !t.Version.IsPreRelease() {
				return versioning.Result{}, fmt.Errorf("%w: %s — branch version %s is already released; for a repeat build pass --set-version %s --set-build-id <id>",
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

	// A final's previous tag is the last final (ADR-0064: finals ignore pre-releases); a
	// pre-release's is the last tag of any kind.
	var previous TagVersion
	hasPrevious := false
	for _, t := range merged {
		if Compare(t.Version, candidate) < 0 && (label != "" || !t.Version.IsPreRelease()) {
			previous, hasPrevious = t, true
			break
		}
	}
	// Re-cutting the same label needs a commit since its previous tag; a higher label promotes
	// the series without one (ADR-0064).
	if label != "" && hasPrevious && previous.Version.IsPreRelease() &&
		Compare(coreVersion(previous.Version), core) == 0 && previous.Version.Pre[0] == label {
		stdout, _, err = r.runner.Run("git", "log", previous.Tag+"..HEAD", "--format=%B%x00")
		if err != nil {
			return versioning.Result{}, fmt.Errorf("reading git log: %w", err)
		}
		if len(parseCommits(stdout)) == 0 {
			return versioning.Result{}, fmt.Errorf("no commits since %s — create at least one commit before running heraut release", previous.Tag)
		}
	}
	if latest, ok := Latest(merged, false); ok && Compare(coreVersion(latest.Version), core) > 0 {
		r.warnings = append(r.warnings, fmt.Sprintf("branch version %s is lower than %s, already in this branch's history", core.Core(), latest.Tag))
	}
	return versioning.Result{
		Version:    version,
		Tag:        prefix + version,
		CurrentTag: previous.Tag,
		Bump:       versioning.BumpNone,
	}, nil
}
