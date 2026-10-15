package perenv

import (
	"slices"

	"github.com/adaouat/heraut/internal/versioning/calver"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

const semverPerEnv = "semver-per-env"

// releaseTags returns env's release tags — never pre-releases — newest first, each paired with the
// bare version handed to the VersionCalculator. semver-per-env orders them by SemVer §11 in Go and
// passes each version's MAJOR.MINOR.PATCH core (ADR-0064). calver-per-env keeps git's
// version:refname order and the lenient dotted-integer check, because CalVer's zero-padded
// components (2026.05.3) are not valid SemVer; it orders them by calendar value under format
// instead, so git's own order never decides which tag is the latest.
func releaseTags(strategy, tf, format string, rawTags []string) (tags, versions []string) {
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
	return sortNewestFirst(format, tags, versions)
}

// sortNewestFirst reorders tags and versions together, newest calendar value first, keeping git's
// order for ties. An unusable format leaves the order as given.
func sortNewestFirst(format string, tags, versions []string) ([]string, []string) {
	cmp, err := calver.CompareByFormat(format)
	if err != nil || len(tags) < 2 {
		return tags, versions
	}
	idx := make([]int, len(tags))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int { return cmp(versions[b], versions[a]) })
	sortedTags, sortedVersions := make([]string, len(tags)), make([]string, len(tags))
	for i, j := range idx {
		sortedTags[i], sortedVersions[i] = tags[j], versions[j]
	}
	return sortedTags, sortedVersions
}

// latestTag returns the destination env's latest tag and its bare version. ok is false when none
// of the tags holds a real version, in which case E002 cannot apply and no tag is returned: the
// tag ends up in Result.CurrentTag and from there in hook variables, so a ref that merely matches
// the tag glob (any ref name is allowed) must never be reported. semver-per-env picks the highest
// §11 tag of any kind; calver-per-env keeps git's order and takes the first tag whose version is
// a dotted-integer one, the highest by calendar value under format.
func latestTag(strategy, tf, format string, rawTags []string) (tag, version string, ok bool) {
	if strategy == semverPerEnv {
		if tv, found := semver.Latest(semver.SortTags(rawTags, versionExtractor(tf)), true); found {
			return tv.Tag, tv.Version.String(), true
		}
		return "", "", false
	}
	tags, versions := releaseTags(strategy, tf, format, rawTags)
	if len(tags) == 0 {
		return "", "", false
	}
	return tags[0], versions[0], true
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
	return calver.CompareVersions(a, b)
}

func versionExtractor(tf string) func(string) (string, bool) {
	return func(tag string) (string, bool) {
		v, err := tagfmt.ParseVersion(tf, tag)
		return v, err == nil
	}
}
