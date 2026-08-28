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
