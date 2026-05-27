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
	"github.com/adaouat/heraut/internal/versioning"
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
}

// BuildPipeline constructs a release Pipeline from config. All generator and platform
// instances are created here — none are created in internal/cmd/.
func BuildPipeline(runner port.Runner, cfg *config.Config, resolver versioning.Resolver, opts PipelineOpts) (*pipeline.Pipeline, error) {
	pipelineCfg, err := buildReleasePipelineConfig(runner, cfg, opts.Env)
	if err != nil {
		return nil, err
	}

	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	return pipeline.New(runner, resolver, pipelineCfg, out, opts.DryRun), nil
}

// BuildChangelogPipeline constructs a ChangelogPipeline from config.
func BuildChangelogPipeline(runner port.Runner, cfg *config.Config, resolver versioning.Resolver, opts PipelineOpts) (*pipeline.ChangelogPipeline, error) {
	changelogCfg, err := buildChangelogPipelineConfig(runner, cfg, opts)
	if err != nil {
		return nil, err
	}

	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	return pipeline.NewChangelog(runner, resolver, changelogCfg, out, opts.DryRun), nil
}

func buildReleasePipelineConfig(runner port.Runner, cfg *config.Config, env string) (*pipeline.Config, error) {
	pCfg := &pipeline.Config{}

	// Resolve effective config: start from root, apply per-env overrides.
	effectiveChangelog := cfg.Changelog
	var effectiveNotes *config.ContentDriver
	var effectivePlatforms []config.Platform
	if cfg.Release != nil {
		effectiveNotes = cfg.Release.Notes
		effectivePlatforms = cfg.Release.Platforms
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
		gen, err := buildGenerator(runner, effectiveChangelog, gitcliff.ModeChangelog)
		if err != nil {
			return nil, fmt.Errorf("changelog generator: %w", err)
		}
		pCfg.Changelog = gen
		pCfg.ChangelogFile = effectiveChangelog.Output
	}

	// Release notes generator
	if effectiveNotes != nil {
		gen, err := buildGenerator(runner, effectiveNotes, gitcliff.ModeReleaseNotes)
		if err != nil {
			return nil, fmt.Errorf("release notes generator: %w", err)
		}
		pCfg.Notes = gen
	}

	// Platforms
	for i, platCfg := range effectivePlatforms {
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
		gen, err := buildGenerator(runner, effectiveChangelog, gitcliff.ModeChangelog)
		if err != nil {
			return nil, fmt.Errorf("changelog generator: %w", err)
		}
		cCfg.Changelog = gen
		cCfg.ChangelogFile = effectiveChangelog.Output
	}

	cCfg.AnnotatedTags = cfg.Versioning.TagType != "lightweight"

	return cCfg, nil
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
