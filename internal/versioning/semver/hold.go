package semver

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
)

const maxHeldBackCommits = 5

// holdMajorAtZero lowers a major bump to minor when currentVersion's major component is 0
// (ADR-0063), returning the possibly-lowered bump and the warning to show; the warning is empty
// when nothing was held back. Whether stay_at_v0 is enabled and whether --allow-major was passed is
// the caller's decision — this only answers "would this be a 0.x → 1.0.0 jump, and what does the
// warning say".
func holdMajorAtZero(currentVersion string, bump versioning.BumpType, commits []string, overrides []config.BumpRule) (versioning.BumpType, string) {
	if bump != versioning.BumpMajor {
		return bump, ""
	}
	major, _, err := MajorMinor(currentVersion)
	if err != nil || major != 0 {
		return bump, ""
	}
	wouldBe, err := BumpVersion(currentVersion, versioning.BumpMajor)
	if err != nil {
		return bump, ""
	}
	held, err := BumpVersion(currentVersion, versioning.BumpMinor)
	if err != nil {
		return bump, ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "major bump held back by versioning.bump.stay_at_v0: %s → %s (pass --allow-major to release %s)", wouldBe, held, wouldBe)
	subjects := majorCommits(commits, overrides)
	for i, s := range subjects {
		if i == maxHeldBackCommits {
			fmt.Fprintf(&b, "\n  … and %d more", len(subjects)-maxHeldBackCommits)
			break
		}
		fmt.Fprintf(&b, "\n  - %s", s)
	}
	return versioning.BumpMinor, b.String()
}

// majorCommits returns the subject line of every commit whose own bump level is major, after
// overrides — the commits that forced the major bump.
func majorCommits(commits []string, overrides []config.BumpRule) []string {
	rules := compileBumpRules(overrides)
	var subjects []string
	for _, c := range commits {
		if resolveBumpLevel(c, rules) == versioning.BumpMajor {
			subjects = append(subjects, firstLine(c))
		}
	}
	return subjects
}
