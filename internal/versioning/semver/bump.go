package semver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/adaouat/heraut/internal/versioning"
)

// DetermineBump scans conventional commit subjects and returns the highest applicable bump.
func DetermineBump(commits []string) versioning.BumpType {
	bump := versioning.BumpPatch // fallback
	for _, c := range commits {
		parsed, err := conventionalcommit.Parse(c)
		if err != nil {
			continue // not a conventional commit — ignore for bump purposes, same as before
		}
		if parsed.Breaking {
			return versioning.BumpMajor
		}
		if parsed.Type == "feat" && bump < versioning.BumpMinor {
			bump = versioning.BumpMinor
		}
	}
	return bump
}

// BumpVersion increments the appropriate SemVer component.
// current must be a bare version string without prefix (e.g. "1.2.3").
func BumpVersion(current string, bump versioning.BumpType) (string, error) {
	parts := strings.SplitN(current, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver %q: expected MAJOR.MINOR.PATCH", current)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major in %q: %w", current, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor in %q: %w", current, err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch in %q: %w", current, err)
	}

	switch bump {
	case versioning.BumpMajor:
		major++
		minor = 0
		patch = 0
	case versioning.BumpMinor:
		minor++
		patch = 0
	default:
		patch++
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

// IsBareVersion reports whether s is a bare MAJOR.MINOR.PATCH version with no
// pre-release or build metadata (e.g. "1.2.3", not "1.2.3-rc.1"). Used by the
// resolver, and by internal/versioning/perenv, to skip git tags that don't
// conform when locating the most recent release tag.
func IsBareVersion(s string) bool {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}
