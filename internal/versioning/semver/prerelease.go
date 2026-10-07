package semver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/versioning"
)

// ErrMajorEscalation is matched (errors.Is) by the error returned when a pre-release run would
// move an open series to a new major without --allow-major.
var ErrMajorEscalation = errors.New("pre-release series major escalation")

// ErrPreReleaseRegression is matched (errors.Is) by the error returned when the candidate
// pre-release would not sort above every existing tag of its core.
var ErrPreReleaseRegression = errors.New("pre-release would not sort above existing tags of its core")

// sentinelError carries a user-facing message verbatim while still matching its sentinel, so the
// sentinel's own text does not prefix the documented message.
type sentinelError struct {
	sentinel error
	msg      string
}

func (e *sentinelError) Error() string { return e.msg }

func (e *sentinelError) Is(target error) bool { return target == e.sentinel }

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

// nextCounter is one more than the highest counter of label on core among tags (1 when none).
// Only tags shaped exactly <label>.<n> count.
func nextCounter(tags []TagVersion, core Version, label string) uint64 {
	counter := uint64(1)
	for _, t := range tags {
		if Compare(coreVersion(t.Version), core) != 0 || len(t.Version.Pre) != 2 || t.Version.Pre[0] != label || !isNumericIdentifier(t.Version.Pre[1]) {
			continue
		}
		n, perr := strconv.ParseUint(t.Version.Pre[1], 10, 64)
		if perr == nil && n >= counter {
			counter = n + 1
		}
	}
	return counter
}

func (r *Resolver) resolvePreRelease() (versioning.Result, error) {
	prefix, label := r.prefix(), r.preReleaseLabel
	extract := func(tag string) (string, bool) { return strings.CutPrefix(tag, prefix) }

	stdout, _, err := r.runner.Run("git", "tag", "-l", prefix+"*", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing git tags: %w", err)
	}
	all := SortTags(parseTags(stdout), extract)

	// On a maintenance line the base is the line's own reachable release; the counter and the
	// monotonicity check stay global (ADR-0064), so all is still the full listing.
	rg := r.maintenance
	var merged []TagVersion
	last, hasFinal := Latest(all, false)
	if rg != nil {
		stdout, _, err = r.runner.Run("git", "tag", "-l", prefix+"*", "--merged", "HEAD", "--sort=-version:refname")
		if err != nil {
			return versioning.Result{}, fmt.Errorf("listing merged git tags: %w", err)
		}
		merged = SortTags(parseTags(stdout), extract)
		if last, err = inRangeBase(rg, merged); err != nil {
			return versioning.Result{}, err
		}
	}
	var (
		coreStr string
		bump    versioning.BumpType
		commits []string
		holdIdx = -1
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
		warned := len(r.warnings)
		bump = r.bumpAfterHold(last.Version.Core(), commits)
		if len(r.warnings) > warned {
			holdIdx = warned
		}
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
	if rg != nil && !rg.Contains(core) {
		return versioning.Result{}, r.outOfRangeError(rg, commits, bump, core.Core())
	}

	if err := r.checkEscalation(all, rg, last, hasFinal, core, bump, commits); err != nil {
		return versioning.Result{}, err
	}

	candidateStr := fmt.Sprintf("%s-%s.%d", core.Core(), label, nextCounter(all, core, label))
	candidate, err := Parse(candidateStr)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building pre-release %q: %w", candidateStr, err)
	}

	for _, t := range all {
		if Compare(coreVersion(t.Version), core) == 0 && Compare(candidate, t.Version) <= 0 {
			return versioning.Result{}, &sentinelError{
				sentinel: ErrPreReleaseRegression,
				msg: fmt.Sprintf("%s%s would sort below existing %s — ship %s or use a label that sorts higher",
					prefix, candidateStr, t.Tag, core.Core()),
			}
		}
	}

	if holdIdx >= 0 {
		r.nameHeldCandidates(all, label, holdIdx, core.Core(), candidateStr)
	}

	if rg == nil {
		stdout, _, err = r.runner.Run("git", "tag", "-l", prefix+"*", "--merged", "HEAD", "--sort=-version:refname")
		if err != nil {
			return versioning.Result{}, fmt.Errorf("listing merged git tags: %w", err)
		}
		merged = SortTags(parseTags(stdout), extract)
	}
	var previous TagVersion
	hasPrevious := false
	for _, t := range merged {
		if Compare(t.Version, candidate) < 0 {
			previous, hasPrevious = t, true
			break
		}
	}
	// A higher label of the same core promotes the existing series (beta.2 → rc.1) without new
	// commits; only re-cutting the same label, or opening a new series, needs them (ADR-0064).
	promotion := hasPrevious && previous.Version.IsPreRelease() &&
		Compare(coreVersion(previous.Version), core) == 0 &&
		previous.Version.Pre[0] != label
	if hasPrevious && !promotion && (!hasFinal || previous.Tag != last.Tag) {
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

// nameHeldCandidates rewrites the stay_at_v0 hold warning at idx — recorded with bare final
// versions — to the pre-release candidates it stands for: the held candidate, and the one
// --allow-major would produce (its counter computed as the resolver would on that core). Advice
// naming the final version would be wrong: --allow-major yields a pre-release too.
func (r *Resolver) nameHeldCandidates(all []TagVersion, label string, idx int, heldCore, heldCandidate string) {
	wouldBeCore := r.wouldBeVersions[idx]
	wouldBe, err := Parse(wouldBeCore)
	if err != nil {
		return
	}
	wouldBeCandidate := fmt.Sprintf("%s-%s.%d", wouldBeCore, label, nextCounter(all, coreVersion(wouldBe), label))
	headline, rest, hasRest := strings.Cut(r.warnings[idx], "\n")
	headline = strings.NewReplacer(wouldBeCore, wouldBeCandidate, heldCore, heldCandidate).Replace(headline)
	if hasRest {
		headline += "\n" + rest
	}
	r.warnings[idx] = headline
	r.wouldBeVersions[idx] = wouldBeCandidate
}

// checkEscalation compares core with the highest open pre-release series (a pre-release whose core
// is above the last final). A rise is an error for a new major without --allow-major, otherwise a
// warning. On a maintenance line (rg non-nil) only the line's own series count: a higher series
// opened on main would otherwise mask the line's escalation. The "" appended to wouldBeVersions
// keeps it parallel to warnings; it tells app.warningResolver there is no would-be tag to rewrite.
func (r *Resolver) checkEscalation(all []TagVersion, rg *Range, last TagVersion, hasFinal bool, core Version, bump versioning.BumpType, commits []string) error {
	var series TagVersion
	found := false
	for _, t := range all {
		if !t.Version.IsPreRelease() || (rg != nil && !rg.Contains(t.Version)) {
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
		subjects = commitsAtLeast(commits, r.cfg.Versioning.BumpOverrides(), bump)
	}
	if core.Major > seriesCore.Major && !r.allowMajor {
		var b strings.Builder
		fmt.Fprintf(&b, "pre-release series %s would escalate to a new major %s: breaking change(s)", series.Tag, core.Core())
		if hasFinal {
			fmt.Fprintf(&b, " since %s", last.Tag)
		}
		fmt.Fprintf(&b, " — pass --allow-major to open the %s series, or ship %s first", core.Core(), seriesCore.Core())
		writeSubjects(&b, subjects)
		return &sentinelError{sentinel: ErrMajorEscalation, msg: b.String()}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "pre-release core escalated %s → %s", seriesCore.Core(), core.Core())
	writeSubjects(&b, subjects)
	r.warnings = append(r.warnings, b.String())
	r.wouldBeVersions = append(r.wouldBeVersions, "")
	return nil
}
