package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/adaouat/heraut/internal/versioning"
)

// bumpRule is a config.BumpRule with its regex precompiled once, up front, rather than per
// commit.
type bumpRule struct {
	typ      string
	regex    *regexp.Regexp
	breaking *bool
	level    versioning.BumpType
}

// compileBumpRules precompiles overrides' regexes. A rule whose regex fails to compile is
// dropped defensively — the config validator rejects invalid regexes upstream, matching the
// same defensive-skip idiom as the native renderer's rendering.excludes handling.
func compileBumpRules(overrides []config.BumpRule) []bumpRule {
	rules := make([]bumpRule, 0, len(overrides))
	for _, o := range overrides {
		var re *regexp.Regexp
		if o.Regex != "" {
			compiled, err := regexp.Compile(o.Regex)
			if err != nil {
				continue
			}
			re = compiled
		}
		rules = append(rules, bumpRule{typ: o.Type, regex: re, breaking: o.Breaking, level: bumpLevelFromString(o.Bump)})
	}
	return rules
}

func bumpLevelFromString(level string) versioning.BumpType {
	switch level {
	case "major":
		return versioning.BumpMajor
	case "minor":
		return versioning.BumpMinor
	case "patch":
		return versioning.BumpPatch
	default: // "none", or anything the validator didn't already reject
		return versioning.BumpNone
	}
}

// DetermineBump scans commits and returns the highest bump level any of them contributes
// (T261). Per commit, in order: the first matching rule in overrides wins (a rule matches when
// every condition it sets — type, regex against the subject, breaking — holds); otherwise the
// built-in defaults apply (breaking → major, feat → minor, any other conventional commit →
// patch); a non-conventional commit matched by nothing contributes nothing. A release with
// nothing to contribute (everything ignored or explicitly "none") resolves to BumpNone —
// callers decide whether that's an error.
func DetermineBump(commits []string, overrides []config.BumpRule) versioning.BumpType {
	rules := compileBumpRules(overrides)
	bump := versioning.BumpNone
	for _, c := range commits {
		if level := resolveBumpLevel(c, rules); level > bump {
			bump = level
		}
	}
	return bump
}

// resolveBumpLevel returns the bump level one commit contributes: BumpNone for a commit that
// matches no rule and isn't a conventional commit at all.
func resolveBumpLevel(c string, rules []bumpRule) versioning.BumpType {
	parsed, err := conventionalcommit.Parse(c)
	isConventional := err == nil
	var typ string
	breaking := false
	if isConventional {
		typ = parsed.Type
		breaking = parsed.Breaking
	}
	subject := firstLine(c)

	for _, r := range rules {
		if r.typ != "" && (!isConventional || typ != r.typ) {
			continue
		}
		if r.regex != nil && !r.regex.MatchString(subject) {
			continue
		}
		if r.breaking != nil && *r.breaking != breaking {
			continue
		}
		return r.level
	}

	if !isConventional {
		return versioning.BumpNone
	}
	if breaking {
		return versioning.BumpMajor
	}
	if typ == "feat" {
		return versioning.BumpMinor
	}
	return versioning.BumpPatch
}

// firstLine returns the subject line (first line) of a raw commit message.
func firstLine(s string) string {
	subject, _, _ := strings.Cut(s, "\n")
	return subject
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
	case versioning.BumpNone:
		// no-op: the resolver decides whether an unchanged version is an error.
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
