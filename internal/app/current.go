package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning/perenv"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

// errNoTagsFound is the sentinel CurrentTag returns when no tags match the resolved glob.
var errNoTagsFound = errors.New("no tags found")

// CurrentTag returns the latest existing git tag for the given strategy and environment. For
// SemVer strategies it is the highest SemVer §11 release, or — with includePreRelease — the
// highest tag including pre-releases (ADR-0064). CalVer strategies ignore includePreRelease and
// keep git's version:refname order. For single-env strategies, env is ignored; for per-env
// strategies, env is required. Under semver with versioning.branches, a maintenance branch reports
// its own line's tag (currentMaintenanceTag); without the block no branch detection runs.
func CurrentTag(runner port.Runner, cfg *config.Config, env string, includePreRelease bool) (string, error) {
	glob, err := currentTagGlob(cfg, env)
	if err != nil {
		return "", err
	}

	if cfg.Versioning.Strategy == "semver" && len(cfg.Versioning.Branches) > 0 {
		branch, known, err := CurrentBranch(runner)
		if err != nil {
			return "", err
		}
		m, err := MatchBranchRule(cfg, branch, known, true)
		if err != nil {
			return "", err
		}
		if m.Kind == BranchMaintenance {
			rg := semver.RangeFrom(m.Range, m.Branch)
			return currentMaintenanceTag(runner, cfg, glob, rg, includePreRelease)
		}
	}
	return globalCurrentTag(runner, cfg, env, glob, includePreRelease)
}

// latestGlobalTag is globalCurrentTag including pre-releases, for callers that bypass branch
// matching.
func latestGlobalTag(runner port.Runner, cfg *config.Config, env string) (string, error) {
	glob, err := currentTagGlob(cfg, env)
	if err != nil {
		return "", err
	}
	return globalCurrentTag(runner, cfg, env, glob, true)
}

// globalCurrentTag is CurrentTag without branch awareness: the latest tag across the whole repo,
// exactly what CurrentTag returns when versioning.branches is absent.
func globalCurrentTag(runner port.Runner, cfg *config.Config, env, glob string, includePreRelease bool) (string, error) {
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

// currentMaintenanceTag is CurrentTag on a maintenance line (ADR-0065): the highest tag reachable
// from HEAD whose core lies in rg — the same base the maintenance resolver bumps from — never a
// higher tag cut on another line.
func currentMaintenanceTag(runner port.Runner, cfg *config.Config, glob string, rg semver.Range, includePreRelease bool) (string, error) {
	stdout, _, err := runner.Run("git", "tag", "-l", glob, "--merged", "HEAD", "--sort=-version:refname")
	if err != nil {
		return "", fmt.Errorf("listing git tags merged into HEAD: %w", err)
	}
	var inRange []semver.TagVersion
	for _, tv := range semver.SortTags(strings.Fields(stdout), semverExtractor(cfg, "")) {
		if rg.Contains(tv.Version) {
			inRange = append(inRange, tv)
		}
	}
	if tv, ok := semver.Latest(inRange, includePreRelease); ok {
		return tv.Tag, nil
	}
	if len(inRange) > 0 {
		return "", fmt.Errorf("%w in range %s reachable from %s: only pre-release tags exist (pass --include-pre-release to show them)", errNoTagsFound, rg.Label, rg.Branch)
	}
	return "", fmt.Errorf("%w in range %s reachable from %s", errNoTagsFound, rg.Label, rg.Branch)
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

// tagOrderFor returns the tag-ordering function native.WithTagOrder needs to bound changelog
// sections, release-notes previous-tag resolution, and changelog rotation by SemVer §11 precedence
// instead of git's version:refname order (T334, ADR-0064). For "semver"/"semver-per-env" it
// re-sorts whatever tag list it's given (already scoped by TagGlob/TagPattern) through
// semver.SortTags using the same extractor CurrentTag uses, then drops every pre-release tag —
// releases only, so a pre-release never gets its own changelog section and is never a range
// boundary; its commits fold into the next release's section. calver/calver-per-env return nil:
// their output must stay byte-for-byte unchanged, still walking git's own order.
func tagOrderFor(cfg *config.Config, env string) func([]string) []string {
	switch cfg.Versioning.Strategy {
	case "semver", "semver-per-env":
		extract := semverExtractor(cfg, env)
		return func(tags []string) []string {
			sorted := semver.SortTags(tags, extract)
			out := make([]string, 0, len(sorted))
			for _, tv := range sorted {
				if !tv.Version.IsPreRelease() {
					out = append(out, tv.Tag)
				}
			}
			return out
		}
	default:
		return nil
	}
}

// notesTagOrderFor is tagOrderFor for a pre-release run's release notes: same §11 sort, but
// pre-release tags are kept — a pre-release's notes span back to the previous tag of any kind.
func notesTagOrderFor(cfg *config.Config, env string) func([]string) []string {
	switch cfg.Versioning.Strategy {
	case "semver", "semver-per-env":
		extract := semverExtractor(cfg, env)
		return func(tags []string) []string {
			sorted := semver.SortTags(tags, extract)
			out := make([]string, 0, len(sorted))
			for _, tv := range sorted {
				out = append(out, tv.Tag)
			}
			return out
		}
	default:
		return nil
	}
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
		bare := strings.TrimPrefix(tag, configuredTagPrefix(cfg))
		v, err := semver.Parse(bare)
		if err != nil {
			// CurrentTag only ever returns tags that already parsed as SemVer for this
			// strategy, so this should be unreachable — fall back to the prefix-stripped
			// string rather than erroring on something that should never happen.
			return bare, nil
		}
		v.Build = nil
		return v.String(), nil
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
			return "", fmt.Errorf("--env %w for %s strategy", perenv.ErrEnvRequired, cfg.Versioning.Strategy)
		}
		if _, ok := cfg.Environments[env]; !ok {
			return "", fmt.Errorf("environment %q %w", env, perenv.ErrEnvNotFound)
		}
		return tagfmt.GlobPattern(cfg.EffectiveTagFormat(env), tagfmt.Tokens{Env: env})
	default:
		return "", fmt.Errorf("unknown versioning strategy %q", cfg.Versioning.Strategy)
	}
}
