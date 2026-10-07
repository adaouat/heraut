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
	preRelease string
	noGuard    bool
}

// WithoutCollisionGuard skips the --set-version tag-collision probe (ADR-0065) for runs that
// never create the tag, such as re-rendering the changelog of an already-released version.
func WithoutCollisionGuard() ResolverOption {
	return func(o *resolverOptions) { o.noGuard = true }
}

// WithPreRelease asks the plain semver resolver to mint a <core>-<label>.<N> pre-release
// (--pre-release). An empty label leaves the final-release path untouched.
func WithPreRelease(label string) ResolverOption {
	return func(o *resolverOptions) { o.preRelease = label }
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
// StaticResolver is returned for all strategies, bypassing git calls to resolve it (a semver
// repo with versioning.branches still runs a tag-collision probe — see guardSetVersion).
// buildID is set when --set-build-id <id> is passed; requires versionOverride to be set.
// opts tune resolution without changing the positional signature — see WithAllowMajor.
func NewResolver(cfg *config.Config, env string, force bool, versionOverride, buildID string, runner port.Runner, opts ...ResolverOption) (versioning.Resolver, error) {
	var o resolverOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.preRelease != "" {
		if err := validatePreReleaseUsage(cfg, versionOverride, o.preRelease); err != nil {
			return nil, err
		}
	}
	if buildID != "" && versionOverride == "" {
		return nil, fmt.Errorf("--set-build-id requires --set-version: build ID cannot be combined with automatic version resolution")
	}
	if versionOverride != "" {
		if buildID != "" && cfg.Versioning.Strategy == "semver" && cfg.EffectiveTagFormat(env) == "" {
			// Plain semver has no tag_format to carry {build}: append the ID as SemVer build
			// metadata (ADR-0064).
			prefix := configuredTagPrefix(cfg)
			version := strings.TrimPrefix(versionOverride, prefix)
			if err := validateSemVerComposition(version, buildID); err != nil {
				return nil, err
			}
			return guardSetVersion(cfg, versioning.NewStaticResolver(prefix+version+"+"+buildID, version), runner, o.noGuard), nil
		}

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
		var expectedPrefixHint string
		if tf != "" {
			// Strip any leading "v" to derive the bare version component fed into the
			// {version} token — a tag_format template has no single "prefix" to strip,
			// so this heuristic (not the configured tag_prefix) applies here.
			version = strings.TrimPrefix(versionOverride, "v")
			if buildID != "" && (cfg.Versioning.Strategy == "semver" || cfg.Versioning.Strategy == "semver-per-env") {
				// {build} always follows "+" (ADR-0064), so under semver/semver-per-env the
				// build ID is SemVer build metadata — tighten tagfmt.ValidateBuildID's lenient
				// "/"-and-whitespace-only check. This also covers plain semver with a
				// tag_format; without one, --set-build-id takes the no-tag_format branch above
				// instead, which already validates composition on its own.
				// calver-per-env keeps the lenient check only: a zero-padded CalVer build ID is
				// not SemVer build metadata.
				if err := validateSemVerBuildID(buildID); err != nil {
					return nil, err
				}
				if err := validateSemVerComposition(version, buildID); err != nil {
					return nil, err
				}
			}
			var err error
			tag, err = tagfmt.Render(tf, tagfmt.Tokens{Env: env, Version: version, Build: buildID})
			if err != nil {
				return nil, fmt.Errorf("rendering tag: %w", err)
			}
		} else {
			prefix := configuredTagPrefix(cfg)
			version = strings.TrimPrefix(versionOverride, prefix)
			tag = prefix + version
			// Plain semver only: a non-default tag_prefix is easy to miss as the reason a
			// seemingly-reasonable --set-version value fails to parse (e.g. "v1.2.3" against
			// tag_prefix: "rel-" leaves "v1.2.3" un-stripped), so name it in the error.
			if cfg.Versioning.Strategy == "semver" && prefix != defaultTagPrefix(cfg.Versioning.Strategy) {
				expectedPrefixHint = prefix
			}
		}
		if buildID == "" {
			// buildID != "" already ran validateSemVerComposition (and, for semver-per-env,
			// validateSemVerBuildID) above, which parses version as part of the full
			// "<version>+<buildID>" string — this covers the two paths those don't reach:
			// plain semver and semver-per-env with no build ID at all (T336/ADR-0064).
			if err := validateSemVerStrategyOverride(cfg.Versioning.Strategy, version, expectedPrefixHint); err != nil {
				return nil, err
			}
		}
		return guardSetVersion(cfg, versioning.NewStaticResolver(tag, version), runner, o.noGuard), nil
	}

	switch cfg.Versioning.Strategy {
	case "semver":
		r := semver.New(runner, cfg)
		r.SetAllowMajor(o.allowMajor)
		r.SetPreRelease(o.preRelease)
		if err := applyMaintenanceRange(r, cfg, runner); err != nil {
			return nil, err
		}
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

// guardSetVersion adds the maintenance collision guard to a --set-version resolver when
// versioning.branches is set (ADR-0065), so a version already released elsewhere fails before
// anything is written. The probe runs inside Resolve, not here: a taken version is a runtime
// condition, and NewResolver's own errors are reported as configuration errors. Without the
// block, or with noGuard (a run that never tags), the resolver is returned unchanged and makes no
// git call.
func guardSetVersion(cfg *config.Config, r versioning.Resolver, runner port.Runner, noGuard bool) versioning.Resolver {
	if noGuard || len(cfg.Versioning.Branches) == 0 || cfg.Versioning.Strategy != "semver" {
		return r
	}
	return collisionGuardResolver{inner: r, runner: runner}
}

type collisionGuardResolver struct {
	inner  versioning.Resolver
	runner port.Runner
}

func (g collisionGuardResolver) Resolve() (versioning.Result, error) {
	res, err := g.inner.Resolve()
	if err != nil {
		return res, err
	}
	existing, err := semver.ExistingRelease(g.runner, res.Tag)
	if err != nil {
		return versioning.Result{}, err
	}
	if existing != "" {
		return versioning.Result{}, fmt.Errorf("%w: %s — pick a free version for --set-version",
			semver.ErrTagExists, existing)
	}
	return res, nil
}

// applyMaintenanceRange confines r to the current branch's maintenance line when
// versioning.branches is set (ADR-0065). Without the block it makes no git call, so repos that
// don't declare branches resolve exactly as before. Manual bump mode is skipped too: its version
// comes from --set-version, which never reaches this point.
func applyMaintenanceRange(r *semver.Resolver, cfg *config.Config, runner port.Runner) error {
	if len(cfg.Versioning.Branches) == 0 || cfg.Versioning.BumpMode() == "manual" {
		return nil
	}
	branch, known, err := CurrentBranch(runner)
	if err != nil {
		return err
	}
	m, err := MatchBranchRule(cfg, branch, known, true)
	if err != nil {
		return err
	}
	if m.Kind == BranchMaintenance {
		rg := semver.RangeFrom(m.Range, m.Branch)
		r.SetMaintenanceRange(&rg)
	}
	return nil
}

// validatePreReleaseUsage rejects --pre-release combinations that cannot mint a pre-release,
// before any other resolver branch runs so --set-version never silently wins.
func validatePreReleaseUsage(cfg *config.Config, versionOverride, label string) error {
	if versionOverride != "" {
		return fmt.Errorf("--pre-release cannot be combined with --set-version: --set-version already chooses the version (pass a pre-release value such as 1.4.0-rc.1 to it instead)")
	}
	if cfg.Versioning.Strategy != "semver" {
		return fmt.Errorf("--pre-release requires versioning.strategy: semver (got %q): pre-releases are minted for plain semver only (ADR-0064)", cfg.Versioning.Strategy)
	}
	if cfg.Versioning.BumpMode() == "manual" {
		return fmt.Errorf("--pre-release requires versioning.bump.mode: auto — manual mode has no computed version to build a pre-release on")
	}
	if err := semver.ValidatePreReleaseLabel(label); err != nil {
		return fmt.Errorf("--pre-release %q: %w", label, err)
	}
	return nil
}

// validateSemVerBuildID checks that buildID alone is valid SemVer build metadata
// (dot-separated [0-9A-Za-z-]+ identifiers), independent of the (possibly invalid)
// --set-version value — required under "semver-per-env" since {build} always follows "+" in
// a rendered tag (ADR-0064). "0.0.0" is a placeholder core purely to drive semver.Parse.
func validateSemVerBuildID(buildID string) error {
	if _, err := semver.Parse("0.0.0+" + buildID); err != nil {
		return fmt.Errorf(
			"--set-build-id %q is not valid SemVer build metadata: must be dot-separated "+
				"[0-9A-Za-z-] identifiers: %w",
			buildID, err,
		)
	}
	return nil
}

// validateSemVerComposition checks that "<version>+<buildID>" itself parses as a SemVer
// version, catching a malformed --set-version (e.g. "1.4") that validateSemVerBuildID alone
// cannot see. Shared by plain semver (ADR-0064) and semver-per-env (T333) — kept as one check
// so both strategies report the same error shape.
func validateSemVerComposition(version, buildID string) error {
	if _, err := semver.Parse(version + "+" + buildID); err != nil {
		return fmt.Errorf(
			"--set-version %q with --set-build-id %q does not form a valid SemVer version: "+
				"--set-version must be MAJOR.MINOR.PATCH[-pre] and the build ID must be "+
				"dot-separated [0-9A-Za-z-] identifiers (SemVer build metadata): %w",
			version, buildID, err,
		)
	}
	return nil
}

// validateSemVerStrategyOverride enforces SemVer v2 syntax on a --set-version value under the
// semver and semver-per-env strategies (T336/ADR-0064 Phase 1.5): build metadata has exactly one
// entry point (--set-build-id), so a value carrying "+" is rejected with a hint toward it instead
// of being parsed; anything else must parse as MAJOR.MINOR.PATCH[-pre-release]. version is already
// stripped of its tag prefix / tag_format wrapping by the caller. CalVer strategies are untouched
// (returns nil) — a CalVer value like "2026.05.0" has a leading zero and is not valid SemVer.
// expectedPrefixHint, when non-empty (the plain-semver, no-tag_format path with a configured
// tag_prefix other than the "v" default), is named in the parse-failure error so a value that
// forgot the configured prefix doesn't just look like bad SemVer syntax.
func validateSemVerStrategyOverride(strategy, version, expectedPrefixHint string) error {
	if strategy != "semver" && strategy != "semver-per-env" {
		return nil
	}
	if strings.Contains(version, "+") {
		return fmt.Errorf("--set-version %q must not carry build metadata: pass it with --set-build-id instead", version)
	}
	if _, err := semver.Parse(version); err != nil {
		if expectedPrefixHint != "" {
			return fmt.Errorf("--set-version %q is not a valid SemVer version (expected MAJOR.MINOR.PATCH[-pre-release]) (expected an optional %q prefix): %w", version, expectedPrefixHint, err)
		}
		return fmt.Errorf("--set-version %q is not a valid SemVer version (expected MAJOR.MINOR.PATCH[-pre-release]): %w", version, err)
	}
	return nil
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

// configuredTagPrefix is versioning.tag_prefix when set, else the strategy's default.
func configuredTagPrefix(cfg *config.Config) string {
	if cfg.Versioning.TagPrefix != nil {
		return *cfg.Versioning.TagPrefix
	}
	return defaultTagPrefix(cfg.Versioning.Strategy)
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
