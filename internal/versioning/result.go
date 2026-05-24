package versioning

// BumpType represents the SemVer component to increment.
type BumpType int

const (
	BumpNone  BumpType = iota
	BumpPatch BumpType = iota
	BumpMinor BumpType = iota
	BumpMajor BumpType = iota
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
}
