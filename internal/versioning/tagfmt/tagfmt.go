package tagfmt

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const (
	versionToken = "{version}"
	envToken     = "{env}"
	buildToken   = "{build}"
)

// Render substitutes {version}, {env}, and {build} tokens in template.
// build may be empty when {build} is not present in the template; if {build}
// is present but build is empty, an error is returned.
func Render(template, env, version, build string) (string, error) {
	if !strings.Contains(template, versionToken) {
		return "", fmt.Errorf("tag format template must contain %s token", versionToken)
	}
	if strings.Contains(template, buildToken) && build == "" {
		return "", fmt.Errorf("tag format template contains %s but no build ID was provided; "+
			"this format is changelog-only — run `heraut changelog --build <id>` "+
			"(heraut release / version next do not accept a build ID)", buildToken)
	}
	result := strings.ReplaceAll(template, versionToken, version)
	result = strings.ReplaceAll(result, envToken, env)
	result = strings.ReplaceAll(result, buildToken, build)
	return result, nil
}

// ParseVersion extracts the version string from a tag using the given template.
func ParseVersion(template, tag string) (string, error) {
	if !strings.Contains(template, versionToken) {
		return "", fmt.Errorf("tag format template must contain %s token", versionToken)
	}

	// Build a regex by escaping the template and replacing tokens with capture groups.
	// {version} → named capture group; {env} and {build} → non-capturing wildcards.
	regexStr := regexp.QuoteMeta(template)
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(versionToken), `(?P<version>.+)`)
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(envToken), `[^/]+`)
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(buildToken), `[^/]+`)
	regexStr = "^" + regexStr + "$"

	re, err := regexp.Compile(regexStr)
	if err != nil {
		return "", fmt.Errorf("compiling tag pattern: %w", err)
	}

	match := re.FindStringSubmatch(tag)
	if match == nil {
		return "", fmt.Errorf("tag %q does not match template %q", tag, template)
	}

	idx := re.SubexpIndex("version")
	if idx < 0 {
		return "", fmt.Errorf("internal: version capture group missing")
	}
	return match[idx], nil
}

// DeriveBuildPostprocessorPattern returns a regex suitable for a git-cliff
// postprocessor that strips the env prefix and trailing numeric build ID from
// version headings (e.g. [uat/7.4.1-158404] → [7.4.1]).
//
// Returns "" when {build} is absent, when {build} precedes {version}, or when
// no separator exists between {version} and {build}.
func DeriveBuildPostprocessorPattern(template string) string {
	if !strings.Contains(template, buildToken) || !strings.Contains(template, versionToken) {
		return ""
	}
	vIdx := strings.Index(template, versionToken)
	bIdx := strings.Index(template, buildToken)
	if bIdx <= vIdx {
		return ""
	}

	buildSep := template[vIdx+len(versionToken) : bIdx]
	if buildSep == "" {
		return ""
	}

	prefixRe := buildTagPrefixRegex(template[:vIdx])
	buildSepRe := regexp.QuoteMeta(buildSep)

	// When the build separator is "-", semver pre-release segments use the same
	// character, so disambiguate: pre-release segments start with a non-digit.
	var versionCapture string
	if buildSep == "-" {
		versionCapture = `([0-9]+\.[0-9]+\.[0-9]+(?:-[^0-9][^-\]]*)*)`
	} else {
		firstChar := regexp.QuoteMeta(string([]rune(buildSep)[0]))
		versionCapture = `([0-9]+\.[0-9]+\.[0-9]+[^` + firstChar + `\]]*)`
	}

	return `\[` + prefixRe + versionCapture + buildSepRe + `[0-9]+\]`
}

// buildTagPrefixRegex converts the portion of a tag format before {version}
// into a regex fragment, replacing {env} with an appropriate character class.
func buildTagPrefixRegex(prefix string) string {
	beforeRaw, after, found := strings.Cut(prefix, envToken)
	if !found {
		return regexp.QuoteMeta(prefix)
	}
	before := regexp.QuoteMeta(beforeRaw)

	var envClass string
	if len(after) > 0 {
		sepChar := regexp.QuoteMeta(string([]rune(after)[0]))
		envClass = `[^` + sepChar + `\]]+`
	} else {
		envClass = `[^/\]]+`
	}
	return before + envClass + regexp.QuoteMeta(after)
}

// ValidateBuildID checks that a build ID is usable as a tag component: non-empty,
// no "/" (path separator) and no whitespace (git ref constraints). CI build IDs
// like "$CI_PIPELINE_ID" and "$GITHUB_RUN_NUMBER" satisfy this.
func ValidateBuildID(build string) error {
	if build == "" {
		return fmt.Errorf("build ID must not be empty")
	}
	if strings.ContainsRune(build, '/') {
		return fmt.Errorf("build ID %q must not contain '/'", build)
	}
	if strings.ContainsFunc(build, unicode.IsSpace) {
		return fmt.Errorf("build ID %q must not contain whitespace", build)
	}
	return nil
}

// DeriveTagPattern returns an anchored regex (for git-cliff --tag-pattern) that matches
// only the given environment's tags. {env} becomes the literal env, {version} and {build}
// become wildcards. Returns "" when the template has no {env} token or env is empty —
// there is nothing to scope in those cases.
func DeriveTagPattern(template, env string) string {
	if env == "" || !strings.Contains(template, envToken) {
		return ""
	}
	regexStr := regexp.QuoteMeta(template)
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(versionToken), `.+`)
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(buildToken), `.+`)
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(envToken), regexp.QuoteMeta(env))
	return "^" + regexStr + "$"
}

// GlobPattern returns a git tag glob pattern for listing tags under the given env.
func GlobPattern(template, env string) (string, error) {
	if !strings.Contains(template, versionToken) {
		return "", fmt.Errorf("tag format template must contain %s token", versionToken)
	}
	result := strings.ReplaceAll(template, envToken, env)
	result = strings.ReplaceAll(result, versionToken, "*")
	result = strings.ReplaceAll(result, buildToken, "*")
	return result, nil
}
