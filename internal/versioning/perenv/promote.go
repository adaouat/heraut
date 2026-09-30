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

	srcEnv  string
	destEnv string

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
	srcGlob, err := tagfmt.GlobPattern(srcTF, tagfmt.Tokens{Env: srcEnv})
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building source tag glob: %w", err)
	}

	stdout, _, err := runner.Run("git", "tag", "-l", srcGlob, "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing source tags: %w", err)
	}

	// 3. Pick the latest source release (never a pre-release) — the same selection resolveAuto
	// uses (T92, ADR-0064).
	srcTags, srcVersions := releaseTags(cfg.Versioning.Strategy, srcTF, splitLines(stdout))
	if len(srcTags) == 0 {
		return versioning.Result{}, &PromotionError{
			sentinel: ErrNoSourceTags,
			srcEnv:   srcEnv,
			destEnv:  env,
			srcGlob:  srcGlob,
		}
	}
	latestSrcTag, candidateVersion := srcTags[0], srcVersions[0]

	// 4. Render the candidate tag under the destination format.
	destTF := tagFormat(cfg, env)
	candidateTag, err := tagfmt.Render(destTF, tagfmt.Tokens{Env: env, Version: candidateVersion})
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
	destGlob, err := tagfmt.GlobPattern(destTF, tagfmt.Tokens{Env: env})
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building destination tag glob: %w", err)
	}

	stdout, _, err = runner.Run("git", "tag", "-l", destGlob, "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing destination tags: %w", err)
	}

	destRawTags := splitLines(stdout)
	currentDestTag, latestDestVersion, destComparable := latestTag(cfg.Versioning.Strategy, destTF, destRawTags)
	if destComparable && compareVersions(cfg.Versioning.Strategy, latestDestVersion, candidateVersion) > 0 && !force {
		suggested, renderErr := tagfmt.Render(srcTF, tagfmt.Tokens{Env: srcEnv, Version: latestDestVersion})
		if renderErr != nil {
			// srcTF needing a {build} token can't render a suggested tag from a promoted version alone
			// — no build ID survives promotion to reuse. Fall back to a placeholder instead of leaving
			// the hint with an empty tag name.
			suggested = fmt.Sprintf("<no suggested tag — %s's tag_format needs a build ID>", srcEnv)
		}
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

	// Result.CurrentTag feeds the promote hook's previous_tag: it must be the highest RELEASE
	// tag, not the highest tag of any kind used above for the E002 comparison — a pre-release
	// ahead of the candidate must still fail E002, but must never be reported as the "current"
	// tag being promoted from (ADR-0064). Falls back to currentDestTag (today's behaviour) when
	// no destination tag parses as a release. calver-per-env has no pre-release concept, so its
	// branch is left byte-for-byte unchanged.
	reportedDestTag := currentDestTag
	if cfg.Versioning.Strategy == semverPerEnv {
		if destReleases, _ := releaseTags(cfg.Versioning.Strategy, destTF, destRawTags); len(destReleases) > 0 {
			reportedDestTag = destReleases[0]
		}
	}

	return versioning.Result{
		Version:    candidateVersion,
		Tag:        candidateTag,
		CurrentTag: reportedDestTag,
	}, nil
}

// resolveSourceEnv determines which environment to promote from.
// Returns the source env name, or an error if ambiguous or a cycle is detected.
func resolveSourceEnv(cfg *config.Config, env string) (string, error) {
	envCfg := cfg.Environments[env]

	if envCfg.Source != "" {
		// Explicit source: check for self-reference (simplest cycle form).
		if envCfg.Source == env {
			return "", fmt.Errorf("cycle detected in source chain: environment %q references itself", env)
		}
		if _, ok := cfg.Environments[envCfg.Source]; !ok {
			return "", fmt.Errorf("source environment %q not found in config", envCfg.Source)
		}
		return envCfg.Source, nil
	}

	// Default: find the single bump:auto environment.
	var autoEnvs []string
	for name, e := range cfg.Environments {
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
