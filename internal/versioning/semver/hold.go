package semver

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
)

const maxListedCommits = 5

// holdMajorAtZero lowers a major bump to minor when currentVersion's major component is 0
// (ADR-0063), returning the possibly-lowered bump, the warning to show, and the bare "would-be"
// major version the warning names ("" for both when nothing was held back). The bare wouldBe
// version lets a caller with the real rendered tag (app.warningResolver) substitute it into the
// warning's headline — this function never sees tag_format for per-env strategies.
func holdMajorAtZero(currentVersion string, bump versioning.BumpType, commits []string, overrides []config.BumpRule) (heldBump versioning.BumpType, warning string, wouldBeVersion string) {
	if bump != versioning.BumpMajor {
		return bump, "", ""
	}
	major, _, err := MajorMinor(currentVersion)
	if err != nil || major != 0 {
		return bump, "", ""
	}
	wouldBe, err := BumpVersion(currentVersion, versioning.BumpMajor)
	if err != nil {
		return bump, "", ""
	}
	held, err := BumpVersion(currentVersion, versioning.BumpMinor)
	if err != nil {
		return bump, "", ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "major bump held back by versioning.bump.stay_at_v0: %s → %s (pass --allow-major on this run to get %s instead)", wouldBe, held, wouldBe)
	writeSubjects(&b, commitsAtLevel(commits, overrides, versioning.BumpMajor))
	return versioning.BumpMinor, b.String(), wouldBe
}

// commitsAtLevel returns the subject line of every commit whose own bump level is level, after
// overrides — the commits that forced a bump to that level.
func commitsAtLevel(commits []string, overrides []config.BumpRule, level versioning.BumpType) []string {
	rules := compileBumpRules(overrides)
	var subjects []string
	for _, c := range commits {
		if resolveBumpLevel(c, rules) == level {
			subjects = append(subjects, firstLine(c))
		}
	}
	return subjects
}

// commitsAtLeast returns the subject line of every commit whose own bump level is at or above
// level. Under a stay_at_v0 hold the applied bump is lower than the level that forced it, so an
// exact-level match would miss the responsible commit.
func commitsAtLeast(commits []string, overrides []config.BumpRule, level versioning.BumpType) []string {
	rules := compileBumpRules(overrides)
	var subjects []string
	for _, c := range commits {
		if resolveBumpLevel(c, rules) >= level {
			subjects = append(subjects, firstLine(c))
		}
	}
	return subjects
}

// writeSubjects appends up to maxListedCommits subjects as "\n  - <subject>" lines, then a
// "\n  … and N more" summary line when some were left out.
func writeSubjects(b *strings.Builder, subjects []string) {
	for i, s := range subjects {
		if i == maxListedCommits {
			fmt.Fprintf(b, "\n  … and %d more", len(subjects)-maxListedCommits)
			return
		}
		fmt.Fprintf(b, "\n  - %s", s)
	}
}
