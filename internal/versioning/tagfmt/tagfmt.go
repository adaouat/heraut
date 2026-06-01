package tagfmt

import (
	"fmt"
	"regexp"
	"strings"
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
		return "", fmt.Errorf("tag format template contains %s but no build ID was provided", buildToken)
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
