package config

import (
	"errors"
	"fmt"
	pathpkg "path"
	"regexp"
	"strings"
)

// validateVersioningBump validates versioning.bump.overrides (T261): each rule sets exactly one
// of type or regex (when either is set), at least one of type/regex/breaking, any regex compiles,
// and bump is a valid level.
func validateVersioningBump(cfg *Config) []ValidationError {
	if cfg.Versioning.Bump == nil {
		return nil
	}
	var errs []ValidationError
	for i, r := range cfg.Versioning.Bump.Overrides {
		path := fmt.Sprintf("versioning.bump.overrides[%d]", i)
		switch {
		case r.Type == "" && r.Regex == "" && r.Breaking == nil:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "must set at least one of type, regex, or breaking",
				Hint:    `e.g. {type: chore, bump: none} or {breaking: true, bump: minor}`,
			})
		case r.Type != "" && r.Regex != "":
			errs = append(errs, ValidationError{Path: path, Message: "set only one of type or regex, not both"})
		}
		if r.Regex != "" {
			if _, err := regexp.Compile(r.Regex); err != nil {
				errs = append(errs, ValidationError{Path: path + ".regex", Message: fmt.Sprintf("invalid regex: %v", err)})
			}
		}
		if !validBumpLevels[r.Bump] {
			errs = append(errs, ValidationError{
				Path:    path + ".bump",
				Message: fmt.Sprintf("%q is not a valid bump level", r.Bump),
				Hint:    "valid levels: major, minor, patch, none",
			})
		}
	}
	return errs
}

// validateBranches validates versioning.branches (ADR-0065): semver strategy only, each entry
// has a name that compiles as a path.Match pattern and an optional range that parses, and
// explicit ranges are unique.
func validateBranches(cfg *Config) []ValidationError {
	branches := cfg.Versioning.Branches
	if len(branches) == 0 {
		return nil
	}
	if cfg.Versioning.Strategy != "semver" {
		return []ValidationError{{
			Path:    "versioning.branches",
			Message: fmt.Sprintf("only valid with strategy: semver (current strategy: %s)", cfg.Versioning.Strategy),
			Hint:    "per-env strategies tie branches to environments via environments.<env>.branch; remove versioning.branches",
		}}
	}
	var errs []ValidationError
	seen := make(map[string]int, len(branches))
	for i, b := range branches {
		path := fmt.Sprintf("versioning.branches[%d]", i)
		if b.Name == "" {
			errs = append(errs, ValidationError{Path: path + ".name", Message: "required"})
		} else if _, err := pathpkg.Match(b.Name, ""); errors.Is(err, pathpkg.ErrBadPattern) {
			errs = append(errs, ValidationError{Path: path + ".name", Message: "invalid glob"})
		}
		if b.Range == "" {
			continue
		}
		r, err := ParseBranchRange(b.Range)
		if err != nil {
			errs = append(errs, ValidationError{
				Path:    path + ".range",
				Message: fmt.Sprintf("%q is not a valid range", b.Range),
				Hint:    "use N.x (patch and minor) or N.N.x (patch only), e.g. 1.x or 1.3.x",
			})
			continue
		}
		key := r.String()
		if first, dup := seen[key]; dup {
			errs = append(errs, ValidationError{
				Path:    path + ".range",
				Message: fmt.Sprintf("duplicates versioning.branches[%d].range (%s)", first, key),
			})
			continue
		}
		seen[key] = i
	}
	return errs
}

func validateStrategySpecific(cfg *Config) []ValidationError {
	var errs []ValidationError

	// Flat-strategy guard: environments is only valid with per-env strategies.
	if len(cfg.Environments) > 0 {
		switch cfg.Versioning.Strategy {
		case "semver-per-env", "calver-per-env":
			// expected
		default:
			if cfg.Versioning.Strategy != "" {
				errs = append(errs, ValidationError{
					Path:    "environments",
					Message: fmt.Sprintf("environments is only valid with semver-per-env or calver-per-env (current strategy: %s)", cfg.Versioning.Strategy),
					Hint:    "remove the environments block, or change the strategy to semver-per-env or calver-per-env",
				})
			}
		}
	}

	switch cfg.Versioning.Strategy {
	case "calver", "calver-per-env":
		if cfg.Versioning.Format == "" {
			errs = append(errs, ValidationError{
				Path:    "versioning.format",
				Message: fmt.Sprintf("required for %s strategy", cfg.Versioning.Strategy),
				Hint:    `set a CalVer format string, e.g. "YYYY.MM.PATCH"`,
			})
		} else if strings.Contains(cfg.Versioning.Format, "SPRINT") && cfg.Versioning.Sprint < 1 {
			errs = append(errs, ValidationError{
				Path:    "versioning.sprint",
				Message: "required: format contains the SPRINT token but sprint is unset",
				Hint:    "set sprint to a positive integer, e.g. sprint: 1, and advance it with `heraut version sprint bump`",
			})
		}
	}
	if buildTokenMisplaced(cfg.Versioning.TagFormat) {
		errs = append(errs, buildTokenError("versioning.tag_format"))
	}
	switch cfg.Versioning.Strategy {
	case "semver-per-env", "calver-per-env":
		errs = append(errs, validatePerEnv(cfg)...)
	}
	return errs
}

// tagFormatMissingVersion reports whether a non-empty tag_format value is missing the required
// {version} token. Shared by validatePerEnv's common and per-environment checks below, and by
// ValidateTagFormatForWizard's live heraut init validation (T259), so the rule can't drift between
// config.Validate and the wizard.
func tagFormatMissingVersion(s string) bool {
	return s != "" && !strings.Contains(s, "{version}")
}

// buildTokenMisplaced reports whether a tag_format uses {build} anywhere other than directly
// after a single "+" — the only placement that makes it SemVer build metadata (§10) rather than a
// pre-release identifier (ADR-0064). "++{build}" is also misplaced: SemVer build metadata has no
// empty identifiers, so "{version}++{build}" would render an invalid version. Shared with
// ValidateTagFormatForWizard.
func buildTokenMisplaced(s string) bool {
	const token = "{build}"
	for i := 0; ; {
		j := strings.Index(s[i:], token)
		if j < 0 {
			return false
		}
		pos := i + j
		if pos == 0 || s[pos-1] != '+' || (pos >= 2 && s[pos-2] == '+') {
			return true
		}
		i = pos + len(token)
	}
}

func buildTokenError(path string) ValidationError {
	return ValidationError{
		Path:    path,
		Message: `{build} must directly follow "+" (SemVer build metadata)`,
		Hint:    `write "{version}+{build}", e.g. "{env}/{version}+{build}" — "-{build}" would make the build ID a pre-release identifier`,
	}
}

// ValidateTagFormatForWizard is a minimal wizard-facing wrapper around tagFormatMissingVersion and
// buildTokenMisplaced, for heraut init's live, per-keystroke validation of both the common
// (versioning.tag_format) and per-environment tag_format fields (T259) — same rules, same
// messages, either field.
func ValidateTagFormatForWizard(s string) error {
	if tagFormatMissingVersion(s) {
		return errors.New("must contain {version}")
	}
	if buildTokenMisplaced(s) {
		return errors.New(`{build} must directly follow "+" — use "{version}+{build}"`)
	}
	return nil
}
