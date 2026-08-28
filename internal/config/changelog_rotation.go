package config

import (
	"fmt"
	"regexp"
)

// rotationTokenPattern matches a {TOKEN} placeholder in a rotating changelog.output pattern,
// e.g. "CHANGELOG_{YYYY}.md" -> "YYYY".
var rotationTokenPattern = regexp.MustCompile(`\{([A-Z]+)\}`)

// calverRotationTokens mirrors internal/versioning/calver's token vocabulary (minus PATCH, which
// is never a valid rotation dimension — see the design doc's Goals). internal/config sits at the
// bottom of the layer graph and must not import internal/versioning/calver (see
// .claude/rules/coding.md's layer table), so this list is intentionally duplicated — keep it in
// sync with calver's own knownTokens if that vocabulary ever changes.
var calverRotationTokens = map[string]bool{
	"YYYY": true, "MM": true, "DD": true, "WW": true, "QQ": true, "SS": true, "SPRINT": true,
}

// semverRotationTokens mirrors the two SemVer components a changelog may rotate by. PATCH is
// deliberately excluded — rotating per patch would create a new file per release.
var semverRotationTokens = map[string]bool{"MAJOR": true, "MINOR": true}

// calverFormatKeywords lists calver's known token keywords, longest-match-first (mirrors
// calver.knownTokens' ordering so "SPRINT" is never scanned as a prefix of anything else).
var calverFormatKeywords = []string{"SPRINT", "PATCH", "YYYY", "MM", "DD", "WW", "QQ", "SS"}

// calverTokenOrder scans a CalVer format string left-to-right and returns the non-PATCH token
// keywords it contains, in the order they appear — e.g. "YYYY.MM.PATCH" -> ["YYYY", "MM"]. This is
// a lightweight, best-effort scan (no literal-segment tracking, no format-validity checking) for
// this validation's purposes only — the authoritative parse lives in calver.ParseFormat, run later
// by internal/app once a resolved version is available (see the changelog-rotation design doc §3).
func calverTokenOrder(format string) []string {
	var order []string
	rem := format
	for rem != "" {
		matched := false
		for _, kw := range calverFormatKeywords {
			if len(rem) >= len(kw) && rem[:len(kw)] == kw {
				if kw != "PATCH" {
					order = append(order, kw)
				}
				rem = rem[len(kw):]
				matched = true
				break
			}
		}
		if !matched {
			rem = rem[1:]
		}
	}
	return order
}

// extractRotationTokens returns the {TOKEN} placeholder names present in output, in the order
// they appear. Returns nil when output has none — the common case, and the signal that a
// changelog.output value is a plain literal path with no rotation behavior at all.
func extractRotationTokens(output string) []string {
	matches := rotationTokenPattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}
	tokens := make([]string, len(matches))
	for i, m := range matches {
		tokens[i] = m[1]
	}
	return tokens
}

// validateChangelogRotation checks {TOKEN} placeholders in d.Output (a rotating changelog file
// name, e.g. "CHANGELOG_{YYYY}.md") against cfg's versioning strategy. d is the effective
// (already-merged, for per-env callers) content driver; path is its config path for error
// reporting (e.g. "changelog" or "environments.prod.changelog"). Returns nil immediately when
// d.Output has no tokens — every existing literal-path config is entirely unaffected.
func validateChangelogRotation(d *ContentDriver, cfg *Config, path string) []ValidationError {
	if d == nil {
		return nil
	}
	tokens := extractRotationTokens(d.Output)
	if len(tokens) == 0 {
		return nil
	}
	outputPath := path + ".output"

	switch cfg.Versioning.Strategy {
	case "calver":
		return validateCalverRotationTokens(tokens, cfg.Versioning.Format, outputPath)
	case "semver":
		return validateSemverRotationTokens(tokens, outputPath)
	case "calver-per-env", "semver-per-env":
		return []ValidationError{{
			Path: outputPath,
			Message: fmt.Sprintf(
				"rotation tokens in changelog.output are not supported with %s yet", cfg.Versioning.Strategy,
			),
			Hint: "remove the {TOKEN} placeholders from output, or switch to a flat semver/calver strategy",
		}}
	default:
		return []ValidationError{{
			Path:    outputPath,
			Message: fmt.Sprintf("rotation tokens in changelog.output require a semver or calver strategy (current: %q)", cfg.Versioning.Strategy),
		}}
	}
}

func validateCalverRotationTokens(tokens []string, format, path string) []ValidationError {
	var errs []ValidationError

	formatOrder := calverTokenOrder(format)
	formatSet := make(map[string]bool, len(formatOrder))
	for _, t := range formatOrder {
		formatSet[t] = true
	}

	seen := make(map[string]bool, len(tokens))
	for _, tok := range tokens {
		switch {
		case !calverRotationTokens[tok]:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("rotation token {%s} is not a valid calver token", tok),
				Hint:    "valid calver rotation tokens: YYYY, MM, DD, WW, QQ, SS, SPRINT",
			})
		case !formatSet[tok]:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("rotation token {%s} is not part of versioning.format %q", tok, format),
				Hint:    "a changelog can only rotate by a dimension the version format itself tracks",
			})
		case seen[tok]:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("rotation token {%s} must not appear more than once", tok),
			})
		default:
			seen[tok] = true
		}
	}
	if len(errs) > 0 {
		return errs
	}

	prefix := formatOrder[:min(len(tokens), len(formatOrder))]
	prefixSet := make(map[string]bool, len(prefix))
	for _, t := range prefix {
		prefixSet[t] = true
	}
	for _, tok := range tokens {
		if !prefixSet[tok] {
			errs = append(errs, ValidationError{
				Path: path,
				Message: fmt.Sprintf(
					"rotation tokens %v are not a prefix of format tokens %v", tokens, formatOrder,
				),
				Hint: "rotation tokens must be the format's own leading tokens, in the format's order",
			})
			break
		}
	}
	return errs
}

func validateSemverRotationTokens(tokens []string, path string) []ValidationError {
	var errs []ValidationError

	seen := make(map[string]bool, len(tokens))
	for _, tok := range tokens {
		switch {
		case !semverRotationTokens[tok]:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("rotation token {%s} is not a valid semver token", tok),
				Hint:    "valid semver rotation tokens: MAJOR, MINOR",
			})
		case seen[tok]:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("rotation token {%s} must not appear more than once", tok),
			})
		default:
			seen[tok] = true
		}
	}
	if len(errs) > 0 {
		return errs
	}

	if len(tokens) == 1 && tokens[0] == "MINOR" {
		errs = append(errs, ValidationError{
			Path:    path,
			Message: "rotation token {MINOR} requires {MAJOR} to also be present",
			Hint:    `use {MAJOR} alone, or {MAJOR}_{MINOR}`,
		})
	}
	return errs
}
