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
	allowMajor      bool
	preReleaseLabel string
	warnings        []string
	wouldBeVersions []string
	unconfirmed     []string
	maintenance     *Range
	fixedCore       string
	allowReleased   bool
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
	r.reset()
	if len(tags) == 0 {
		return r.initialVersion(), nil
	}
	currentVersion := tags[0]
	if len(commits) == 0 {
		return "", versioning.NothingToReleasef("no commits since %s — create at least one commit before running heraut release", currentVersion)
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

// UnconfirmedMajor returns the subject of every commit that forced the major bump of the most
// recent Resolve or BumpAuto call when versioning.bump.confirm_major asks for --allow-major before
// it (ADR-0078) — nil when there is nothing to confirm. The resolver still returns the major; the
// commands decide whether to refuse it.
func (r *Resolver) UnconfirmedMajor() []string {
	return slices.Clone(r.unconfirmed)
}

func (r *Resolver) reset() {
	r.warnings = nil
	r.wouldBeVersions = nil
	r.unconfirmed = nil
}

// bumpAfterHold is DetermineBump plus versioning.bump.stay_at_v0 (ADR-0063), recording any
// warning for Warnings, then versioning.bump.confirm_major (ADR-0078), recording a major still
// left for UnconfirmedMajor: the hold goes first, so a major it lowered needs no confirmation.
func (r *Resolver) bumpAfterHold(currentVersion string, commits []string) versioning.BumpType {
	overrides := r.cfg.Versioning.BumpOverrides()
	bump := DetermineBump(commits, overrides)
	r.warnIfChangelogWouldBeEmpty(commits, overrides)
	if r.allowMajor {
		return bump
	}
	if r.cfg.Versioning.StayAtV0() {
		held, warning, wouldBe := holdMajorAtZero(currentVersion, bump, commits, overrides)
		if warning != "" {
			r.warnings = append(r.warnings, warning)
			r.wouldBeVersions = append(r.wouldBeVersions, wouldBe)
		}
		bump = held
	}
	if bump == versioning.BumpMajor && r.cfg.Versioning.ConfirmMajor() {
		r.unconfirmed = commitsAtLevel(commits, overrides, versioning.BumpMajor)
	}
	return bump
}

// Resolve returns the next version result. --set-version never reaches it: app.NewResolver returns a
// static resolver instead, so bump: manual here means the flag is missing.
func (r *Resolver) Resolve() (versioning.Result, error) {
	r.reset()
	if r.cfg.Versioning.BumpMode() == "manual" {
		return versioning.Result{}, fmt.Errorf("manual bump mode requires --set-version flag")
	}
	if r.fixedCore != "" {
		return r.resolveFixed()
	}
	if r.preReleaseLabel != "" {
		return r.resolvePreRelease()
	}
	return r.resolveAuto()
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
		return versioning.Result{}, versioning.NothingToReleasef("no commits since %s — create at least one commit before running heraut release", currentTag)
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
	return port.TagNames(stdout)
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
	return versioning.NothingToReleasef(
		"no releasable commits since %s: %d commit(s) since then are excluded from the version bump\n%s",
		currentTag, len(commits), strings.Join(lines, "\n"),
	)
}
