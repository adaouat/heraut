package config

import (
	"fmt"
	"net/url"
)

// validateForges validates the forges/release.targets/commits.enrichment_* keys
// (ADR-0043/ADR-0044): each forge's name is non-empty and unique, its platform and api_mode are valid enums;
// commits.enrichment_policy is a valid enum; commits.enrichment_forge and each
// release.targets[].forge, when set, must name a known forge, and are required when more than
// one forge is configured (unambiguous with exactly one). Resolution-time concerns (e.g.
// api_mode: graphql requiring a resolvable token) are out of scope here — see T157.
// forgeDestination is the coordinate tuple two forges: entries collide on when they point at the
// same place — comparing names alone (as duplicate-name detection does) misses two distinctly
// named entries that resolve identically at runtime, e.g. two `platform: github` entries with no
// explicit repository, both filled from the same CI env or git origin (T176).
type forgeDestination struct {
	Type       string
	BaseURL    string
	Project    string
	Repository string
}

func validateForges(cfg *Config) []ValidationError {
	var errs []ValidationError

	knownForges := make(map[string]bool, len(cfg.Forges))
	seen := make(map[string]int, len(cfg.Forges))
	seenDest := make(map[forgeDestination]int, len(cfg.Forges))
	for i, f := range cfg.Forges {
		path := fmt.Sprintf("forges[%d]", i)
		if f.Name == "" {
			errs = append(errs, ValidationError{
				Path:    path + ".name",
				Message: "required",
				Hint:    `set a unique name for this forge, e.g. "gitlab"`,
			})
		} else if first, ok := seen[f.Name]; ok {
			errs = append(errs, ValidationError{
				Path:    path + ".name",
				Message: fmt.Sprintf("duplicate forge name %q (already used by forges[%d])", f.Name, first),
				Hint:    "forge names must be unique within forges",
			})
		} else {
			seen[f.Name] = i
			knownForges[f.Name] = true
		}
		if f.Type == "" {
			errs = append(errs, ValidationError{
				Path:    path + ".platform",
				Message: "required",
				Hint:    "set platform to one of: github, gitlab, azure_devops",
			})
		} else if !validForgePlatforms[f.Type] {
			errs = append(errs, ValidationError{
				Path:    path + ".platform",
				Message: fmt.Sprintf("%q is not a valid platform", f.Type),
				Hint:    "valid platforms: github, gitlab, azure_devops",
			})
		}
		if !validAPIModes[f.APIMode] {
			errs = append(errs, ValidationError{
				Path:    path + ".api_mode",
				Message: fmt.Sprintf("%q is not a valid api_mode", f.APIMode),
				Hint:    "valid api_mode values: rest, graphql",
			})
		}
		if f.BaseURL != "" {
			switch {
			case !isValidBaseURL(f.BaseURL):
				errs = append(errs, ValidationError{
					Path:    path + ".base_url",
					Message: fmt.Sprintf("%q is not a valid URL", f.BaseURL),
					Hint:    "base_url must be an absolute https URL, e.g. https://gitlab.example.com",
				})
			case !IsSecureURL(f.BaseURL):
				errs = append(errs, ValidationError{
					Path:    path + ".base_url",
					Message: fmt.Sprintf("%q must use https", f.BaseURL),
					Hint:    "the forge token is sent to this host: use an https URL (plain http is accepted only for localhost and loopback addresses)",
				})
			}
		}
		if f.APIURL != "" {
			switch {
			case !isValidBaseURL(f.APIURL):
				errs = append(errs, ValidationError{
					Path:    path + ".api_url",
					Message: fmt.Sprintf("%q is not a valid URL", f.APIURL),
					Hint:    "api_url must be an absolute https URL, e.g. https://gitlab.example.com/api",
				})
			case !IsSecureURL(f.APIURL):
				errs = append(errs, ValidationError{
					Path:    path + ".api_url",
					Message: fmt.Sprintf("%q must use https", f.APIURL),
					Hint:    "the forge token is sent to this URL: use an https URL (plain http is accepted only for localhost and loopback addresses)",
				})
			}
		}

		// Only entries with a valid platform take part in the destination scan: an entry
		// already rejected above (missing/unknown platform) would otherwise draw a second
		// error for one mistake (T176, mirroring validateTargetForges's own skip).
		if f.Type != "" && validForgePlatforms[f.Type] {
			dest := forgeDestination{Type: f.Type, BaseURL: f.BaseURL, Project: f.Project, Repository: f.Repository}
			if first, ok := seenDest[dest]; ok {
				errs = append(errs, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("resolves to the same destination as forges[%d] (identical platform/base_url/project/repository)", first),
					Hint:    "set an explicit base_url and/or project/repository to disambiguate, or remove the duplicate forge",
				})
			} else {
				seenDest[dest] = i
			}
		}
	}

	if cfg.Commits != nil {
		if !validEnrichmentPolicies[cfg.Commits.EnrichmentPolicy] {
			errs = append(errs, ValidationError{
				Path:    "commits.enrichment_policy",
				Message: fmt.Sprintf("%q is not a valid enrichment_policy", cfg.Commits.EnrichmentPolicy),
				Hint:    "valid values: disabled, optional, required",
			})
		}
		switch ef := cfg.Commits.EnrichmentForge; {
		case ef != "" && !knownForges[ef]:
			errs = append(errs, ValidationError{
				Path:    "commits.enrichment_forge",
				Message: fmt.Sprintf("unknown forge %q", ef),
				Hint:    "must match one of forges[].name",
			})
		case ef == "" && len(cfg.Forges) > 1:
			errs = append(errs, ValidationError{
				Path:    "commits.enrichment_forge",
				Message: "required when more than one forge is configured",
				Hint:    "set commits.enrichment_forge to one of forges[].name",
			})
		}
	}

	if cfg.Release != nil {
		errs = append(errs, validateTargetForges(cfg.Release.Targets, knownForges, cfg.Forges, "release.targets")...)
	}

	for _, envName := range sortedEnvKeys(cfg.Environments) {
		env := cfg.Environments[envName]
		if env.Release == nil {
			continue
		}
		path := fmt.Sprintf("environments.%s.release.targets", envName)
		errs = append(errs, validateTargetForges(env.Release.Targets, knownForges, cfg.Forges, path)...)
	}

	return errs
}

// autoDetectedForge stands in for the identity every bare target shares when no forges: entry is
// configured — internal/app's resolveTargetForge hands them all the same auto-detected forge, so
// they collide with each other. The NUL prefix keeps it distinct from any user-supplied name.
const autoDetectedForge = "\x00auto-detected"

// validateTargetForges validates a release.targets list (top-level or per-environment): each
// entry's forge, when set, must name a known forge, and is required when more than one forge is
// configured (unambiguous with exactly one). It also rejects a target set that is unsatisfiable —
// two or more targets that would resolve to the same forge — since that lets the first
// `release create` succeed and the second fail after the tag has already been pushed (T171).
// pathPrefix is "release.targets" at the top level or "environments.<env>.release.targets" for a
// per-env override.
func validateTargetForges(targets []Target, knownForges map[string]bool, forges []Forge, pathPrefix string) []ValidationError {
	var errs []ValidationError
	seen := map[string]bool{}
	duplicate := false
	for i, t := range targets {
		path := fmt.Sprintf("%s[%d].forge", pathPrefix, i)
		switch {
		case t.Forge != "" && !knownForges[t.Forge]:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("unknown forge %q", t.Forge),
				Hint:    "must match one of forges[].name",
			})
		case t.Forge == "" && len(forges) > 1:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "required when more than one forge is configured",
				Hint:    "set forge to one of forges[].name",
			})
		default:
			// Only targets that resolve at all take part in the duplicate scan: an entry
			// already rejected above would otherwise draw a second error for one mistake.
			dest := resolvedForgeName(t, forges)
			if seen[dest] {
				duplicate = true
			}
			seen[dest] = true
		}
	}
	if duplicate {
		errs = append(errs, ValidationError{
			Path:    pathPrefix,
			Message: "more than one target resolves to the same forge",
			Hint:    "declare a forges: entry per destination and set forge on each target to disambiguate",
		})
	}
	return errs
}

// resolvedForgeName is the forge a target will resolve to at release time, mirroring
// internal/app's resolveTargetForge: an explicit forge names itself, a bare target takes the sole
// configured forge, and with no forges: at all every bare target shares the auto-detected identity.
func resolvedForgeName(t Target, forges []Forge) string {
	switch {
	case t.Forge != "":
		return t.Forge
	case len(forges) == 1:
		return forges[0].Name
	default:
		return autoDetectedForge
	}
}

func validateRelease(r *Release, path string) []ValidationError {
	if r == nil {
		return nil
	}
	return validateContentDriver(r.Notes, path+".notes")
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
	return errs
}
