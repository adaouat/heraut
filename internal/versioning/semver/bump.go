package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/versioning"
)

// breakingPrefixPattern matches a conventional-commit subject whose type (and
// optional scope) is marked breaking with "!" immediately before the colon,
// e.g. "feat!:" or "fix(api)!:". A bare "!:" elsewhere in the subject — such
// as inside the description — does not count.
var breakingPrefixPattern = regexp.MustCompile(`^\w+(\([^)]*\))?!:`)

// DetermineBump scans conventional commit subjects and returns the highest applicable bump.
func DetermineBump(commits []string) versioning.BumpType {
	bump := versioning.BumpPatch // fallback
	for _, c := range commits {
		if isBreaking(c) {
			return versioning.BumpMajor
		}
		if isFeat(c) && bump < versioning.BumpMinor {
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

// isBareVersion reports whether s is a bare MAJOR.MINOR.PATCH version with no
// pre-release or build metadata (e.g. "1.2.3", not "1.2.3-rc.1"). Used by the
// resolver to skip git tags that don't conform when locating the most recent
// release tag.
func isBareVersion(s string) bool {
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

func isBreaking(commit string) bool {
	// type! or type(scope)! immediately before the colon
	if breakingPrefixPattern.MatchString(firstLine(commit)) {
		return true
	}
	// BREAKING CHANGE / BREAKING-CHANGE footer anywhere in the full commit
	// message — Conventional Commits 1.0.0 treats the hyphenated form as a
	// synonym of the spaced form.
	return strings.Contains(commit, "BREAKING CHANGE:") || strings.Contains(commit, "BREAKING-CHANGE:")
}

func isFeat(commit string) bool {
	subject := firstLine(commit)
	// feat(...): or feat:
	return strings.HasPrefix(subject, "feat(") || strings.HasPrefix(subject, "feat:")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
