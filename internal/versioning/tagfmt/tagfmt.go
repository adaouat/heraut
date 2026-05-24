package tagfmt

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	versionToken = "{version}"
	envToken     = "{env}"
)

// Render substitutes {version} and {env} tokens in template.
func Render(template, env, version string) (string, error) {
	if !strings.Contains(template, versionToken) {
		return "", fmt.Errorf("tag format template must contain %s token", versionToken)
	}
	result := strings.ReplaceAll(template, versionToken, version)
	result = strings.ReplaceAll(result, envToken, env)
	return result, nil
}

// ParseVersion extracts the version string from a tag using the given template.
func ParseVersion(template, tag string) (string, error) {
	if !strings.Contains(template, versionToken) {
		return "", fmt.Errorf("tag format template must contain %s token", versionToken)
	}

	// Build a regex by escaping the template and replacing tokens with capture groups.
	// {version} → named capture group; {env} → non-capturing wildcard.
	regexStr := regexp.QuoteMeta(template)
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(versionToken), `(?P<version>.+)`)
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(envToken), `[^/]+`)
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
	return result, nil
}
