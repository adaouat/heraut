package semver

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ErrInvalidVersion is returned (wrapped) by Parse for any string that is not a SemVer 2.0.0
// version.
var ErrInvalidVersion = errors.New("invalid SemVer version")

// Version is a parsed SemVer 2.0.0 version (https://semver.org/spec/v2.0.0.html).
type Version struct {
	Major, Minor, Patch uint64
	// Pre holds the pre-release identifiers (§9); empty for a release.
	Pre []string
	// Build holds the build-metadata identifiers (§10); Compare ignores it.
	Build []string
}

// Parse parses s strictly per the SemVer 2.0.0 grammar. s must not carry a tag prefix.
func Parse(s string) (Version, error) {
	rest, build, hasBuild := strings.Cut(s, "+")
	core, pre, hasPre := strings.Cut(rest, "-")

	var v Version
	nums := strings.Split(core, ".")
	if len(nums) != 3 {
		return Version{}, fmt.Errorf("%w %q: expected MAJOR.MINOR.PATCH", ErrInvalidVersion, s)
	}
	fields := []*uint64{&v.Major, &v.Minor, &v.Patch}
	for i, n := range nums {
		if !isNumericIdentifier(n) {
			return Version{}, fmt.Errorf("%w %q: %q is not a number without leading zeros", ErrInvalidVersion, s, n)
		}
		val, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("%w %q: %w", ErrInvalidVersion, s, err)
		}
		*fields[i] = val
	}
	if hasPre {
		ids, err := splitIdentifiers(pre, true)
		if err != nil {
			return Version{}, fmt.Errorf("%w %q: pre-release: %w", ErrInvalidVersion, s, err)
		}
		v.Pre = ids
	}
	if hasBuild {
		ids, err := splitIdentifiers(build, false)
		if err != nil {
			return Version{}, fmt.Errorf("%w %q: build metadata: %w", ErrInvalidVersion, s, err)
		}
		v.Build = ids
	}
	return v, nil
}

// String renders v in canonical SemVer form, including pre-release and build metadata.
func (v Version) String() string {
	s := v.Core()
	if len(v.Pre) > 0 {
		s += "-" + strings.Join(v.Pre, ".")
	}
	if len(v.Build) > 0 {
		s += "+" + strings.Join(v.Build, ".")
	}
	return s
}

// Core returns MAJOR.MINOR.PATCH without pre-release or build metadata.
func (v Version) Core() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// IsPreRelease reports whether v carries pre-release identifiers. Build metadata alone does not
// make a pre-release: 1.4.0+5 is the 1.4.0 release.
func (v Version) IsPreRelease() bool { return len(v.Pre) > 0 }

// Compare orders a and b by SemVer §11 precedence: -1 if a < b, 0 if equal, +1 if a > b. Build
// metadata is ignored, so versions differing only in it compare equal.
func Compare(a, b Version) int {
	if c := cmp.Compare(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Patch, b.Patch); c != 0 {
		return c
	}
	switch {
	case len(a.Pre) == 0 && len(b.Pre) == 0:
		return 0
	case len(a.Pre) == 0:
		return 1
	case len(b.Pre) == 0:
		return -1
	}
	for i := 0; i < len(a.Pre) && i < len(b.Pre); i++ {
		if c := compareIdentifier(a.Pre[i], b.Pre[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a.Pre), len(b.Pre))
}

// compareIdentifier compares two pre-release identifiers. Numeric identifiers have no leading
// zeros (Parse guarantees it), so a longer digit string is the larger number — comparing by
// length then lexically avoids overflowing on arbitrarily long identifiers.
func compareIdentifier(a, b string) int {
	aNum, bNum := allDigits(a), allDigits(b)
	switch {
	case aNum && bNum:
		if c := cmp.Compare(len(a), len(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	case aNum:
		return -1
	case bNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// TagVersion pairs a git tag with the SemVer version parsed from it.
type TagVersion struct {
	Tag     string
	Version Version
}

// SortTags parses every tag through extract — which returns the tag's bare version string and
// whether the tag belongs to this scheme at all — and returns the valid SemVer ones, highest §11
// precedence first. Tags that don't extract or don't parse are dropped. The sort is stable, so
// tags of equal precedence (differing only in build metadata) keep their input order.
func SortTags(tags []string, extract func(tag string) (string, bool)) []TagVersion {
	out := make([]TagVersion, 0, len(tags))
	for _, tag := range tags {
		bare, ok := extract(tag)
		if !ok {
			continue
		}
		v, err := Parse(bare)
		if err != nil {
			continue
		}
		out = append(out, TagVersion{Tag: tag, Version: v})
	}
	slices.SortStableFunc(out, func(a, b TagVersion) int { return Compare(b.Version, a.Version) })
	return out
}

// Latest returns the first entry of sorted (as returned by SortTags) that is a release — or, with
// includePreRelease, the first entry of any kind. ok is false when none qualifies.
func Latest(sorted []TagVersion, includePreRelease bool) (TagVersion, bool) {
	for _, tv := range sorted {
		if includePreRelease || !tv.Version.IsPreRelease() {
			return tv, true
		}
	}
	return TagVersion{}, false
}

func isNumericIdentifier(s string) bool {
	return allDigits(s) && (len(s) == 1 || s[0] != '0')
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// splitIdentifiers splits a dot-separated pre-release or build-metadata string into identifiers,
// enforcing [0-9A-Za-z-]+ each. Numeric pre-release identifiers must not have leading zeros
// (§9); build metadata allows them (§10).
func splitIdentifiers(s string, rejectLeadingZeros bool) ([]string, error) {
	ids := strings.Split(s, ".")
	for _, id := range ids {
		if id == "" {
			return nil, errors.New("empty identifier")
		}
		for _, r := range id {
			if (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && r != '-' {
				return nil, fmt.Errorf("identifier %q contains %q (allowed: [0-9A-Za-z-])", id, r)
			}
		}
		if rejectLeadingZeros && allDigits(id) && !isNumericIdentifier(id) {
			return nil, fmt.Errorf("numeric identifier %q has a leading zero", id)
		}
	}
	return ids, nil
}
