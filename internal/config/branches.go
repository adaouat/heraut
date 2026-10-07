package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// BranchRule is one versioning.branches entry (ADR-0065).
type BranchRule struct {
	Name  string `yaml:"name"`
	Range string `yaml:"range,omitempty"`
}

// IsGlob reports whether Name is a path.Match pattern rather than an exact branch name.
func (b BranchRule) IsGlob() bool {
	return strings.ContainsAny(b.Name, "*?[")
}

// BranchRange is a maintenance line. Minor == nil means N.x (>=N.0.0 <N+1.0.0, patch and
// minor bumps); Minor set means N.M.x (>=N.M.0 <N.M+1.0, patch bumps only).
type BranchRange struct {
	Major uint64
	Minor *uint64
}

// String renders the range in its config spelling: "1.x" or "1.3.x".
func (r BranchRange) String() string {
	if r.Minor == nil {
		return fmt.Sprintf("%d.x", r.Major)
	}
	return fmt.Sprintf("%d.%d.x", r.Major, *r.Minor)
}

// The numeric components forbid leading zeros so a range spelling is canonical and
// duplicate detection can compare rendered strings.
var (
	branchRangePattern  = regexp.MustCompile(`^(0|[1-9]\d*)\.(?:(0|[1-9]\d*)\.)?x$`)
	branchDerivePattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(?:(0|[1-9]\d*)(?:\.x)?|x)$`)
)

// ParseBranchRange parses "N.x" or "N.N.x".
func ParseBranchRange(s string) (BranchRange, error) {
	if r, ok := rangeFromMatch(branchRangePattern.FindStringSubmatch(s)); ok {
		return r, nil
	}
	return BranchRange{}, fmt.Errorf("%q is not a valid range (want N.x or N.N.x)", s)
}

// DeriveBranchRange derives a range from a branch name's last "/" segment: N.x, N.N.x or N.N,
// with one optional leading v/V. N.N.N is deliberately not derivable (it names a release, not a
// line).
func DeriveBranchRange(branch string) (BranchRange, bool) {
	seg := branch[strings.LastIndex(branch, "/")+1:]
	if len(seg) > 0 && (seg[0] == 'v' || seg[0] == 'V') {
		seg = seg[1:]
	}
	return rangeFromMatch(branchDerivePattern.FindStringSubmatch(seg))
}

func rangeFromMatch(m []string) (BranchRange, bool) {
	if m == nil {
		return BranchRange{}, false
	}
	major, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return BranchRange{}, false
	}
	if m[2] == "" {
		return BranchRange{Major: major}, true
	}
	minor, err := strconv.ParseUint(m[2], 10, 64)
	if err != nil {
		return BranchRange{}, false
	}
	return BranchRange{Major: major, Minor: &minor}, true
}
