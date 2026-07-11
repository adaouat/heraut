package app

import (
	"fmt"
	"io"
	"log/slog"
	"maps"
	"strings"

	forgeui "github.com/adaouat/forge/ui"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/communique"
	"github.com/adaouat/heraut/internal/generators/gitcliff"
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
	pipelineCfg, err := buildReleasePipelineConfig(runner, cfg, opts.Env, opts.HerautVersion, opts.RegenerateChangelog)
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
	changelogCfg, err := buildChangelogPipelineConfig(runner, cfg, opts)
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

func buildReleasePipelineConfig(runner port.Runner, cfg *config.Config, env, herautVersion string, regenerateChangelog bool) (*pipeline.Config, error) {
	pCfg := &pipeline.Config{}

	// Resolve effective config: start from root, apply per-env overrides.
	effectiveChangelog := cfg.Changelog
	var effectiveNotes *config.ContentDriver
	var releaseAssets []string
	if cfg.Release != nil {
		effectiveNotes = cfg.Release.Notes
		releaseAssets = cfg.Release.Assets
	}
	effectivePlatforms := config.EffectivePlatforms(cfg, env)

	if env != "" {
		if envCfg, ok := cfg.Environments[env]; ok {
			pCfg.DisableChangelog = envCfg.DisableChangelog
			pCfg.DisableNotes = envCfg.DisableNotes
			if envCfg.Changelog != nil {
				effectiveChangelog = config.MergeContentDriver(effectiveChangelog, envCfg.Changelog)
			}
			if envCfg.Release != nil {
				if envCfg.Release.Notes != nil {
					effectiveNotes = config.MergeContentDriver(effectiveNotes, envCfg.Release.Notes)
				}
			}
		}
	}

	// Changelog generator
	if effectiveChangelog != nil {
		driver := withEnvDerivations(effectiveChangelog, cfg, env)
		gen, err := buildGenerator(runner, driver, gitcliff.ModeChangelog, herautVersion, regenerateChangelog)
		if err != nil {
			return nil, fmt.Errorf("changelog generator: %w", err)
		}
		pCfg.Changelog = gen
		pCfg.ChangelogFile = effectiveChangelog.Output
		pCfg.ChangelogRemote = driver.Remote
	}

	// Release notes generator
	if effectiveNotes != nil {
		driver := withEnvDerivations(effectiveNotes, cfg, env)
		gen, err := buildGenerator(runner, driver, gitcliff.ModeReleaseNotes, herautVersion, regenerateChangelog)
		if err != nil {
			return nil, fmt.Errorf("release notes generator: %w", err)
		}
		pCfg.Notes = gen
	}

	// Platforms — propagate release.assets (top-level) to each platform with lenient
	// glob semantics (warn on no-match instead of error).
	for i, platCfg := range effectivePlatforms {
		if len(releaseAssets) > 0 && len(effectivePlatforms[i].Assets) == 0 {
			effectivePlatforms[i].Assets = releaseAssets
			effectivePlatforms[i].LenientAssets = true
		}
		p, err := buildPlatform(runner, &effectivePlatforms[i])
		if err != nil {
			return nil, fmt.Errorf("platform %d (%s): %w", i, platCfg.Type, err)
		}
		pCfg.Platforms = append(pCfg.Platforms, p)
	}

	pCfg.AnnotatedTags = cfg.Versioning.TagType != "lightweight"
	pCfg.RegenerateChangelog = regenerateChangelog

	return pCfg, nil
}

func buildChangelogPipelineConfig(runner port.Runner, cfg *config.Config, opts PipelineOpts) (*pipeline.ChangelogConfig, error) {
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
		driver := withEnvDerivations(effectiveChangelog, cfg, opts.Env)
		gen, err := buildGenerator(runner, driver, gitcliff.ModeChangelog, opts.HerautVersion, opts.RegenerateChangelog)
		if err != nil {
			return nil, fmt.Errorf("changelog generator: %w", err)
		}
		cCfg.Changelog = gen
		cCfg.ChangelogFile = effectiveChangelog.Output
		cCfg.ChangelogRemote = driver.Remote
	}

	cCfg.AnnotatedTags = cfg.Versioning.TagType != "lightweight"
	cCfg.RegenerateChangelog = opts.RegenerateChangelog

	return cCfg, nil
}

// withEnvDerivations returns a ContentDriver copy with env-derived scoping applied
// from the effective tag format, plus the top-level remote_metadata policy:
//   - HeadingVersionPattern: strips env prefix/suffix and build from changelog headings
//     (when {env} or {build} is present)
//   - TagPattern: scopes git-cliff to the active env's tags (when {env} and the user has
//     not set an explicit tag_pattern)
//   - RemoteMetadata: the top-level Config.RemoteMetadata policy, so the generator honours
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

	rm := cfg.RemoteMetadata()
	tickets := cfg.Tickets()
	templates := effectiveTemplates(cfg, driver)
	hasCommits := cfg.Commits != nil && (len(cfg.Commits.Types) > 0 || cfg.Commits.TypesHeadingLevel > 0)
	hasRendering := cfg.Rendering != nil && len(cfg.Rendering.Excludes) > 0
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
	if cfg.Rendering != nil {
		clone.Excludes = cfg.Rendering.Excludes
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

func buildGenerator(runner port.Runner, driver *config.ContentDriver, defaultMode gitcliff.Mode, herautVersion string, regenerateChangelog bool) (port.Generator, error) {
	switch driver.Generator {
	case "git-cliff":
		return gitcliff.New(runner, driver, defaultMode), nil
	case "communique":
		return communique.New(runner, driver), nil
	case "native":
		// Copy so setting the running version never mutates the shared config.
		nativeDriver := *driver
		nativeDriver.HerautVersion = herautVersion
		nativeDriver.RegenerateChangelog = regenerateChangelog
		return native.New(runner, &nativeDriver, nativeMode(defaultMode)), nil
	default:
		return nil, fmt.Errorf("unsupported generator %q (supported: native, git-cliff, communique)", driver.Generator)
	}
}

// nativeMode maps the build-time gitcliff.Mode to the native generator's mode.
func nativeMode(m gitcliff.Mode) native.Mode {
	if m == gitcliff.ModeReleaseNotes {
		return native.ModeReleaseNotes
	}
	return native.ModeChangelog
}

func buildPlatform(runner port.Runner, cfg *config.Platform) (port.Platform, error) {
	switch strings.ToLower(cfg.Type) {
	case "github":
		return buildGitHubPlatform(runner, cfg)
	case "gitlab":
		return buildGitLabPlatform(runner, cfg)
	default:
		return nil, fmt.Errorf("unsupported platform %q (supported: github, gitlab)", cfg.Type)
	}
}
