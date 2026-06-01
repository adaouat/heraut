package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/cocogitto"
	"github.com/adaouat/heraut/internal/generators/communique"
	"github.com/adaouat/heraut/internal/generators/gitcliff"
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
	// SignTags mirrors git config tag.gpgSign — when true the pipeline creates
	// signed tags (-s) instead of annotated ones. Set by the caller via ReadGPGSign.
	SignTags bool
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
	pipelineCfg, err := buildReleasePipelineConfig(runner, cfg, opts.Env)
	if err != nil {
		return nil, err
	}
	pipelineCfg.SignTags = opts.SignTags

	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	pipe := pipeline.New(runner, resolver, pipelineCfg, out, opts.DryRun)
	pipe = pipe.WithReporter(ui.NewProgress(out, releaseStepTotal(pipelineCfg)).Step)
	return pipe, nil
}

// releaseStepTotal computes the number of numbered steps for a release pipeline.
// Asset uploads are sub-results of the platform step, not separate numbered steps.
func releaseStepTotal(cfg *pipeline.Config) int {
	total := 3 // resolve version + create tag + push tag
	if cfg.Changelog != nil && !cfg.DisableChangelog {
		total += 2 // generate changelog + commit changelog
	}
	if cfg.Notes != nil && !cfg.DisableNotes {
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
	pipe = pipe.WithReporter(ui.NewProgress(out, changelogStepTotal(changelogCfg)).Step)
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
		total += 2 // create tag + push tags
	}
	return total
}

func buildReleasePipelineConfig(runner port.Runner, cfg *config.Config, env string) (*pipeline.Config, error) {
	pCfg := &pipeline.Config{}

	// Resolve effective config: start from root, apply per-env overrides.
	effectiveChangelog := cfg.Changelog
	var effectiveNotes *config.ContentDriver
	var effectivePlatforms []config.Platform
	var releaseAssets []string
	if cfg.Release != nil {
		effectiveNotes = cfg.Release.Notes
		effectivePlatforms = cfg.Release.Platforms
		releaseAssets = cfg.Release.Assets
	}

	if env != "" {
		if envCfg, ok := cfg.Environments[env]; ok {
			pCfg.DisableChangelog = envCfg.DisableChangelog
			pCfg.DisableNotes = envCfg.DisableNotes
			if envCfg.Changelog != nil {
				effectiveChangelog = envCfg.Changelog
			}
			if envCfg.Release != nil {
				if envCfg.Release.Notes != nil {
					effectiveNotes = envCfg.Release.Notes
				}
				if len(envCfg.Release.Platforms) > 0 {
					effectivePlatforms = envCfg.Release.Platforms
				}
			}
		}
	}

	// Changelog generator
	if effectiveChangelog != nil {
		driver := withBuildPostprocessor(effectiveChangelog, cfg, env)
		gen, err := buildGenerator(runner, driver, gitcliff.ModeChangelog)
		if err != nil {
			return nil, fmt.Errorf("changelog generator: %w", err)
		}
		pCfg.Changelog = gen
		pCfg.ChangelogFile = effectiveChangelog.Output
	}

	// Release notes generator
	if effectiveNotes != nil {
		driver := withBuildPostprocessor(effectiveNotes, cfg, env)
		gen, err := buildGenerator(runner, driver, gitcliff.ModeReleaseNotes)
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

	return pCfg, nil
}

func buildChangelogPipelineConfig(runner port.Runner, cfg *config.Config, opts PipelineOpts) (*pipeline.ChangelogConfig, error) {
	cCfg := &pipeline.ChangelogConfig{
		Commit: opts.Commit || opts.Tag,
		Tag:    opts.Tag,
	}

	// Resolve effective changelog: start from root, apply per-env override.
	effectiveChangelog := cfg.Changelog
	if opts.Env != "" {
		if envCfg, ok := cfg.Environments[opts.Env]; ok {
			cCfg.DisableChangelog = envCfg.DisableChangelog
			if envCfg.Changelog != nil {
				effectiveChangelog = envCfg.Changelog
			}
		}
	}

	if effectiveChangelog != nil {
		driver := withBuildPostprocessor(effectiveChangelog, cfg, opts.Env)
		gen, err := buildGenerator(runner, driver, gitcliff.ModeChangelog)
		if err != nil {
			return nil, fmt.Errorf("changelog generator: %w", err)
		}
		cCfg.Changelog = gen
		cCfg.ChangelogFile = effectiveChangelog.Output
	}

	cCfg.AnnotatedTags = cfg.Versioning.TagType != "lightweight"

	return cCfg, nil
}

// withBuildPostprocessor returns a ContentDriver copy with BuildPostprocessorPattern
// populated when {build} is present in the effective tag format. The original
// driver is not modified. Returns the original pointer when no derivation is needed.
func withBuildPostprocessor(driver *config.ContentDriver, cfg *config.Config, env string) *config.ContentDriver {
	pat := tagfmt.DeriveBuildPostprocessorPattern(cfg.EffectiveTagFormat(env))
	if pat == "" {
		return driver
	}
	clone := *driver
	clone.BuildPostprocessorPattern = pat
	return &clone
}

func buildGenerator(runner port.Runner, driver *config.ContentDriver, defaultMode gitcliff.Mode) (port.Generator, error) {
	switch strings.ToLower(driver.Generator) {
	case "git-cliff":
		return gitcliff.New(runner, driver, defaultMode), nil
	case "communique":
		return communique.New(runner, driver), nil
	case "cocogitto":
		mode := cocogitto.ModeChangelog
		if defaultMode == gitcliff.ModeReleaseNotes {
			mode = cocogitto.ModeReleaseNotes
		}
		return cocogitto.New(runner, driver, mode), nil
	default:
		return nil, fmt.Errorf("unsupported generator %q (supported: git-cliff, communique, cocogitto)", driver.Generator)
	}
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
