package versioning

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
