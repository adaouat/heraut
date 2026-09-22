package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/calver"
	"github.com/adaouat/heraut/internal/versioning/perenv"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

// ResolverOption tunes NewResolver without changing its positional signature.
type ResolverOption func(*resolverOptions)

type resolverOptions struct {
	allowMajor bool
}

// WithAllowMajor lifts versioning.bump.stay_at_v0's hold-back for this resolution (--allow-major).
func WithAllowMajor(allow bool) ResolverOption {
	return func(o *resolverOptions) { o.allowMajor = allow }
}

// warningResolver copies the warnings a semver calculator recorded during Resolve into
// Result.Warnings, rewriting each warning's bare "would-be"/"held" version tokens into the real
// tag shape once the full Result (with its rendered Tag) is known — hold.go and the semver
// resolver never see tag_format for per-env strategies, so this is the earliest point that can.
// It exists so semver-per-env's warnings can cross perenv without widening
// perenv.VersionCalculator, whose BumpAuto returns only (string, error). The semver resolver
// resets its recorded warnings on entry to both Resolve and BumpAuto, so reading them after
// Resolve can never return a previous run's text.
type warningResolver struct {
	inner           versioning.Resolver
	warnings        func() []string
	wouldBeVersions func() []string
}

func (w warningResolver) Resolve() (versioning.Result, error) {
	res, err := w.inner.Resolve()
	if err != nil {
		return res, err
	}
	warnings := w.warnings()
	wouldBes := w.wouldBeVersions()
	rewritten := make([]string, len(warnings))
	for i, warn := range warnings {
		if i < len(wouldBes) && wouldBes[i] != "" {
			warn = rewriteHeldTags(warn, wouldBes[i], res.Version, res.Tag)
		}
		rewritten[i] = warn
	}
	res.Warnings = append(res.Warnings, rewritten...)
	return res, nil
}

// rewriteHeldTags replaces the bare wouldBeVersion and version (the resolved "held" bare version)
// tokens in warning's headline — its first line — with their real tag shape, derived from where
// version appears inside tag. Both replacements run in one pass via strings.NewReplacer so
// neither replacement's output can be re-matched by the other. Only the headline is rewritten —
// any commit-subject lines below it are left untouched, so a commit message that happens to
// contain the bare version string is never corrupted. version is expected to be a literal
// substring of tag (true by construction: tagfmt substitutes {version} verbatim into the tag it
// renders) — if it is not found, warning is returned unchanged rather than guessing. When the
// bare held version occurs more than once inside the rendered tag — an env name or a tag_format
// literal that happens to contain it, or a tag_format that repeats {version} — strings.Index
// takes the first occurrence, so the would-be side of the headline is approximate; the resolved
// tag itself is never affected, only this advisory text.
func rewriteHeldTags(warning, wouldBeVersion, version, tag string) string {
	idx := strings.Index(tag, version)
	if version == "" || wouldBeVersion == "" || idx < 0 {
		return warning
	}
	prefix, suffix := tag[:idx], tag[idx+len(version):]
	headline, rest, hasRest := strings.Cut(warning, "\n")
	headline = strings.NewReplacer(
		version, tag,
		wouldBeVersion, prefix+wouldBeVersion+suffix,
	).Replace(headline)
	if hasRest {
		return headline + "\n" + rest
	}
	return headline
}

// NewResolver builds the appropriate versioning.Resolver from config.
// env is the active environment name (empty for non-per-env strategies).
// force is the --force flag value.
// versionOverride is set when --set-version X.Y.Z is passed; when non-empty a
// StaticResolver is returned for all strategies, bypassing git calls entirely.
// buildID is set when --set-build-id <id> is passed; requires versionOverride to be set.
// opts tune resolution without changing the positional signature — see WithAllowMajor.
func NewResolver(cfg *config.Config, env string, force bool, versionOverride, buildID string, runner port.Runner, opts ...ResolverOption) (versioning.Resolver, error) {
	if buildID != "" && versionOverride == "" {
		return nil, fmt.Errorf("--set-build-id requires --set-version: build ID cannot be combined with automatic version resolution")
	}
	if versionOverride != "" {
		var tf string
		if buildID != "" {
			var err error
			tf, err = effectiveTagFmt(cfg, env)
			if err != nil {
				return nil, err
			}
		} else {
			tf = cfg.EffectiveTagFormat(env)
		}

		var tag, version string
		if tf != "" {
			// Strip any leading "v" to derive the bare version component fed into the
			// {version} token — a tag_format template has no single "prefix" to strip,
			// so this heuristic (not the configured tag_prefix) applies here.
			version = strings.TrimPrefix(versionOverride, "v")
			var err error
			tag, err = tagfmt.Render(tf, tagfmt.Tokens{Env: env, Version: version, Build: buildID})
			if err != nil {
				return nil, fmt.Errorf("rendering tag: %w", err)
			}
		} else {
			prefix := defaultTagPrefix(cfg.Versioning.Strategy)
			if cfg.Versioning.TagPrefix != nil {
				prefix = *cfg.Versioning.TagPrefix
			}
			version = strings.TrimPrefix(versionOverride, prefix)
			tag = prefix + version
		}
		return versioning.NewStaticResolver(tag, version), nil
	}

	var o resolverOptions
	for _, opt := range opts {
		opt(&o)
	}

	switch cfg.Versioning.Strategy {
	case "semver":
		r := semver.New(runner, cfg)
		r.SetAllowMajor(o.allowMajor)
		return warningResolver{inner: r, warnings: r.Warnings, wouldBeVersions: r.WouldBeVersions}, nil
	case "calver":
		return calver.New(runner, cfg, time.Now), nil
	case "semver-per-env":
		calc := semver.New(nil, cfg)
		calc.SetAllowMajor(o.allowMajor)
		return warningResolver{inner: perenv.New(runner, cfg, env, force, calc), warnings: calc.Warnings, wouldBeVersions: calc.WouldBeVersions}, nil
	case "calver-per-env":
		calc := calver.New(nil, cfg, time.Now)
		return perenv.New(runner, cfg, env, force, calc), nil
	default:
		return nil, fmt.Errorf("unknown versioning strategy %q (supported: semver, calver, semver-per-env, calver-per-env)", cfg.Versioning.Strategy)
	}
}

// ValidateBuildID reports whether a --set-build-id value is usable as a tag component.
// Delegates to tagfmt so cmd does not import the versioning layer directly.
func ValidateBuildID(build string) error {
	return tagfmt.ValidateBuildID(build)
}

// ValidateVersionOverride reports whether a --set-version value is usable as a
// tag/version override. Delegates to tagfmt so cmd does not import the
// versioning layer directly.
func ValidateVersionOverride(version string) error {
	return tagfmt.ValidateVersionOverride(version)
}

// defaultTagPrefix mirrors the unexported default each strategy resolver falls back to when
// versioning.tag_prefix is unset (semver.Resolver.prefix, calver.Resolver.prefix): "v" for
// SemVer-based strategies, "" for CalVer-based ones, where a version like 2026.05.3 already
// reads as a tag without help from a prefix.
func defaultTagPrefix(strategy string) string {
	switch strategy {
	case "semver", "semver-per-env":
		return "v"
	default:
		return ""
	}
}

// effectiveTagFmt returns the tag format to use for build ID rendering and
// validates that {build} is present (required when --set-build-id is passed). The
// env-override → top-level resolution lives in config.EffectiveTagFormat.
func effectiveTagFmt(cfg *config.Config, env string) (string, error) {
	tf := cfg.EffectiveTagFormat(env)
	if tf == "" {
		return "", fmt.Errorf("--set-build-id requires versioning.tag_format to contain a {build} token, but tag_format is not set")
	}
	if !strings.Contains(tf, "{build}") {
		return "", fmt.Errorf("--set-build-id requires a {build} token in versioning.tag_format (got %q)", tf)
	}
	return tf, nil
}
