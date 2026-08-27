package app

import (
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"strings"

	forgeui "github.com/adaouat/forge/ui"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	azureforge "github.com/adaouat/heraut/internal/forge/azure"
	githubforge "github.com/adaouat/heraut/internal/forge/github"
	gitlabforge "github.com/adaouat/heraut/internal/forge/gitlab"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

// PipelineOpts carries runtime options for building a pipeline.
type PipelineOpts struct {
	DryRun          bool
	Force           bool
	VersionOverride string
	Env             string
	Out             io.Writer
	// Commit and Tag are used by the changelog pipeline only.
	Commit bool
	Tag    bool
	// NoPush is used by the changelog pipeline only: when true, the commit and tag
	// are created locally but not pushed.
	NoPush bool
	// SignTags mirrors git config tag.gpgSign — when true the pipeline creates
	// signed tags (-s) instead of annotated ones. Set by the caller via ReadGPGSign.
	SignTags bool
	// HerautVersion is the running heraut binary's version, propagated to the native generator
	// so templates can render .Heraut.Version. Empty for dev builds.
	HerautVersion string
	// ReadRunner performs read-only operations (currently: forge git-origin detection) with a
	// real, non-dry-run runner so `--dry-run` never changes what is resolved — only what is
	// executed. Mirrors the readRunner already threaded into ResolveEnv/NewResolver/ReadGPGSign
	// in internal/cmd. Falls back to the pipeline runner passed to BuildPipeline /
	// BuildChangelogPipeline when nil (that runner is a real runner outside dry-run anyway).
	ReadRunner port.Runner
	// RegenerateChangelog forces the native changelog generator to rebuild + re-enrich the whole
	// file instead of incrementally splicing the new section. Native only.
	RegenerateChangelog bool
	// Logger receives operator-debug diagnostics (nil discards them). See forge ADR-0011.
	Logger *slog.Logger
}

// ReadGPGSign reads tag.gpgSign from git config and returns true when it is set to "true".
// Any error (key absent, git not available) is treated as false — non-signing is the safe default.
// Callers should pass a non-dry-run runner so the config is always read regardless of --dry-run.
func ReadGPGSign(runner port.Runner) bool {
	stdout, _, err := runner.Run("git", "config", "--get", "tag.gpgSign")
	if err != nil {
		return false
	}
	return strings.TrimSpace(stdout) == "true"
}

// BuildPipeline constructs a release Pipeline from config. All generator and platform
// instances are created here — none are created in internal/cmd/.
func BuildPipeline(runner port.Runner, cfg *config.Config, resolver versioning.Resolver, opts PipelineOpts) (*pipeline.Pipeline, error) {
	readRunner := opts.ReadRunner
	if readRunner == nil {
		readRunner = runner
	}
	pipelineCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, opts.Env, opts.HerautVersion, opts.RegenerateChangelog, opts.Force)
	if err != nil {
		return nil, err
	}
	pipelineCfg.SignTags = opts.SignTags

	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	pipe := pipeline.New(runner, resolver, pipelineCfg, out, opts.DryRun)
	pipe = pipe.WithReporter(spinnerReporter(out, releaseStepTotal(pipelineCfg)))
	pipe = pipe.WithLogger(opts.Logger)
	return pipe, nil
}

// spinnerReporter adapts a forge ui.Spinner to the pipeline's ui.StepFn: each
// step animates a spinner (human mode on a TTY) and resolves to a ✓/✗ line with
// an [N/total] counter. The pipeline always passes a non-nil fn.
func spinnerReporter(out io.Writer, total int) ui.StepFn {
	sp := forgeui.NewSpinner(out, forgeui.Human).Total(total)
	return func(name string, fn func() (result string, subs []string, err error)) error {
		return sp.Run(name, func() (forgeui.Result, error) {
			result, subs, err := fn()
			return forgeui.Result{Detail: result, Subs: subs}, err
		})
	}
}

// releaseStepTotal computes the number of numbered steps for a release pipeline.
// Asset uploads are sub-results of the platform step, not separate numbered steps.
func releaseStepTotal(cfg *pipeline.Config) int {
	total := 3 // resolve version + create tag + push tag
	if cfg.Changelog != nil && !cfg.DisableChangelog {
		total += 2 // generate changelog + commit changelog
	}
	// The standalone "generate release notes" step exists only for single-platform
	// releases. With multiple platforms, notes are regenerated inside each publish step
	// (folded as a sub-result, not a numbered step) — see ADR-0021.
	if cfg.Notes != nil && !cfg.DisableNotes && len(cfg.Platforms) <= 1 {
		total++ // generate release notes
	}
	total += len(cfg.Platforms) // one numbered step per platform
	return total
}

// BuildChangelogPipeline constructs a ChangelogPipeline from config.
func BuildChangelogPipeline(runner port.Runner, cfg *config.Config, resolver versioning.Resolver, opts PipelineOpts) (*pipeline.ChangelogPipeline, error) {
	readRunner := opts.ReadRunner
	if readRunner == nil {
		readRunner = runner
	}
	changelogCfg, err := buildChangelogPipelineConfig(runner, readRunner, cfg, opts)
	if err != nil {
		return nil, err
	}
	changelogCfg.SignTags = opts.SignTags

	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	pipe := pipeline.NewChangelog(runner, resolver, changelogCfg, out, opts.DryRun)
	pipe = pipe.WithReporter(spinnerReporter(out, changelogStepTotal(changelogCfg)))
	return pipe, nil
}

// changelogStepTotal computes the number of numbered steps for a changelog pipeline.
func changelogStepTotal(cfg *pipeline.ChangelogConfig) int {
	total := 1 // resolve version
	if cfg.Changelog != nil && !cfg.DisableChangelog {
		total++ // generate changelog
		if cfg.Commit || cfg.Tag {
			total++ // commit changelog
		}
	}
	if cfg.Tag {
		total++ // create tag
		if !cfg.NoPush {
			total++ // push tags
		}
	}
	return total
}

// HasResolvablePublishTarget reports whether cfg has at least one publish destination for env: a
// non-empty effective release.targets list, or — when that list is empty — a forge that resolves
// to a type with a publish driver, from cfg/CI env/git origin (the same zero-config synthesis
// buildTargetPlatforms performs, via synthesizeDefaultTarget — T221 taught both to treat a
// driver-less resolved forge, e.g. azure_devops, as equivalent to no forge resolved at all, so this
// preflight and the pipeline's own behavior can never disagree). This is heraut release's pre-flight
// gate: with release.platforms gone, "no entry in release.targets and nothing auto-detects to a
// publishable forge" is the only shape left that must hard-fail before the pipeline runs (see
// buildTargetPlatforms — resolving zero forges is not itself an error, since a changelog-only flow
// legitimately needs no publish target at all).
//
// An *explicit* release.targets entry naming a driver-less forge is deliberately not caught here —
// that is a user-authored reference, and buildPlatform's own "unsupported platform" error is already
// specific and actionable when the pipeline actually tries to build it (T221).
//
// A forge-resolution error here (e.g. an ambiguous multi-token machine) is deliberately treated as
// "not resolvable" rather than propagated: BuildPipeline performs the same resolution again and
// surfaces that error with full context, so this pre-flight only needs a yes/no answer.
func HasResolvablePublishTarget(runner port.Runner, cfg *config.Config, env string) bool {
	if cfg == nil {
		return false
	}
	if len(config.EffectiveTargets(cfg, env)) > 0 {
		return true
	}
	resolved, err := resolveForge(runner, os.Getenv, cfg)
	if err != nil {
		return false
	}
	return len(synthesizeDefaultTarget(resolved)) > 0
}

func buildReleasePipelineConfig(runner, readRunner port.Runner, cfg *config.Config, env, herautVersion string, regenerateChangelog, force bool) (*pipeline.Config, error) {
	pCfg := &pipeline.Config{}

	// Resolve effective config: start from root, apply per-env overrides.
	effectiveChangelog := cfg.Changelog
	// EffectiveReleaseNotes owns the root/per-env merge and ADR-0046 default-populate together
	// (T223) — duplicating that logic here previously missed the per-env-only case, since the
	// loader's normalize() only default-populates the top-level Release.Notes.
	effectiveNotes := config.EffectiveReleaseNotes(cfg, env)
	var releaseAssets []string
	if cfg.Release != nil {
		releaseAssets = cfg.Release.Assets
	}
	effectiveTargets := config.EffectiveTargets(cfg, env)

	if env != "" {
		if envCfg, ok := cfg.Environments[env]; ok {
			pCfg.DisableChangelog = envCfg.DisableChangelog
			// disable_release turns off the entire release: behavior for this environment (T217)
			// — pCfg.DisableNotes gates notes generation at runtime (internal/pipeline/release.go)
			// exactly like disable_notes used to, and is also used below to skip building publish
			// targets entirely, since there is no longer a way to disable just one half.
			pCfg.DisableNotes = envCfg.DisableRelease
			if envCfg.Changelog != nil {
				effectiveChangelog = config.MergeContentDriver(effectiveChangelog, envCfg.Changelog)
			}
		}
	}

	// Enrichment and publishing share one forge resolution — a second forge.Resolve call would add
	// a duplicate `git remote get-url origin` invocation (and could break MockRunner's FIFO
	// response ordering in tests). This call is unconditional, not gated on --offline or on
	// whether a native generator is in play: with release.platforms gone, release.targets is the
	// only publish surface — an explicit list, or, when empty, the zero-config synthesis path in
	// buildTargetPlatforms — so this pipeline (used only by `heraut release`, which requires at
	// least one resolvable publish destination) always needs forge resolution for publishing,
	// regardless of what enrichment needs. A prior version of this comment described a `needsForge`
	// guard combining an enrichment disjunct with the two publish-target-length disjuncts
	// (len(targets) > 0 || len(targets) == 0) — that guard was a tautology (always true) and was
	// removed (T173); resolveEnrichForgeIfNeeded (used by the changelog-only pipeline, not this
	// one) is what actually gates resolution on --offline/enrichment policy.
	resolved, err := resolveForge(readRunner, os.Getenv, cfg)
	if err != nil {
		return nil, err
	}

	var enrichForge port.Forge
	var forgeID *port.ForgeIdentity
	if cfg.EnrichmentPolicy() != "disabled" {
		enrichForge, forgeID = enrichForgeFrom(resolved)
	}

	// Changelog generator
	if effectiveChangelog != nil {
		driver := withEnvDerivations(effectiveChangelog, cfg, env)
		gen := buildGenerator(runner, driver, native.ModeChangelog, herautVersion, regenerateChangelog, force, enrichForge, "")
		pCfg.Changelog = gen
		pCfg.ChangelogFile = effectiveChangelog.Output
		pCfg.ForgeIdentity = forgeID
	}

	// Release notes generator
	if effectiveNotes != nil {
		driver := withEnvDerivations(effectiveNotes, cfg, env)
		gen := buildGenerator(runner, driver, native.ModeReleaseNotes, herautVersion, regenerateChangelog, force, enrichForge, "")
		pCfg.Notes = gen
	}

	// Targets (release.targets — ADR-0043/ADR-0044, the only publish surface). Skipped entirely
	// when disable_release is set for this environment (pCfg.DisableNotes) — the whole release:
	// behavior is off, so no target is built even when release.targets is explicit (T217).
	var targetPlatforms []port.Platform
	if !pCfg.DisableNotes {
		targetPlatforms, err = buildTargetPlatforms(runner, cfg, effectiveTargets, releaseAssets, resolved)
		if err != nil {
			return nil, err
		}
	}
	pCfg.Platforms = append(pCfg.Platforms, targetPlatforms...)

	pCfg.AnnotatedTags = cfg.Versioning.TagType != "lightweight"
	pCfg.RegenerateChangelog = regenerateChangelog

	return pCfg, nil
}

// buildTargetPlatforms builds one port.Platform per effective release.targets entry (ADR-0043),
// resolving each target's forge reference (or the sole/enrichment forge when a target — or the
// whole list — leaves it implicit) against resolved. When targets is empty but resolved found at
// least one forge, a single default target is synthesized for the enrichment/sole forge so
// zero-config repos (no forges:, no release.targets) still publish (T216: release: presence
// always means "publish" — there is no longer a "notes only" shape to protect against).
// release.assets propagates to targets that declare none. Every target's resolved asset list —
// whether inherited from release.assets or declared on the target itself — resolves leniently
// (T228): a non-matching glob warns and is skipped rather than aborting the release, since by the
// time asset globs are resolved the tag has already been created and pushed, so a strict failure
// here would leave the repository in a partially-completed state (tag exists, no platform release).
func buildTargetPlatforms(runner port.Runner, cfg *config.Config, targets []config.Target, releaseAssets []string, resolved forge.Resolved) ([]port.Platform, error) {
	if len(targets) == 0 {
		targets = synthesizeDefaultTarget(resolved)
	}

	var platforms []port.Platform
	for i, t := range targets {
		f, id, err := resolveTargetForge(cfg, t, resolved)
		if err != nil {
			return nil, fmt.Errorf("release.targets[%d]: %w", i, err)
		}
		platCfg := platformConfigFromTarget(t, f, id)
		if len(platCfg.Assets) == 0 && len(releaseAssets) > 0 {
			platCfg.Assets = releaseAssets
		}
		platCfg.LenientAssets = len(platCfg.Assets) > 0
		p, err := buildPlatform(runner, &platCfg)
		if err != nil {
			return nil, fmt.Errorf("release.targets[%d] (%s): %w", i, platCfg.Type, err)
		}
		platforms = append(platforms, p)
	}
	return platforms, nil
}

// resolveTargetForge finds the config.Forge and resolved port.ForgeIdentity a target refers to:
// by name (t.Forge) when set, or the sole configured forge when cfg.Forges has exactly one entry,
// or the enrichment/sole resolved identity for zero-config repos (cfg.Forges empty). Config
// validation (validateForges) rejects an empty t.Forge with more than one forge configured for
// both release.targets and every environments.<env>.release.targets — but the default: branch
// below re-checks the same ambiguity at runtime rather than assuming validation ran: BuildPipeline
// is reachable from paths that do not call config.Validate first (e.g. programmatic callers,
// tests constructing a *config.Config directly), so this is defense in depth, not dead code.
func resolveTargetForge(cfg *config.Config, t config.Target, resolved forge.Resolved) (config.Forge, port.ForgeIdentity, error) {
	if len(cfg.Forges) == 0 {
		if len(resolved.Forges) == 0 {
			return config.Forge{}, port.ForgeIdentity{}, fmt.Errorf("no forge resolved for target")
		}
		return config.Forge{}, resolved.Forges[resolved.EnrichmentIndex], nil
	}

	idx := -1
	switch {
	case t.Forge != "":
		for i, f := range cfg.Forges {
			if f.Name == t.Forge {
				idx = i
				break
			}
		}
		if idx == -1 {
			return config.Forge{}, port.ForgeIdentity{}, fmt.Errorf("unknown forge %q", t.Forge)
		}
	case len(cfg.Forges) == 1:
		idx = 0
	default:
		return config.Forge{}, port.ForgeIdentity{}, fmt.Errorf("forge is required when more than one forge is configured")
	}

	if idx >= len(resolved.Forges) {
		return config.Forge{}, port.ForgeIdentity{}, fmt.Errorf("forge %q did not resolve", cfg.Forges[idx].Name)
	}
	return cfg.Forges[idx], resolved.Forges[idx], nil
}

func buildChangelogPipelineConfig(runner, readRunner port.Runner, cfg *config.Config, opts PipelineOpts) (*pipeline.ChangelogConfig, error) {
	cCfg := &pipeline.ChangelogConfig{
		Commit: opts.Commit || opts.Tag,
		Tag:    opts.Tag,
		NoPush: opts.NoPush,
	}

	// Resolve effective changelog: start from root, deep-merge per-env override (ADR-0019).
	effectiveChangelog := cfg.Changelog
	if opts.Env != "" {
		if envCfg, ok := cfg.Environments[opts.Env]; ok {
			cCfg.DisableChangelog = envCfg.DisableChangelog
			if envCfg.Changelog != nil {
				effectiveChangelog = config.MergeContentDriver(effectiveChangelog, envCfg.Changelog)
			}
		}
	}

	if effectiveChangelog != nil {
		enrichForge, forgeID, degradedReason, err := resolveEnrichForgeIfNeeded(readRunner, os.Getenv, cfg, opts.Force)
		if err != nil {
			return nil, err
		}
		driver := withEnvDerivations(effectiveChangelog, cfg, opts.Env)
		gen := buildGenerator(runner, driver, native.ModeChangelog, opts.HerautVersion, opts.RegenerateChangelog, opts.Force, enrichForge, degradedReason)
		cCfg.Changelog = gen
		cCfg.ChangelogFile = effectiveChangelog.Output
		cCfg.ForgeIdentity = forgeID
	}

	cCfg.AnnotatedTags = cfg.Versioning.TagType != "lightweight"
	cCfg.RegenerateChangelog = opts.RegenerateChangelog

	return cCfg, nil
}

// withEnvDerivations returns a ContentDriver copy with env-derived scoping applied
// from the effective tag format, plus the top-level enrichment_policy:
//   - HeadingVersionPattern: strips env prefix/suffix and build from changelog headings
//     (when {env} or {build} is present)
//   - TagPattern: scopes native's tag walk to the active env's tags (when {env} and the user
//     has not set an explicit tag_pattern)
//   - RemoteMetadata: the top-level Config.EnrichmentPolicy, so the generator honours
//     it (empty is left empty — the generator treats that as "optional")
//   - Tickets: the top-level Config.Tickets, so the generator can inject link_parsers
//
// The original driver is never mutated. Returns the original pointer when nothing applies.
func withEnvDerivations(driver *config.ContentDriver, cfg *config.Config, env string) *config.ContentDriver {
	tf := cfg.EffectiveTagFormat(env)
	headingPat := tagfmt.DeriveHeadingVersionPattern(tf)

	var tagPat, tagGlob string
	if driver.TagPattern == "" {
		tagPat = tagfmt.DeriveTagPattern(tf, env)
		// The git glob is the native equivalent of git-cliff's --tag-pattern regex: it scopes
		// listTags / previousTag to the active env's tags. Derived only for a per-env format
		// ({env} present) — otherwise native keeps walking all tags.
		if env != "" && strings.Contains(tf, "{env}") {
			if g, err := tagfmt.GlobPattern(tf, env); err == nil {
				tagGlob = g
			}
		}
	}

	rm := cfg.EnrichmentPolicy()
	tickets := cfg.Tickets()
	templates := effectiveTemplates(cfg, driver)
	excludes := effectiveExcludes(cfg, driver)
	hasCommits := cfg.Commits != nil && (len(cfg.Commits.Types) > 0 || cfg.Commits.TypesHeadingLevel > 0)
	hasRendering := len(excludes) > 0
	if headingPat == "" && tagPat == "" && tagGlob == "" && rm == "" && len(tickets) == 0 && !hasCommits && !hasRendering && len(templates) == 0 {
		return driver
	}
	clone := *driver
	if headingPat != "" {
		clone.HeadingVersionPattern = headingPat
	}
	if tagPat != "" {
		clone.TagPattern = tagPat
	}
	if tagGlob != "" {
		clone.TagGlob = tagGlob
	}
	if rm != "" {
		clone.RemoteMetadata = rm
	}
	if len(tickets) > 0 {
		clone.Tickets = tickets
	}
	if cfg.Commits != nil {
		clone.Types = cfg.Commits.Types
		clone.TypesHeadingLevel = cfg.Commits.TypesHeadingLevel
	}
	if len(excludes) > 0 {
		clone.Excludes = excludes
	}
	if len(templates) > 0 {
		clone.EffectiveTemplates = templates
	}
	return &clone
}

// effectiveTemplates overlays the driver's rendering.templates over the global rendering.templates
// (driver wins per key; unset keys fall through). Returns nil when neither level sets any template.
func effectiveTemplates(cfg *config.Config, driver *config.ContentDriver) map[string]string {
	var global, perDriver map[string]string
	if cfg.Rendering != nil {
		global = cfg.Rendering.Templates
	}
	if driver.Rendering != nil {
		perDriver = driver.Rendering.Templates
	}
	if len(global) == 0 && len(perDriver) == 0 {
		return nil
	}
	eff := make(map[string]string, len(global)+len(perDriver))
	maps.Copy(eff, global)
	maps.Copy(eff, perDriver)
	return eff
}

// effectiveExcludes concatenates the global rendering.excludes with this driver's own
// rendering.excludes (ADR-0037 per-driver rendering overrides, T224) — additive, not
// overriding: excludes are independent drop rules, not a single value where one level should
// replace the other, unlike effectiveTemplates' per-key overlay.
func effectiveExcludes(cfg *config.Config, driver *config.ContentDriver) []config.Exclude {
	var excludes []config.Exclude
	if cfg.Rendering != nil {
		excludes = append(excludes, cfg.Rendering.Excludes...)
	}
	if driver.Rendering != nil {
		excludes = append(excludes, driver.Rendering.Excludes...)
	}
	return excludes
}

func buildGenerator(runner port.Runner, driver *config.ContentDriver, defaultMode native.Mode, herautVersion string, regenerateChangelog, force bool, enrichForge port.Forge, degradedReason string) port.Generator {
	// Copy so setting the running version never mutates the shared config.
	nativeDriver := *driver
	nativeDriver.HerautVersion = herautVersion
	nativeDriver.RegenerateChangelog = regenerateChangelog
	nativeDriver.Force = force
	var opts []native.Option
	if enrichForge != nil {
		opts = append(opts, native.WithForge(enrichForge))
	}
	if degradedReason != "" {
		opts = append(opts, native.WithDegraded(degradedReason))
	}
	return native.New(runner, &nativeDriver, defaultMode, opts...)
}

// resolveEnrichForgeIfNeeded resolves the configured/ambient forge and constructs the matching
// port.Forge. forge.Resolve shells out to `git remote get-url origin`.
//
// When the effective enrichment policy is "disabled" (including via --offline, which forces it),
// resolution is skipped entirely rather than attempted and its error discarded: enrichment being
// switched off must never be able to *cause* a failure, e.g. an ambiguous multi-token environment
// that forge.Resolve can't disambiguate should not block an explicitly offline run.
//
// A resolution failure under any other policy is fatal only when the policy is "required" and not
// downgraded by force — matching enrichForRelease's "required fails outright" contract
// (internal/generators/native/enrich.go). Under the default/optional policy, which promises "on
// failure, degrade", a resolution failure degrades the same way a post-resolution fetch failure
// does: the returned forge/identity are nil and the third return value carries a non-empty reason
// for the caller to seed onto the generator (native.WithDegraded), instead of failing the whole
// pipeline (T175 — without this, heraut check's warn-only severity for an unconfigured, ambiguous
// changelog-only environment (T172) predicted success while heraut changelog hard-failed on the
// identical resolution error).
//
// getenv is injected rather than reaching for os.Getenv directly: forge.Resolve keys off CI
// markers (GITHUB_ACTIONS, GITLAB_CI, TF_BUILD), so a hardcoded os.Getenv would let the ambient
// CI environment of heraut's *own* pipeline decide what a test resolves — which is exactly how
// this function's tests broke on GitHub Actions while passing locally.
func resolveEnrichForgeIfNeeded(runner port.Runner, getenv func(string) string, cfg *config.Config, force bool) (port.Forge, *port.ForgeIdentity, string, error) {
	policy := cfg.EnrichmentPolicy()
	if policy == "disabled" {
		return nil, nil, "", nil
	}
	resolved, err := resolveForge(runner, getenv, cfg)
	if err != nil {
		if policy == "required" && !force {
			return nil, nil, "", err
		}
		return nil, nil, fmt.Sprintf("remote enrichment unavailable; rendering without PR attribution: %v", err), nil
	}
	enrichForge, forgeID := enrichForgeFrom(resolved)
	return enrichForge, forgeID, "", nil
}

// resolveForge is the single call site for forge.Resolve: it shells out to
// `git remote get-url origin` via runner and wraps any resolution error (e.g.
// forge.ErrAmbiguousForge). Callers that need both the enrichment forge and the full set of
// resolved identities (publishing) must call this once and derive both from its result — a
// second call would add a duplicate git subprocess invocation.
func resolveForge(runner port.Runner, getenv func(string) string, cfg *config.Config) (forge.Resolved, error) {
	resolved, err := forge.Resolve(cfg, getenv, gitOriginURL(runner))
	if err != nil {
		return forge.Resolved{}, fmt.Errorf("resolving forge: %w", err)
	}
	return resolved, nil
}

// enrichForgeFrom constructs the concrete port.Forge (and its identity) for resolved's
// enrichment index. Returns (nil, nil) when resolution found no forge at all.
func enrichForgeFrom(resolved forge.Resolved) (port.Forge, *port.ForgeIdentity) {
	if len(resolved.Forges) == 0 {
		return nil, nil
	}
	id := resolved.Forges[resolved.EnrichmentIndex]
	var enrichForge port.Forge
	switch id.Type {
	case "gitlab":
		enrichForge = gitlabforge.New(id, nil)
	case "github":
		enrichForge = githubforge.New(id, nil)
	case "azure_devops":
		enrichForge = azureforge.New(id, nil)
	}
	return enrichForge, &id
}

// gitOriginURL returns the origin remote URL, or "" when there is no origin (forge resolution
// then falls back to CI env or offline).
func gitOriginURL(runner port.Runner) string {
	out, _, err := runner.Run("git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func buildPlatform(runner port.Runner, cfg *config.Platform) (port.Platform, error) {
	build, ok := platformBuilders[strings.ToLower(cfg.Type)]
	if !ok {
		return nil, fmt.Errorf("unsupported platform %q (supported: github, gitlab)", cfg.Type)
	}
	return build(runner, cfg)
}
