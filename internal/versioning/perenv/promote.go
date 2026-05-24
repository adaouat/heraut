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

func resolvePromote(runner port.Runner, cfg *config.Config, env string, force bool) (versioning.Result, error) {
	// 1. Determine which environment to promote from.
	srcEnv, err := resolveSourceEnv(cfg, env)
	if err != nil {
		return versioning.Result{}, err
	}

	srcEnvCfg := cfg.Versioning.Environments[srcEnv]

	// 2. List source tags and find the latest.
	srcGlob, err := tagfmt.GlobPattern(srcEnvCfg.TagFormat, srcEnv)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building source tag glob: %w", err)
	}

	stdout, _, err := runner.Run("git", "tag", "-l", srcGlob, "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing source tags: %w", err)
	}

	srcTags := splitLines(stdout)
	if len(srcTags) == 0 {
		return versioning.Result{}, fmt.Errorf("%w: no tags found in source environment %q", ErrNoSourceTags, srcEnv)
	}

	// 3. Extract the bare version from the latest source tag.
	latestSrcTag := srcTags[0]
	candidateVersion, err := tagfmt.ParseVersion(srcEnvCfg.TagFormat, latestSrcTag)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("parsing source tag %q: %w", latestSrcTag, err)
	}

	// 4. Render the candidate tag under the destination format.
	destEnvCfg := cfg.Versioning.Environments[env]
	candidateTag, err := tagfmt.Render(destEnvCfg.TagFormat, env, candidateVersion)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("rendering candidate tag: %w", err)
	}

	// 5. E001: fail if the candidate tag already exists.
	stdout, _, err = runner.Run("git", "tag", "-l", candidateTag)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("checking candidate tag existence: %w", err)
	}
	if strings.TrimSpace(stdout) != "" && !force {
		return versioning.Result{}, fmt.Errorf("%w: tag %q already exists (pass --force to bypass)",
			ErrTargetExists, candidateTag)
	}

	// 6. E002: fail if the destination is already ahead of the candidate.
	destGlob, err := tagfmt.GlobPattern(destEnvCfg.TagFormat, env)
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
		latestDestVersion, parseErr := tagfmt.ParseVersion(destEnvCfg.TagFormat, currentDestTag)
		if parseErr == nil {
			if compareVersionStrings(latestDestVersion, candidateVersion) > 0 && !force {
				return versioning.Result{}, fmt.Errorf(
					"%w: destination %q is at %s, candidate is %s (pass --force to bypass)",
					ErrDestinationAhead, env, latestDestVersion, candidateVersion)
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
