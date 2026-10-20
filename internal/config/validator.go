package config

import (
	"fmt"
)

var (
	validStrategies = map[string]bool{
		"semver": true, "calver": true,
		"semver-per-env": true, "calver-per-env": true,
	}
	validForgePlatforms = map[string]bool{
		"github": true, "gitlab": true, "azure_devops": true,
	}
	validAPIModes = map[string]bool{
		"": true, "rest": true, "graphql": true,
	}
	validEnrichmentPolicies = map[string]bool{
		"": true, "disabled": true, "optional": true, "required": true,
	}
	validTagTypes = map[string]bool{
		"annotated": true, "lightweight": true,
	}
	validBumpModes = map[string]bool{
		"auto": true, "manual": true,
	}
	validBumpLevels = map[string]bool{
		"major": true, "minor": true, "patch": true, "none": true,
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
	errs = append(errs, validateVersioningBump(cfg)...)
	errs = append(errs, validateBranches(cfg)...)
	errs = append(errs, validateStrategySpecific(cfg)...)
	errs = append(errs, validateEnvContradictions(cfg.Environments)...)
	errs = append(errs, validateTickets(cfg)...)
	errs = append(errs, validateCommits(cfg)...)
	errs = append(errs, validateRendering(cfg)...)
	errs = append(errs, validateForges(cfg)...)
	errs = append(errs, validateHooks(cfg)...)
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
	if cfg.Versioning.Bump != nil && cfg.Versioning.Bump.Mode != "" && !validBumpModes[cfg.Versioning.Bump.Mode] {
		errs = append(errs, ValidationError{
			Path:    "versioning.bump.mode",
			Message: fmt.Sprintf("%q is not a valid bump mode", cfg.Versioning.Bump.Mode),
			Hint:    "valid modes: auto, manual",
		})
	}
	errs = append(errs, validateContentDriver(cfg.Changelog, "changelog")...)
	errs = append(errs, validateChangelogRotation(cfg.Changelog, cfg, "changelog")...)
	if cfg.Notes != nil {
		errs = append(errs, validateContentDriverTemplates(&ContentDriver{Template: cfg.Notes.Template, Rendering: cfg.Notes.Rendering}, "notes")...)
	}
	errs = append(errs, validateRelease(cfg.Release, "release")...)
	for _, envName := range sortedEnvKeys(cfg.Environments) {
		env := cfg.Environments[envName]
		base := "environments." + envName
		// Per-env content drivers merge over the top-level (ADR-0019); validate the
		// effective merged driver so an inherited generator satisfies the required check.
		if env.Changelog != nil {
			eff := MergeContentDriver(cfg.Changelog, env.Changelog)
			errs = append(errs, validateContentDriver(eff, base+".changelog")...)
			errs = append(errs, validateChangelogRotation(eff, cfg, base+".changelog")...)
		}
		errs = append(errs, validateEnvRelease(env.Release, cfg.Release, base+".release")...)
	}
	return errs
}
