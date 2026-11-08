package versioning

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNothingToRelease is matched (errors.Is) by every "nothing to release" condition: no commits since
// the last release, no releasable commits, no release tag yet (ADR-0082). It has its own exit code, so
// a CI job that releases on every push can tell it from a failure.
var ErrNothingToRelease = errors.New("nothing to release")

// NothingToRelease returns an error with message msg that matches ErrNothingToRelease. Its own
// identity is kept, so a package can hold one as a sentinel too.
func NothingToRelease(msg string) error { return &nothingToRelease{msg: msg} }

// NothingToReleasef is NothingToRelease with a format.
func NothingToReleasef(format string, args ...any) error {
	return &nothingToRelease{msg: fmt.Sprintf(format, args...)}
}

type nothingToRelease struct{ msg string }

func (e *nothingToRelease) Error() string        { return e.msg }
func (e *nothingToRelease) Is(target error) bool { return target == ErrNothingToRelease }

// ErrMajorNotConfirmed is matched (errors.Is) by MajorConfirmationError's error: an automatic major
// bump that versioning.bump.confirm_major asks --allow-major for (ADR-0078).
var ErrMajorNotConfirmed = errors.New("major bump not confirmed")

// maxListedCommits caps the commits a major-confirmation message lists.
const maxListedCommits = 5

// BumpType represents the SemVer component to increment.
type BumpType int

const (
	BumpNone BumpType = iota
	BumpPatch
	BumpMinor
	BumpMajor
)

// Result is the output of a version resolution.
type Result struct {
	// Version is the next version string (without prefix).
	Version string
	// Tag is the full tag that will be created (prefix + version, or tagfmt rendering).
	Tag string
	// CurrentTag is the most recent existing tag, empty if none.
	CurrentTag string
	// Bump is how the version was bumped (semver strategies only).
	Bump BumpType
	// Warnings are user-facing notices produced while resolving — e.g. a major bump held back by
	// versioning.bump.stay_at_v0. One entry per warning; an entry may span several lines, headline
	// first. Resolvers never print them; callers do.
	Warnings []string
	// UnconfirmedMajor lists the subject of every commit that forced an automatic major bump which
	// versioning.bump.confirm_major asks --allow-major for (ADR-0078); nil when there is nothing to
	// confirm. Resolvers never refuse it; the commands that tag do.
	UnconfirmedMajor []string
}

// String names the bump ("none", "patch", "minor", "major").
func (b BumpType) String() string {
	switch b {
	case BumpPatch:
		return "patch"
	case BumpMinor:
		return "minor"
	case BumpMajor:
		return "major"
	}
	return "none"
}

// MajorConfirmationError is the refusal heraut release and heraut changelog return for a major still
// to confirm (ADR-0078), naming the tag and the commits that forced it; nil when there is nothing
// to confirm.
func (r Result) MajorConfirmationError() error {
	if len(r.UnconfirmedMajor) == 0 {
		return nil
	}
	msg := fmt.Sprintf("%s is a major release, and versioning.bump.confirm_major asks to confirm it: pass --allow-major on this run to release it", r.Tag)
	return &majorNotConfirmedError{msg: msg + listCommits(r.UnconfirmedMajor)}
}

// MajorConfirmationWarning is what heraut version next prints instead of refusing; "" when there
// is nothing to confirm.
func (r Result) MajorConfirmationWarning() string {
	if len(r.UnconfirmedMajor) == 0 {
		return ""
	}
	return fmt.Sprintf("%s is a major release: heraut release and heraut changelog refuse it without --allow-major (versioning.bump.confirm_major)", r.Tag) +
		listCommits(r.UnconfirmedMajor)
}

type majorNotConfirmedError struct{ msg string }

func (e *majorNotConfirmedError) Error() string { return e.msg }
func (e *majorNotConfirmedError) Unwrap() error { return ErrMajorNotConfirmed }

// listCommits renders up to maxListedCommits subjects as "\n  - <subject>" lines, then a
// "\n  … and N more" line for the others.
func listCommits(subjects []string) string {
	var b strings.Builder
	for i, s := range subjects {
		if i == maxListedCommits {
			fmt.Fprintf(&b, "\n  … and %d more", len(subjects)-maxListedCommits)
			break
		}
		fmt.Fprintf(&b, "\n  - %s", s)
	}
	return b.String()
}
