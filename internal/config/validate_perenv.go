package config

import (
	"fmt"
	"sort"
	"strings"
)

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
	if tagFormatMissingVersion(cfg.Versioning.TagFormat) {
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
		if tagFormatMissingVersion(env.TagFormat) {
			errs = append(errs, ValidationError{
				Path:    envPath + ".tag_format",
				Message: "must contain {version}",
				Hint:    fmt.Sprintf(`example: "%s/{version}"`, envName),
			})
		}

		if buildTokenMisplaced(env.TagFormat) {
			errs = append(errs, buildTokenError(envPath+".tag_format"))
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
		if env.DisableRelease && env.Release != nil {
			errs = append(errs, ValidationError{
				Path:    base + ".release",
				Message: "disable_release: true makes the release override unreachable",
				Hint:    "remove either disable_release: true (to apply the override) or the release: block (to keep the whole step disabled)",
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
