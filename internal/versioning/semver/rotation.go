package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// MajorMinor extracts the MAJOR and MINOR components from a bare SemVer version string, for
// substitution into a rotating changelog output pattern's {MAJOR}/{MINOR} tokens. version must
// have at least MAJOR.MINOR.PATCH shape; PATCH may carry a pre-release/build-metadata suffix
// (e.g. "1.4.2-rc.1", "1.4.2+build.5") since heraut's --version override is strategy-agnostic
// about shape (see tagfmt.ValidateVersionOverride) — MAJOR and MINOR are unaffected either way.
func MajorMinor(version string) (major, minor int, err error) {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 3 {
		return 0, 0, fmt.Errorf("invalid semver %q: expected MAJOR.MINOR.PATCH", version)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major in %q: %w", version, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor in %q: %w", version, err)
	}
	return major, minor, nil
}

// RotationPattern returns a regular expression matching any bare version string in the same
// rotation bucket as major/minor, given the tokens a rotating changelog output/tag pattern groups
// by ("MAJOR" alone, or "MAJOR" and "MINOR" together — order doesn't matter). "MINOR" alone is
// rejected: MAJOR is always the leading, required dimension, mirroring calver.BucketPattern's
// prefix-order rule for the CalVer strategy family. The pattern is anchored at the start (^) only —
// callers compose their own tag_prefix quoting.
func RotationPattern(tokens []string, major, minor int) (string, error) {
	hasMajor, hasMinor := false, false
	for _, t := range tokens {
		switch t {
		case "MAJOR":
			hasMajor = true
		case "MINOR":
			hasMinor = true
		default:
			return "", fmt.Errorf("rotation token %q is not a valid semver token", t)
		}
	}
	if !hasMajor {
		return "", fmt.Errorf("rotation token \"MINOR\" requires \"MAJOR\" to also be present")
	}

	if hasMinor {
		return fmt.Sprintf(`^%d\.%d\.`, major, minor), nil
	}
	return fmt.Sprintf(`^%d\.`, major), nil
}
