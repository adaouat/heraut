package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning/calver"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

// rotatingGenerator wraps a port.Generator so a rotating changelog.output pattern (e.g.
// "CHANGELOG_{YYYY}.md") resolves to a concrete path — and a matching tag-scoping pattern — from
// the pipeline's actual resolved tag, not at buildChangelogPipelineConfig/buildReleasePipelineConfig
// time. A manual --version override bypasses the resolver's own date/bump computation, so only the
// tag Generate() actually receives is guaranteed to match what gets written; native itself must not
// perform this substitution either, since native may only import internal/{port,config,
// conventionalcommit} (.claude/rules/coding.md), never internal/versioning. See
// docs/superpowers/specs/2026-08-28-changelog-rotation-design.md §3.
type rotatingGenerator struct {
	fallback port.Generator // delegate for Check/Validate — both are no-ops on native regardless

	runner              port.Runner
	cfg                 *config.Config
	driver              *config.ContentDriver // raw driver; Output/TagPattern may hold {TOKEN} placeholders
	herautVersion       string
	regenerateChangelog bool
	force               bool
	enrichForge         port.Forge
	degradedReason      string
	tokens              []string // {TOKEN} names found in driver.Output, e.g. ["YYYY"]

	lastOutputPath string
}

// wrapWithRotation returns gen unchanged when driver.Output has no {TOKEN} placeholders — the
// common case, zero overhead for every existing config. Otherwise it returns a rotatingGenerator
// that computes the concrete Output/TagPattern from each Generate call's resolved tag.
func wrapWithRotation(
	gen port.Generator,
	runner port.Runner,
	cfg *config.Config,
	driver *config.ContentDriver,
	herautVersion string,
	regenerateChangelog, force bool,
	enrichForge port.Forge,
	degradedReason string,
) port.Generator {
	tokens := config.ExtractRotationTokens(driver.Output)
	if len(tokens) == 0 {
		return gen
	}
	return &rotatingGenerator{
		fallback:            gen,
		runner:              runner,
		cfg:                 cfg,
		driver:              driver,
		herautVersion:       herautVersion,
		regenerateChangelog: regenerateChangelog,
		force:               force,
		enrichForge:         enrichForge,
		degradedReason:      degradedReason,
		tokens:              tokens,
	}
}

func (r *rotatingGenerator) Check() error    { return r.fallback.Check() }
func (r *rotatingGenerator) Validate() error { return r.fallback.Validate() }

// LastOutputPath returns the concrete path the most recent Generate call resolved to, or "" before
// the first call. internal/pipeline type-asserts for this (structurally — it never imports
// internal/app) so the commit/summary steps target the real file instead of the raw pattern.
func (r *rotatingGenerator) LastOutputPath() string { return r.lastOutputPath }

func (r *rotatingGenerator) Generate(tag string, lc *port.LinkContext) (string, error) {
	concrete, err := r.resolveDriver(tag)
	if err != nil {
		return "", fmt.Errorf("resolving rotated changelog output for %q: %w", tag, err)
	}
	r.lastOutputPath = concrete.Output
	gen := buildGenerator(r.runner, concrete, native.ModeChangelog, r.herautVersion, r.regenerateChangelog, r.force, r.enrichForge, r.degradedReason)
	return gen.Generate(tag, lc)
}

// resolveDriver substitutes {TOKEN} placeholders in r.driver.Output with the values the resolved
// tag actually carries, and derives a matching tag-scoping pattern — applied only when the user
// hasn't already set an explicit tag_pattern (same precedence withEnvDerivations uses for per-env
// scoping).
func (r *rotatingGenerator) resolveDriver(tag string) (*config.ContentDriver, error) {
	prefix := defaultTagPrefix(r.cfg.Versioning.Strategy)
	if r.cfg.Versioning.TagPrefix != nil {
		prefix = *r.cfg.Versioning.TagPrefix
	}
	bare := strings.TrimPrefix(tag, prefix)

	var output, tagPattern string
	var err error
	switch r.cfg.Versioning.Strategy {
	case "calver":
		output, tagPattern, err = r.resolveCalver(bare)
	case "semver":
		output, tagPattern, err = r.resolveSemver(bare)
	default:
		err = fmt.Errorf("rotation tokens require a semver or calver strategy (current: %q)", r.cfg.Versioning.Strategy)
	}
	if err != nil {
		return nil, err
	}

	clone := *r.driver
	clone.Output = output
	if clone.TagPattern == "" {
		clone.TagPattern = tagPattern
	}
	return &clone, nil
}

func (r *rotatingGenerator) resolveCalver(bareVersion string) (output, tagPattern string, err error) {
	tokens, err := calver.ParseFormat(r.cfg.Versioning.Format)
	if err != nil {
		return "", "", fmt.Errorf("invalid calver format %q: %w", r.cfg.Versioning.Format, err)
	}

	kinds := make([]calver.TokenKind, len(r.tokens))
	for i, name := range r.tokens {
		kind, ok := calver.TokenKindFromName(name)
		if !ok {
			return "", "", fmt.Errorf("rotation token {%s} is not a valid calver token", name)
		}
		kinds[i] = kind
	}

	values, err := calver.ParseVersion(tokens, bareVersion)
	if err != nil {
		return "", "", fmt.Errorf("parsing resolved version %q against calver format %q: %w", bareVersion, r.cfg.Versioning.Format, err)
	}

	pattern, err := calver.BucketPattern(tokens, kinds, values)
	if err != nil {
		return "", "", err
	}

	subs := make(map[string]string, len(r.tokens))
	for i, name := range r.tokens {
		subs[name] = calver.RenderToken(kinds[i], values)
	}
	return substituteTokens(r.driver.Output, subs), pattern, nil
}

func (r *rotatingGenerator) resolveSemver(bareVersion string) (output, tagPattern string, err error) {
	major, minor, err := semver.MajorMinor(bareVersion)
	if err != nil {
		return "", "", fmt.Errorf("parsing resolved version %q: %w", bareVersion, err)
	}

	pattern, err := semver.RotationPattern(r.tokens, major, minor)
	if err != nil {
		return "", "", err
	}

	subs := map[string]string{"MAJOR": strconv.Itoa(major), "MINOR": strconv.Itoa(minor)}
	return substituteTokens(r.driver.Output, subs), pattern, nil
}

// substituteTokens replaces each "{NAME}" placeholder present in pattern with its value. A value
// for a token not actually referenced in pattern is simply never looked up.
func substituteTokens(pattern string, values map[string]string) string {
	oldnew := make([]string, 0, len(values)*2)
	for tok, val := range values {
		oldnew = append(oldnew, "{"+tok+"}", val)
	}
	return strings.NewReplacer(oldnew...).Replace(pattern)
}
