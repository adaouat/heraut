package semver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/versioning"
)

// ErrMajorEscalation is returned (wrapped) when a pre-release run would move an open series to a
// new major without --allow-major.
var ErrMajorEscalation = errors.New("pre-release series major escalation")

// ErrPreReleaseRegression is returned (wrapped) when the candidate pre-release would not sort
// above every existing tag of its core.
var ErrPreReleaseRegression = errors.New("pre-release would not sort above existing tags of its core")

// ValidatePreReleaseLabel checks that label is a single SemVer pre-release identifier that is not
// purely numeric: [0-9A-Za-z-]+, no dots. heraut appends ".N", so a numeric label would collide
// with the counter's own identifier type.
func ValidatePreReleaseLabel(label string) error {
	if label == "" {
		return errors.New("pre-release label must not be empty")
	}
	if allDigits(label) {
		return fmt.Errorf("pre-release label %q must not be purely numeric", label)
	}
	for _, r := range label {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && r != '-' {
			return fmt.Errorf("pre-release label %q contains %q (allowed: [0-9A-Za-z-], no dots)", label, r)
		}
	}
	return nil
}

func coreVersion(v Version) Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
}

func (r *Resolver) resolvePreRelease() (versioning.Result, error) {
	prefix, label := r.prefix(), r.preReleaseLabel
	extract := func(tag string) (string, bool) { return strings.CutPrefix(tag, prefix) }

	stdout, _, err := r.runner.Run("git", "tag", "-l", prefix+"*", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing git tags: %w", err)
	}
	all := SortTags(parseTags(stdout), extract)

	last, hasFinal := Latest(all, false)
	var (
		coreStr string
		bump    versioning.BumpType
		commits []string
	)
	if hasFinal {
		stdout, _, err = r.runner.Run("git", "log", last.Tag+"..HEAD", "--format=%B%x00")
		if err != nil {
			return versioning.Result{}, fmt.Errorf("reading git log: %w", err)
		}
		commits = parseCommits(stdout)
		if len(commits) == 0 {
			return versioning.Result{}, fmt.Errorf("no commits since %s — create at least one commit before running heraut release", last.Tag)
		}
		bump = r.bumpAfterHold(last.Version.Core(), commits)
		if bump == versioning.BumpNone {
			return versioning.Result{}, noReleasableCommitsError(last.Tag, commits)
		}
		coreStr, err = BumpVersion(last.Version.Core(), bump)
		if err != nil {
			return versioning.Result{}, fmt.Errorf("bumping version: %w", err)
		}
	} else {
		coreStr = r.initialVersion()
	}
	core, err := Parse(coreStr)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("parsing core %q: %w", coreStr, err)
	}
	core = coreVersion(core)

	if err := r.checkEscalation(all, last, hasFinal, core, bump, commits); err != nil {
		return versioning.Result{}, err
	}

	counter := uint64(1)
	for _, t := range all {
		if Compare(coreVersion(t.Version), core) != 0 || len(t.Version.Pre) != 2 || t.Version.Pre[0] != label || !isNumericIdentifier(t.Version.Pre[1]) {
			continue
		}
		n, perr := strconv.ParseUint(t.Version.Pre[1], 10, 64)
		if perr == nil && n >= counter {
			counter = n + 1
		}
	}
	candidateStr := fmt.Sprintf("%s-%s.%d", core.Core(), label, counter)
	candidate, err := Parse(candidateStr)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building pre-release %q: %w", candidateStr, err)
	}

	for _, t := range all {
		if Compare(coreVersion(t.Version), core) == 0 && Compare(candidate, t.Version) <= 0 {
			return versioning.Result{}, fmt.Errorf(
				"%w: %s%s would sort below existing %s — ship %s or use a label that sorts higher",
				ErrPreReleaseRegression, prefix, candidateStr, t.Tag, core.Core(),
			)
		}
	}

	stdout, _, err = r.runner.Run("git", "tag", "-l", prefix+"*", "--merged", "HEAD", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing merged git tags: %w", err)
	}
	var previous TagVersion
	hasPrevious := false
	for _, t := range SortTags(parseTags(stdout), extract) {
		if Compare(t.Version, candidate) < 0 {
			previous, hasPrevious = t, true
			break
		}
	}
	if hasPrevious && (!hasFinal || previous.Tag != last.Tag) {
		stdout, _, err = r.runner.Run("git", "log", previous.Tag+"..HEAD", "--format=%B%x00")
		if err != nil {
			return versioning.Result{}, fmt.Errorf("reading git log: %w", err)
		}
		if len(parseCommits(stdout)) == 0 {
			return versioning.Result{}, fmt.Errorf("no commits since %s — create at least one commit before running heraut release", previous.Tag)
		}
	}

	currentTag := last.Tag
	if hasPrevious {
		currentTag = previous.Tag
	}
	return versioning.Result{
		Version:    candidateStr,
		Tag:        prefix + candidateStr,
		CurrentTag: currentTag,
		Bump:       bump,
	}, nil
}

// checkEscalation compares core with the highest open pre-release series (a pre-release whose core
// is above the last final). A rise is an error for a new major without --allow-major, otherwise a
// warning. The "" appended to wouldBeVersions keeps it parallel to warnings; it tells
// app.warningResolver there is no would-be tag to rewrite.
func (r *Resolver) checkEscalation(all []TagVersion, last TagVersion, hasFinal bool, core Version, bump versioning.BumpType, commits []string) error {
	var series TagVersion
	found := false
	for _, t := range all {
		if !t.Version.IsPreRelease() {
			continue
		}
		if hasFinal && Compare(coreVersion(t.Version), coreVersion(last.Version)) <= 0 {
			continue
		}
		series, found = t, true
		break
	}
	if !found {
		return nil
	}
	seriesCore := coreVersion(series.Version)
	if Compare(core, seriesCore) <= 0 {
		return nil
	}

	var subjects []string
	if hasFinal {
		subjects = commitsAtLevel(commits, r.cfg.Versioning.BumpOverrides(), bump)
	}
	if core.Major > seriesCore.Major && !r.allowMajor {
		var b strings.Builder
		fmt.Fprintf(&b, "pre-release series %s would escalate to a new major %s: breaking change(s)", series.Tag, core.Core())
		if hasFinal {
			fmt.Fprintf(&b, " since %s", last.Tag)
		}
		fmt.Fprintf(&b, " — pass --allow-major to open the %s series, or ship %s first", core.Core(), seriesCore.Core())
		writeSubjects(&b, subjects)
		return fmt.Errorf("%w: %s", ErrMajorEscalation, b.String())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "pre-release core escalated %s → %s", seriesCore.Core(), core.Core())
	writeSubjects(&b, subjects)
	r.warnings = append(r.warnings, b.String())
	r.wouldBeVersions = append(r.wouldBeVersions, "")
	return nil
}
