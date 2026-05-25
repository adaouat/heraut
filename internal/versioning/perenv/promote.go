package perenv

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

// PromotionError is the rich error type for the three per-env promotion guards
// (ADR-0007). It wraps the sentinel error so errors.Is works through the chain
// and renders a Biome-style multi-line message via Error().
type PromotionError struct {
	sentinel error

	srcEnv   string
	destEnv  string

	// E001 + E002: source and candidate tags
	srcTag       string
	candidateTag string

	// E002: destination context for regression message
	latestDestTag     string
	latestDestVersion string
	suggestedSrcTag   string

	// E003: glob pattern for the source env
	srcGlob string
}

func (e *PromotionError) Error() string {
	switch e.sentinel {
	case ErrTargetExists:
		return e.renderE001()
	case ErrDestinationAhead:
		return e.renderE002()
	default:
		return e.renderE003()
	}
}

func (e *PromotionError) Unwrap() error { return e.sentinel }

func (e *PromotionError) renderE001() string {
	return fmt.Sprintf(
		"error[E001]: tag already exists\n"+
			"\n"+
			"  Promoting %s would create %s, but that tag already exists.\n"+
			"\n"+
			"  Found:\n"+
			"    latest %s tag  →  %s  (promotion candidate)\n"+
			"    %s             →  already exists ✗\n"+
			"\n"+
			"  Tags are immutable. Creating a duplicate tag overwrites history.\n"+
			"\n"+
			"  How to fix:\n"+
			"    · If this version was already promoted, no action is needed.\n"+
			"    · If you need to re-release, remove the existing tag first:\n"+
			"        git tag -d %s\n"+
			"        git push origin :refs/tags/%s\n"+
			"    · To bypass this check: heraut release --env %s --force",
		e.srcTag, e.candidateTag,
		e.srcEnv, e.srcTag,
		e.candidateTag,
		e.candidateTag, e.candidateTag,
		e.destEnv,
	)
}

func (e *PromotionError) renderE002() string {
	return fmt.Sprintf(
		"error[E002]: version regression detected\n"+
			"\n"+
			"  Promoting %s would create %s, but %s already exists.\n"+
			"  This would move %s backwards.\n"+
			"\n"+
			"  Found:\n"+
			"    latest %s tag   →  %s  (promotion candidate)\n"+
			"    latest %s tag  →  %s (higher than candidate ✗)\n"+
			"\n"+
			"  How to fix:\n"+
			"    · Check whether a hotfix was applied directly to %s without a %s tag.\n"+
			"      If so, create the corresponding %s tag first:\n"+
			"        git tag %s <commit-sha>\n"+
			"        git push origin %s\n"+
			"    · Or promote from a newer %s version once one exists.\n"+
			"    · To bypass this check: heraut release --env %s --force",
		e.srcTag, e.candidateTag, e.latestDestTag,
		e.destEnv,
		e.srcEnv, e.srcTag,
		e.destEnv, e.latestDestTag,
		e.destEnv, e.srcEnv,
		e.srcEnv, e.suggestedSrcTag, e.suggestedSrcTag,
		e.srcEnv,
		e.destEnv,
	)
}

func (e *PromotionError) renderE003() string {
	return fmt.Sprintf(
		"error[E003]: no source tags found\n"+
			"\n"+
			"  Cannot promote to %s: no %s tags exist in this repository.\n"+
			"\n"+
			"  bump: promote requires at least one release in the source environment before\n"+
			"  promoting.\n"+
			"\n"+
			"  How to fix:\n"+
			"    1. Create a source release first:\n"+
			"         heraut release --env %s\n"+
			"    2. Then promote to the destination:\n"+
			"         heraut release --env %s\n"+
			"\n"+
			"  Note: --force cannot bypass this error (there is no version to promote).",
		e.destEnv, e.srcGlob,
		e.srcEnv,
		e.destEnv,
	)
}

func resolvePromote(runner port.Runner, cfg *config.Config, env string, force bool) (versioning.Result, error) {
	// 1. Determine which environment to promote from.
	srcEnv, err := resolveSourceEnv(cfg, env)
	if err != nil {
		return versioning.Result{}, err
	}

	srcTF := tagFormat(cfg, srcEnv)

	// 2. List source tags and find the latest.
	srcGlob, err := tagfmt.GlobPattern(srcTF, srcEnv)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building source tag glob: %w", err)
	}

	stdout, _, err := runner.Run("git", "tag", "-l", srcGlob, "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing source tags: %w", err)
	}

	srcTags := splitLines(stdout)
	if len(srcTags) == 0 {
		return versioning.Result{}, &PromotionError{
			sentinel: ErrNoSourceTags,
			srcEnv:   srcEnv,
			destEnv:  env,
			srcGlob:  srcGlob,
		}
	}

	// 3. Extract the bare version from the latest source tag.
	latestSrcTag := srcTags[0]
	candidateVersion, err := tagfmt.ParseVersion(srcTF, latestSrcTag)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("parsing source tag %q: %w", latestSrcTag, err)
	}

	// 4. Render the candidate tag under the destination format.
	destTF := tagFormat(cfg, env)
	candidateTag, err := tagfmt.Render(destTF, env, candidateVersion)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("rendering candidate tag: %w", err)
	}

	// 5. E001: fail if the candidate tag already exists.
	stdout, _, err = runner.Run("git", "tag", "-l", candidateTag)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("checking candidate tag existence: %w", err)
	}
	if strings.TrimSpace(stdout) != "" && !force {
		return versioning.Result{}, &PromotionError{
			sentinel:     ErrTargetExists,
			srcEnv:       srcEnv,
			destEnv:      env,
			srcTag:       latestSrcTag,
			candidateTag: candidateTag,
		}
	}

	// 6. E002: fail if the destination is already ahead of the candidate.
	destGlob, err := tagfmt.GlobPattern(destTF, env)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building destination tag glob: %w", err)
	}

	stdout, _, err = runner.Run("git", "tag", "-l", destGlob, "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing destination tags: %w", err)
	}

	destTags := splitLines(stdout)
	var currentDestTag string
	if len(destTags) > 0 {
		currentDestTag = destTags[0]
		latestDestVersion, parseErr := tagfmt.ParseVersion(destTF, currentDestTag)
		if parseErr == nil {
			if compareVersionStrings(latestDestVersion, candidateVersion) > 0 && !force {
				suggested, _ := tagfmt.Render(srcTF, srcEnv, latestDestVersion)
				return versioning.Result{}, &PromotionError{
					sentinel:          ErrDestinationAhead,
					srcEnv:            srcEnv,
					destEnv:           env,
					srcTag:            latestSrcTag,
					candidateTag:      candidateTag,
					latestDestTag:     currentDestTag,
					latestDestVersion: latestDestVersion,
					suggestedSrcTag:   suggested,
				}
			}
		}
	}

	return versioning.Result{
		Version:    candidateVersion,
		Tag:        candidateTag,
		CurrentTag: currentDestTag,
	}, nil
}

// resolveSourceEnv determines which environment to promote from.
// Returns the source env name, or an error if ambiguous or a cycle is detected.
func resolveSourceEnv(cfg *config.Config, env string) (string, error) {
	envCfg := cfg.Versioning.Environments[env]

	if envCfg.Source != "" {
		// Explicit source: check for self-reference (simplest cycle form).
		if envCfg.Source == env {
			return "", fmt.Errorf("cycle detected in source chain: environment %q references itself", env)
		}
		if _, ok := cfg.Versioning.Environments[envCfg.Source]; !ok {
			return "", fmt.Errorf("source environment %q not found in config", envCfg.Source)
		}
		return envCfg.Source, nil
	}

	// Default: find the single bump:auto environment.
	var autoEnvs []string
	for name, e := range cfg.Versioning.Environments {
		if e.Bump == "auto" {
			autoEnvs = append(autoEnvs, name)
		}
	}

	switch len(autoEnvs) {
	case 1:
		return autoEnvs[0], nil
	case 0:
		return "", fmt.Errorf("no auto environment found as promotion source for %q; set source: in config", env)
	default:
		return "", fmt.Errorf("ambiguous source for %q: %d auto environments exist; set source: to resolve", env, len(autoEnvs))
	}
}

// compareVersionStrings compares two dot-separated version strings component by component.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// Works for both SemVer (1.2.3) and CalVer (2026.05.3) since both use dot-separated integers.
func compareVersionStrings(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}

	for i := range n {
		var av, bv int
		if i < len(aParts) {
			av, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bv, _ = strconv.Atoi(bParts[i])
		}
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}
	return 0
}
