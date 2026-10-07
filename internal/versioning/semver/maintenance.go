package semver

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
)

var (
	ErrOutOfRange       = errors.New("next version is outside the maintenance range")
	ErrTagExists        = errors.New("tag already exists")
	ErrNoInRangeRelease = errors.New("no release in the maintenance range is reachable from HEAD")
)

// Range is a maintenance line [Lo, Hi) (ADR-0065).
type Range struct {
	Lo, Hi Version
	Label  string // "1.3.x"
	Branch string // "release/1.3"
}

// RangeFrom turns a configured branch range into the [Lo, Hi) interval it covers: N.x is
// [N.0.0, N+1.0.0), N.M.x is [N.M.0, N.M+1.0).
func RangeFrom(r config.BranchRange, branch string) Range {
	rg := Range{Label: r.String(), Branch: branch}
	if r.Minor == nil {
		rg.Lo = Version{Major: r.Major}
		rg.Hi = Version{Major: r.Major + 1}
		return rg
	}
	rg.Lo = Version{Major: r.Major, Minor: *r.Minor}
	rg.Hi = Version{Major: r.Major, Minor: *r.Minor + 1}
	return rg
}

// Contains reports whether v's core lies in [Lo, Hi). Pre-release and build metadata are
// ignored: a line owns every pre-release of the cores it owns.
func (r Range) Contains(v Version) bool {
	core := Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
	return Compare(core, r.Lo) >= 0 && Compare(core, r.Hi) < 0
}

// SetMaintenanceRange confines auto resolution to rg's line; nil restores global resolution.
func (r *Resolver) SetMaintenanceRange(rg *Range) {
	r.maintenance = rg
}

// resolveMaintenance resolves the next final version on a maintenance line: the base is the
// highest in-range release reachable from HEAD, never a higher tag cut on another line.
func (r *Resolver) resolveMaintenance(rg *Range) (versioning.Result, error) {
	prefix := r.prefix()

	stdout, _, err := r.runner.Run("git", "tag", "-l", prefix+"*", "--merged", "HEAD", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing git tags merged into HEAD: %w", err)
	}
	sorted := SortTags(parseTags(stdout), func(tag string) (string, bool) {
		return strings.CutPrefix(tag, prefix)
	})
	base, err := inRangeBase(rg, sorted)
	if err != nil {
		return versioning.Result{}, err
	}
	currentTag, currentVersion := base.Tag, base.Version.Core()

	stdout, _, err = r.runner.Run("git", "log", currentTag+"..HEAD", "--format=%B%x00")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("reading git log: %w", err)
	}
	commits := parseCommits(stdout)
	if len(commits) == 0 {
		return versioning.Result{}, fmt.Errorf("no commits since %s — create at least one commit before running heraut release", currentTag)
	}

	bump := r.bumpAfterHold(currentVersion, commits)
	if bump == versioning.BumpNone {
		return versioning.Result{}, noReleasableCommitsError(currentTag, commits)
	}
	nextVersion, err := BumpVersion(currentVersion, bump)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("bumping version: %w", err)
	}
	next, err := Parse(nextVersion)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("parsing next version %s: %w", nextVersion, err)
	}
	if !rg.Contains(next) {
		return versioning.Result{}, r.outOfRangeError(rg, commits, bump, nextVersion)
	}

	nextTag := prefix + nextVersion
	stdout, _, err = r.runner.Run("git", "tag", "-l", nextTag)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("checking for existing tag %s: %w", nextTag, err)
	}
	if strings.TrimSpace(stdout) != "" {
		return versioning.Result{}, fmt.Errorf("%w: %s (cut on another branch) — pick the next free version with --set-version",
			ErrTagExists, nextTag)
	}

	return versioning.Result{
		Version:    nextVersion,
		Tag:        nextTag,
		CurrentTag: currentTag,
		Bump:       bump,
	}, nil
}

// inRangeBase is the highest final release of sorted (merged into HEAD, precedence order) whose
// core lies in rg.
func inRangeBase(rg *Range, sorted []TagVersion) (TagVersion, error) {
	for _, tv := range sorted {
		if !tv.Version.IsPreRelease() && rg.Contains(tv.Version) {
			return tv, nil
		}
	}
	return TagVersion{}, fmt.Errorf("%w: no release in range %s in the history of %s — tag the branch's starting point or pass --set-version",
		ErrNoInRangeRelease, rg.Label, rg.Branch)
}

func (r *Resolver) outOfRangeError(rg *Range, commits []string, bump versioning.BumpType, version string) error {
	return fmt.Errorf("%w: %s would release %s, outside %s (>=%s <%s) — land it on a branch whose range allows it, or on main",
		ErrOutOfRange, bumpSubject(commits, r.cfg.Versioning.BumpOverrides(), bump), version, rg.Branch, rg.Lo, rg.Hi)
}

// bumpSubject names the commit responsible for bump: the first at that level, else the first
// commit at all (a stay_at_v0 hold can leave no commit at the applied level).
func bumpSubject(commits []string, overrides []config.BumpRule, bump versioning.BumpType) string {
	if subjects := commitsAtLevel(commits, overrides, bump); len(subjects) > 0 && subjects[0] != "" {
		return subjects[0]
	}
	return firstLine(commits[0])
}
