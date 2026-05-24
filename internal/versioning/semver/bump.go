package semver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/versioning"
)

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

func isBreaking(commit string) bool {
	// feat! or fix! (type with bang before colon)
	subject := firstLine(commit)
	if idx := strings.Index(subject, "!:"); idx > 0 {
		return true
	}
	// BREAKING CHANGE footer anywhere in the full commit message
	if strings.Contains(commit, "BREAKING CHANGE:") {
		return true
	}
	return false
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
