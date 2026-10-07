package semver

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
)

const defaultPrefix = "v"
const defaultInitialVersion = "0.1.0"

// Resolver resolves the next SemVer version.
type Resolver struct {
	runner          port.Runner
	cfg             *config.Config
	versionOverride string
	allowMajor      bool
	preReleaseLabel string
	warnings        []string
	wouldBeVersions []string
	maintenance     *Range
}

// New constructs a SemVer Resolver.
func New(runner port.Runner, cfg *config.Config) *Resolver {
	return &Resolver{runner: runner, cfg: cfg}
}

// BumpAuto computes the next SemVer from pre-fetched bare versions and commits.
// tags is sorted newest-first with the prefix already stripped; commits are the
// full commit messages since the latest tag. Implements the VersionCalculator
// interface consumed by internal/versioning/perenv.
func (r *Resolver) BumpAuto(tags []string, commits []string) (string, error) {
	r.warnings = nil
	r.wouldBeVersions = nil
	if len(tags) == 0 {
		return r.initialVersion(), nil
	}
	currentVersion := tags[0]
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits since %s — create at least one commit before running heraut release", currentVersion)
	}
	bump := r.bumpAfterHold(currentVersion, commits)
	if bump == versioning.BumpNone {
		return "", noReleasableCommitsError(currentVersion, commits)
	}
	return BumpVersion(currentVersion, bump)
}

// BumpFromDate is not used by the SemVer calculator; it satisfies the
// VersionCalculator interface for internal/versioning/perenv.
func (r *Resolver) BumpFromDate(_ []string) (string, error) {
	return "", fmt.Errorf("BumpFromDate is not supported by the SemVer calculator")
}

// SetVersionOverride pins the resolved version, bypassing both auto and manual bump logic.
// The caller passes the bare version (e.g. "1.0.0"); the prefix is still applied.
func (r *Resolver) SetVersionOverride(v string) {
	r.versionOverride = v
}

// SetAllowMajor lifts versioning.bump.stay_at_v0's hold-back for this resolver (--allow-major).
func (r *Resolver) SetAllowMajor(allow bool) {
	r.allowMajor = allow
}

// SetPreRelease switches Resolve into pre-release mode for label; "" restores final-release
// resolution. The label is not validated here — see ValidatePreReleaseLabel.
func (r *Resolver) SetPreRelease(label string) {
	r.preReleaseLabel = label
}

// Warnings returns the warnings produced by the most recent Resolve or BumpAuto call — nil when
// nothing was held back. The resolver never prints them; the caller decides how to surface them.
func (r *Resolver) Warnings() []string {
	return slices.Clone(r.warnings)
}

// WouldBeVersions returns the bare "would-be" major version for each entry in Warnings, in the
// same order — nil when Warnings is empty. See holdMajorAtZero's doc comment for why this exists.
func (r *Resolver) WouldBeVersions() []string {
	return slices.Clone(r.wouldBeVersions)
}

// bumpAfterHold is DetermineBump plus versioning.bump.stay_at_v0 (ADR-0063), recording any
// warning for Warnings.
func (r *Resolver) bumpAfterHold(currentVersion string, commits []string) versioning.BumpType {
	overrides := r.cfg.Versioning.BumpOverrides()
	bump := DetermineBump(commits, overrides)
	if !r.cfg.Versioning.StayAtV0() || r.allowMajor {
		return bump
	}
	held, warning, wouldBe := holdMajorAtZero(currentVersion, bump, commits, overrides)
	if warning != "" {
		r.warnings = append(r.warnings, warning)
		r.wouldBeVersions = append(r.wouldBeVersions, wouldBe)
	}
	return held
}

// Resolve returns the next version result.
// An explicit versionOverride (set via SetVersionOverride) always takes precedence over
// the configured bump mode — this allows --set-version to short-circuit auto resolution.
func (r *Resolver) Resolve() (versioning.Result, error) {
	r.warnings = nil
	r.wouldBeVersions = nil
	if r.versionOverride != "" || r.cfg.Versioning.BumpMode() == "manual" {
		return r.resolveManual()
	}
	if r.preReleaseLabel != "" {
		return r.resolvePreRelease()
	}
	return r.resolveAuto()
}

func (r *Resolver) resolveManual() (versioning.Result, error) {
	if r.versionOverride == "" {
		return versioning.Result{}, fmt.Errorf("manual bump mode requires --set-version flag")
	}
	prefix := r.prefix()
	// Strip the prefix if the caller passed the full tag (e.g. from `heraut version next`)
	// so that --set-version v1.0.0 and --set-version 1.0.0 both produce the tag v1.0.0.
	version := strings.TrimPrefix(r.versionOverride, prefix)
	return versioning.Result{
		Version: version,
		Tag:     prefix + version,
		Bump:    versioning.BumpNone,
	}, nil
}

func (r *Resolver) resolveAuto() (versioning.Result, error) {
	if r.maintenance != nil {
		return r.resolveMaintenance(r.maintenance)
	}
	prefix := r.prefix()

	stdout, _, err := r.runner.Run("git", "tag", "-l", prefix+"*", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing git tags: %w", err)
	}

	// Order by SemVer §11 in Go rather than trusting git's version:refname sort, which puts
	// pre-releases above their release without versionsort.suffix (ADR-0064). Pre-release tags are
	// never the bump base; a build-metadata-only tag (v1.4.0+5) is the release of its core.
	sorted := SortTags(parseTags(stdout), func(tag string) (string, bool) {
		return strings.CutPrefix(tag, prefix)
	})
	latest, ok := Latest(sorted, false)
	if !ok {
		iv := r.initialVersion()
		return versioning.Result{
			Version: iv,
			Tag:     prefix + iv,
			Bump:    versioning.BumpNone,
		}, nil
	}
	currentTag, currentVersion := latest.Tag, latest.Version.Core()

	// %B gives the full commit message; %x00 is a null-byte separator between commits.
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

	return versioning.Result{
		Version:    nextVersion,
		Tag:        prefix + nextVersion,
		CurrentTag: currentTag,
		Bump:       bump,
	}, nil
}

func (r *Resolver) prefix() string {
	if r.cfg.Versioning.TagPrefix != nil {
		return *r.cfg.Versioning.TagPrefix
	}
	return defaultPrefix
}

func (r *Resolver) initialVersion() string {
	if r.cfg.Versioning.InitialVersion != "" {
		return r.cfg.Versioning.InitialVersion
	}
	return defaultInitialVersion
}

func parseTags(stdout string) []string {
	var tags []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tags = append(tags, line)
		}
	}
	return tags
}

func parseCommits(stdout string) []string {
	var commits []string
	for _, chunk := range strings.Split(stdout, "\x00") {
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			commits = append(commits, chunk)
		}
	}
	return commits
}

// noReleasableCommitsError reports that commits exist since currentTag but every one of them was
// excluded from the bump (T261) — distinct from "no commits at all" (parseCommits/BumpAuto's own
// empty-commits check). Lists each excluded commit's subject line so the user can see why.
func noReleasableCommitsError(currentTag string, commits []string) error {
	lines := make([]string, len(commits))
	for i, c := range commits {
		lines[i] = "  - " + firstLine(c)
	}
	return fmt.Errorf(
		"no releasable commits since %s: %d commit(s) since then are excluded from the version bump\n%s",
		currentTag, len(commits), strings.Join(lines, "\n"),
	)
}
