package perenv

import (
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

const semverPerEnv = "semver-per-env"

// releaseTags returns env's release tags — never pre-releases — newest first, each paired with the
// bare version handed to the VersionCalculator. semver-per-env orders them by SemVer §11 in Go and
// passes each version's MAJOR.MINOR.PATCH core (ADR-0064). calver-per-env keeps git's
// version:refname order and the lenient dotted-integer check, because CalVer's zero-padded
// components (2026.05.3) are not valid SemVer.
func releaseTags(strategy, tf string, rawTags []string) (tags, versions []string) {
	if strategy == semverPerEnv {
		for _, tv := range semver.SortTags(rawTags, versionExtractor(tf)) {
			if tv.Version.IsPreRelease() {
				continue
			}
			tags = append(tags, tv.Tag)
			versions = append(versions, tv.Version.Core())
		}
		return tags, versions
	}
	for _, tag := range rawTags {
		bare, err := tagfmt.ParseVersion(tf, tag)
		if err != nil || !semver.IsBareVersion(bare) {
			continue
		}
		tags = append(tags, tag)
		versions = append(versions, bare)
	}
	return tags, versions
}

// latestTag returns the destination env's latest tag and its bare version. ok is false when no
// version could be parsed from it, in which case E002 cannot apply. semver-per-env picks the
// highest §11 tag of any kind; calver-per-env keeps git's first line.
func latestTag(strategy, tf string, rawTags []string) (tag, version string, ok bool) {
	if strategy == semverPerEnv {
		if tv, found := semver.Latest(semver.SortTags(rawTags, versionExtractor(tf)), true); found {
			return tv.Tag, tv.Version.String(), true
		}
	}
	if len(rawTags) == 0 {
		return "", "", false
	}
	v, err := tagfmt.ParseVersion(tf, rawTags[0])
	return rawTags[0], v, err == nil && strategy != semverPerEnv
}

// compareVersions orders two bare versions: SemVer §11 for semver-per-env when both parse, else
// the dotted-integer comparison calver-per-env has always used.
func compareVersions(strategy, a, b string) int {
	if strategy == semverPerEnv {
		va, errA := semver.Parse(a)
		vb, errB := semver.Parse(b)
		if errA == nil && errB == nil {
			return semver.Compare(va, vb)
		}
	}
	return compareVersionStrings(a, b)
}

func versionExtractor(tf string) func(string) (string, bool) {
	return func(tag string) (string, bool) {
		v, err := tagfmt.ParseVersion(tf, tag)
		return v, err == nil
	}
}
