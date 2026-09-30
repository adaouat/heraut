package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

// errNoTagsFound is the sentinel CurrentTag returns when no tags match the resolved glob.
var errNoTagsFound = errors.New("no tags found")

// CurrentTag returns the latest existing git tag for the given strategy and environment. For
// SemVer strategies it is the highest SemVer §11 release, or — with includePreRelease — the
// highest tag including pre-releases (ADR-0064). CalVer strategies ignore includePreRelease and
// keep git's version:refname order. For single-env strategies, env is ignored; for per-env
// strategies, env is required.
func CurrentTag(runner port.Runner, cfg *config.Config, env string, includePreRelease bool) (string, error) {
	glob, err := currentTagGlob(cfg, env)
	if err != nil {
		return "", err
	}

	stdout, _, err := runner.Run("git", "tag", "-l", glob, "--sort=-version:refname")
	if err != nil {
		return "", fmt.Errorf("listing git tags: %w", err)
	}

	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}

	switch cfg.Versioning.Strategy {
	case "semver", "semver-per-env":
		sorted := semver.SortTags(lines, semverExtractor(cfg, env))
		if tv, ok := semver.Latest(sorted, includePreRelease); ok {
			return tv.Tag, nil
		}
		if len(sorted) > 0 {
			return "", fmt.Errorf("%w for %q: only pre-release tags exist (pass --include-pre-release to show them)", errNoTagsFound, glob)
		}
		return "", fmt.Errorf("%w for %q", errNoTagsFound, glob)
	default:
		if len(lines) > 0 {
			return lines[0], nil
		}
		return "", fmt.Errorf("%w for %q", errNoTagsFound, glob)
	}
}

// semverExtractor returns how to read a tag's bare version: strip tag_prefix for plain semver,
// parse through the effective tag_format for semver-per-env.
func semverExtractor(cfg *config.Config, env string) func(string) (string, bool) {
	if cfg.Versioning.Strategy == "semver-per-env" {
		tf := cfg.EffectiveTagFormat(env)
		return func(tag string) (string, bool) {
			v, err := tagfmt.ParseVersion(tf, tag)
			return v, err == nil
		}
	}
	prefix := configuredTagPrefix(cfg)
	return func(tag string) (string, bool) { return strings.CutPrefix(tag, prefix) }
}

// CurrentVersion returns the bare semantic version of the latest tag (the tag with
// any prefix / env / build components stripped). For per-env strategies the version is
// parsed via the effective tag_format; for single-env strategies the tag prefix is
// stripped. includePreRelease is passed through to CurrentTag unchanged.
func CurrentVersion(runner port.Runner, cfg *config.Config, env string, includePreRelease bool) (string, error) {
	tag, err := CurrentTag(runner, cfg, env, includePreRelease)
	if err != nil {
		return "", err
	}
	switch cfg.Versioning.Strategy {
	case "semver":
		prefix := "v"
		if cfg.Versioning.TagPrefix != nil {
			prefix = *cfg.Versioning.TagPrefix
		}
		return strings.TrimPrefix(tag, prefix), nil
	case "calver":
		prefix := ""
		if cfg.Versioning.TagPrefix != nil {
			prefix = *cfg.Versioning.TagPrefix
		}
		return strings.TrimPrefix(tag, prefix), nil
	case "semver-per-env", "calver-per-env":
		v, err := tagfmt.ParseVersion(cfg.EffectiveTagFormat(env), tag)
		if err != nil {
			return "", fmt.Errorf("parsing version from tag %q: %w", tag, err)
		}
		return v, nil
	default:
		return "", fmt.Errorf("unknown versioning strategy %q", cfg.Versioning.Strategy)
	}
}

func currentTagGlob(cfg *config.Config, env string) (string, error) {
	prefix := func() string {
		if cfg.Versioning.TagPrefix != nil {
			return *cfg.Versioning.TagPrefix
		}
		return ""
	}

	switch cfg.Versioning.Strategy {
	case "semver":
		p := "v"
		if cfg.Versioning.TagPrefix != nil {
			p = *cfg.Versioning.TagPrefix
		}
		return p + "*", nil
	case "calver":
		return prefix() + "*", nil
	case "semver-per-env", "calver-per-env":
		if env == "" {
			return "", fmt.Errorf("--env is required for %s strategy", cfg.Versioning.Strategy)
		}
		if _, ok := cfg.Environments[env]; !ok {
			return "", fmt.Errorf("environment %q not found in config", env)
		}
		return tagfmt.GlobPattern(cfg.EffectiveTagFormat(env), tagfmt.Tokens{Env: env})
	default:
		return "", fmt.Errorf("unknown versioning strategy %q", cfg.Versioning.Strategy)
	}
}
