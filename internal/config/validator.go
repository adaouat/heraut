package config

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

var (
	validStrategies = map[string]bool{
		"semver": true, "calver": true,
		"semver-per-env": true, "calver-per-env": true,
	}
	validGenerators = map[string]bool{
		"git-cliff": true, "communique": true, "cocogitto": true,
	}
	validPlatforms = map[string]bool{
		"github": true, "gitlab": true,
	}
	validTagTypes = map[string]bool{
		"annotated": true, "lightweight": true,
	}
)

// Validate runs all semantic validation layers against cfg.
// All errors are collected and returned; validation does not stop on the first error.
func Validate(cfg *Config) ValidationErrors {
	if cfg == nil {
		return nil
	}
	var errs ValidationErrors
	errs = append(errs, validateRequired(cfg)...)
	errs = append(errs, validateEnums(cfg)...)
	errs = append(errs, validateStrategySpecific(cfg)...)
	errs = append(errs, validateEnvContradictions(cfg.Environments)...)
	return errs
}

func validateRequired(cfg *Config) []ValidationError {
	var errs []ValidationError
	if cfg.Version == "" {
		errs = append(errs, ValidationError{
			Path:    "version",
			Message: "required",
			Hint:    `set version: "1"`,
		})
	} else if cfg.Version != "1" {
		errs = append(errs, ValidationError{
			Path:    "version",
			Message: fmt.Sprintf("%q is not a valid schema version", cfg.Version),
			Hint:    `the only valid schema version is "1"`,
		})
	}
	if cfg.Versioning.Strategy == "" {
		errs = append(errs, ValidationError{
			Path:    "versioning.strategy",
			Message: "required",
			Hint:    "set strategy to one of: semver, calver, semver-per-env, calver-per-env",
		})
	}
	return errs
}

func validateEnums(cfg *Config) []ValidationError {
	var errs []ValidationError
	if cfg.Versioning.Strategy != "" && !validStrategies[cfg.Versioning.Strategy] {
		errs = append(errs, ValidationError{
			Path:    "versioning.strategy",
			Message: fmt.Sprintf("%q is not a valid strategy", cfg.Versioning.Strategy),
			Hint:    "valid strategies: semver, calver, semver-per-env, calver-per-env",
		})
	}
	if cfg.Versioning.TagType != "" && !validTagTypes[cfg.Versioning.TagType] {
		errs = append(errs, ValidationError{
			Path:    "versioning.tag_type",
			Message: fmt.Sprintf("%q is not a valid tag type", cfg.Versioning.TagType),
			Hint:    "valid tag types: annotated, lightweight",
		})
	}
	errs = append(errs, validateContentDriver(cfg.Changelog, "changelog")...)
	errs = append(errs, validateRelease(cfg.Release, "release")...)
	for envName, env := range cfg.Environments {
		base := "environments." + envName
		// Per-env content drivers merge over the top-level (ADR-0019); validate the
		// effective merged driver so an inherited generator satisfies the required check.
		if env.Changelog != nil {
			eff := MergeContentDriver(cfg.Changelog, env.Changelog)
			errs = append(errs, validateContentDriver(eff, base+".changelog")...)
		}
		errs = append(errs, validateEnvRelease(env.Release, cfg.Release, base+".release")...)
	}
	return errs
}

func validateContentDriver(d *ContentDriver, path string) []ValidationError {
	if d == nil {
		return nil
	}
	var errs []ValidationError
	if d.Generator == "" {
		errs = append(errs, ValidationError{
			Path:    path + ".generator",
			Message: "required",
			Hint:    "set generator to one of: git-cliff, communique, cocogitto",
		})
	} else if !validGenerators[d.Generator] {
		errs = append(errs, ValidationError{
			Path:    path + ".generator",
			Message: fmt.Sprintf("%q is not a valid generator", d.Generator),
			Hint:    "valid generators: git-cliff, communique, cocogitto",
		})
	}
	return errs
}

func validateRelease(r *Release, path string) []ValidationError {
	if r == nil {
		return nil
	}
	var errs []ValidationError
	errs = append(errs, validateContentDriver(r.Notes, path+".notes")...)
	for i, plat := range r.Platforms {
		platPath := fmt.Sprintf("%s.platforms[%d]", path, i)
		if plat.Type == "" {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: "required",
				Hint:    "set platform to one of: github, gitlab",
			})
		} else if !validPlatforms[plat.Type] {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: fmt.Sprintf("%q is not a valid platform", plat.Type),
				Hint:    "valid platforms: github, gitlab",
			})
		}
		errs = append(errs, validatePlatformBaseURL(plat, platPath)...)
	}
	return errs
}

// validatePlatformBaseURL validates a platform's base_url: it must be a well-formed
// absolute http(s) URL, and — per ADR-0020 — it must equal the platform-type default
// until self-hosted host targeting lands (the gate). An empty value means "use the
// default" and is always accepted.
func validatePlatformBaseURL(plat Platform, platPath string) []ValidationError {
	if plat.BaseURL == "" {
		return nil
	}
	raw := strings.TrimRight(plat.BaseURL, "/")
	if !isValidBaseURL(raw) {
		return []ValidationError{{
			Path:    platPath + ".base_url",
			Message: fmt.Sprintf("%q is not a valid URL", plat.BaseURL),
			Hint:    "base_url must be an absolute http(s) URL, e.g. https://gitlab.example.com",
		}}
	}
	if def := defaultBaseURL(plat.Type); def != "" && raw != def {
		return []ValidationError{{
			Path:    platPath + ".base_url",
			Message: "self-hosted hosts are not yet supported",
			Hint: fmt.Sprintf(
				"base_url currently only accepts the platform default (%s); self-hosted publishing is tracked separately (ADR-0020)",
				def,
			),
		}}
	}
	return nil
}

func isValidBaseURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func validateEnvRelease(r *EnvRelease, topRelease *Release, path string) []ValidationError {
	if r == nil {
		return nil
	}
	var errs []ValidationError
	// Per-env notes merge over the top-level release.notes (ADR-0019); validate the
	// effective merged driver.
	if r.Notes != nil {
		var topNotes *ContentDriver
		if topRelease != nil {
			topNotes = topRelease.Notes
		}
		errs = append(errs, validateContentDriver(MergeContentDriver(topNotes, r.Notes), path+".notes")...)
	}
	for i, plat := range r.Platforms {
		platPath := fmt.Sprintf("%s.platforms[%d]", path, i)
		if plat.Type == "" {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: "required",
				Hint:    "set platform to one of: github, gitlab",
			})
		} else if !validPlatforms[plat.Type] {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: fmt.Sprintf("%q is not a valid platform", plat.Type),
				Hint:    "valid platforms: github, gitlab",
			})
		}
		errs = append(errs, validatePlatformBaseURL(plat, platPath)...)
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
		}
	}
	switch cfg.Versioning.Strategy {
	case "semver-per-env", "calver-per-env":
		errs = append(errs, validatePerEnv(cfg)...)
	}
	return errs
}

func validatePerEnv(cfg *Config) []ValidationError {
	var errs []ValidationError
	envs := cfg.Environments

	if len(envs) == 0 {
		return append(errs, ValidationError{
			Path:    "environments",
			Message: fmt.Sprintf("required for %s strategy", cfg.Versioning.Strategy),
			Hint:    "define at least one environment with its tag_format and bump mode",
		})
	}

	// Common tag_format must contain {version} if set.
	if cfg.Versioning.TagFormat != "" && !strings.Contains(cfg.Versioning.TagFormat, "{version}") {
		errs = append(errs, ValidationError{
			Path:    "versioning.tag_format",
			Message: "must contain {version}",
			Hint:    `example: "{env}/{version}"`,
		})
	}

	// Count auto envs for ambiguity detection.
	autoCount := 0
	for _, env := range envs {
		if env.Bump == "auto" {
			autoCount++
		}
	}

	for _, envName := range sortedEnvKeys(envs) {
		env := envs[envName]
		envPath := "environments." + envName

		// bump required and enum.
		if env.Bump == "" {
			errs = append(errs, ValidationError{
				Path:    envPath + ".bump",
				Message: "required",
				Hint:    "set bump to auto or promote",
			})
		} else if env.Bump != "auto" && env.Bump != "promote" {
			errs = append(errs, ValidationError{
				Path:    envPath + ".bump",
				Message: fmt.Sprintf("%q is not a valid bump mode for per-env strategy", env.Bump),
				Hint:    "valid modes: auto, promote",
			})
		}

		// tag_format must contain {version} if set.
		if env.TagFormat != "" && !strings.Contains(env.TagFormat, "{version}") {
			errs = append(errs, ValidationError{
				Path:    envPath + ".tag_format",
				Message: "must contain {version}",
				Hint:    fmt.Sprintf(`example: "%s/{version}"`, envName),
			})
		}

		// Every env must have a tag_format, either directly or via the common one.
		if env.TagFormat == "" && cfg.Versioning.TagFormat == "" {
			errs = append(errs, ValidationError{
				Path:    envPath + ".tag_format",
				Message: "required: no tag_format set on this environment or at versioning.tag_format",
				Hint:    "set tag_format on this environment or set versioning.tag_format as a shared template using {env} and {version}",
			})
		}

		// source validation (ADR-0008).
		sourcePath := envPath + ".source"
		if env.Bump == "auto" && env.Source != "" {
			errs = append(errs, ValidationError{
				Path:    sourcePath,
				Message: "source is not valid on bump: auto environments",
				Hint:    "remove source: from this environment; source is only meaningful for bump: promote",
			})
		}
		if env.Bump == "promote" {
			if env.Source != "" {
				if _, exists := envs[env.Source]; !exists {
					errs = append(errs, ValidationError{
						Path:    sourcePath,
						Message: fmt.Sprintf("environment %q does not exist", env.Source),
						Hint:    "available environments: " + strings.Join(sortedEnvKeys(envs), ", "),
					})
				}
				if env.Source == envName {
					errs = append(errs, ValidationError{
						Path:    sourcePath,
						Message: "environment cannot promote from itself",
						Hint:    "set source to a different environment",
					})
				}
			} else {
				switch {
				case autoCount == 0:
					errs = append(errs, ValidationError{
						Path:    sourcePath,
						Message: "no auto environment found to promote from",
						Hint:    "add source: <env-name> or add an environment with bump: auto",
					})
				case autoCount > 1:
					errs = append(errs, ValidationError{
						Path:    sourcePath,
						Message: "multiple auto environments exist; source is ambiguous",
						Hint:    "add source: <env-name> to specify which environment to promote from",
					})
				}
			}
		}
	}

	errs = append(errs, detectCycles(envs)...)
	return errs
}

func validateEnvContradictions(envs map[string]Environment) []ValidationError {
	var errs []ValidationError
	for _, envName := range sortedEnvKeys(envs) {
		env := envs[envName]
		base := "environments." + envName

		if env.DisableChangelog && env.Changelog != nil {
			errs = append(errs, ValidationError{
				Path:    base + ".changelog",
				Message: "disable_changelog: true makes the changelog override unreachable",
				Hint:    "remove either disable_changelog: true (to apply the override) or the changelog: block (to keep the step disabled)",
			})
		}
		if env.DisableNotes && env.Release != nil && env.Release.Notes != nil {
			errs = append(errs, ValidationError{
				Path:    base + ".release.notes",
				Message: "disable_notes: true makes the release notes override unreachable",
				Hint:    "remove either disable_notes: true (to apply the override) or the release.notes: block (to keep the step disabled)",
			})
		}
	}
	return errs
}

func detectCycles(envs map[string]Environment) []ValidationError {
	var errs []ValidationError
	reported := map[string]bool{}

	for _, envName := range sortedEnvKeys(envs) {
		if reported[envName] {
			continue
		}
		if envs[envName].Bump != "promote" {
			continue
		}
		if found, path := findCycle(envs, envName); found {
			errs = append(errs, ValidationError{
				Path:    "environments." + envName + ".source",
				Message: "cycle detected (" + strings.Join(path, " → ") + ")",
				Hint:    "each promotion source must trace back to an auto env without revisiting envs",
			})
			for _, e := range path {
				reported[e] = true
			}
		}
	}
	return errs
}

func findCycle(envs map[string]Environment, start string) (bool, []string) {
	path := []string{start}
	seen := map[string]bool{start: true}
	current := start

	for {
		env, exists := envs[current]
		if !exists {
			return false, nil
		}
		src := env.Source
		if src == "" {
			return false, nil
		}
		if seen[src] {
			return true, append(path, src)
		}
		seen[src] = true
		path = append(path, src)
		current = src
	}
}

func sortedEnvKeys(envs map[string]Environment) []string {
	keys := make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
