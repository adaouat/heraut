package pipeline

import (
	"fmt"
	"io"
	"strings"

	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
)

// ChangelogConfig holds runtime options for a changelog pipeline run.
type ChangelogConfig struct {
	// Changelog is the optional changelog generator.
	Changelog port.Generator
	// ChangelogFile is the output path for the changelog.
	ChangelogFile string
	// CommitMessage is the git commit message template. Defaults to "chore(release): ${version}".
	CommitMessage string
	// DisableChangelog skips all steps and exits 0 with an info message.
	DisableChangelog bool
	// Commit causes the generated changelog to be committed and pushed.
	Commit bool
	// Tag creates a git tag after committing (implies Commit).
	Tag bool
	// AnnotatedTags creates annotated git tags (-a -m <commit_message>).
	// When false, lightweight tags are created. Defaults to false (set by app layer).
	AnnotatedTags bool
}

// ChangelogPipeline executes the changelog-only flow.
type ChangelogPipeline struct {
	runner   port.Runner
	resolver versioning.Resolver
	cfg      *ChangelogConfig
	out      io.Writer
	dryRun   bool
}

// NewChangelog constructs a ChangelogPipeline.
func NewChangelog(runner port.Runner, resolver versioning.Resolver, cfg *ChangelogConfig, out io.Writer, dryRun bool) *ChangelogPipeline {
	return &ChangelogPipeline{runner: runner, resolver: resolver, cfg: cfg, out: out, dryRun: dryRun}
}

// Run executes the changelog sequence:
//  1. Resolve version
//  2. If DisableChangelog: print info and return
//  3. If Changelog configured: generate changelog
//  4. If Commit or Tag (and Changelog configured): git add → git commit → git push
//  5. If Tag: git tag → git push --tags
func (p *ChangelogPipeline) Run() error {
	result, err := p.resolver.Resolve()
	if err != nil {
		return fmt.Errorf("resolving version: %w", err)
	}

	if p.cfg.DisableChangelog {
		_, _ = fmt.Fprintf(p.out, "changelog disabled for %s\n", result.Tag)
		return nil
	}

	if p.dryRun {
		return p.dryRunOutput(result)
	}

	// Generate changelog
	if p.cfg.Changelog != nil {
		if _, err := p.cfg.Changelog.Generate(result.Tag); err != nil {
			return fmt.Errorf("generating changelog: %w", err)
		}

		// Commit the generated file when --commit or --tag is set
		if p.cfg.Commit || p.cfg.Tag {
			if err := p.gitCommitChangelog(result); err != nil {
				return fmt.Errorf("committing changelog: %w", err)
			}
		}
	}

	// Tag the commit when --tag is set
	if p.cfg.Tag {
		if err := p.gitTag(result.Tag, result.Version); err != nil {
			return fmt.Errorf("git tag: %w", err)
		}
		if err := p.run("git", "push", "--tags"); err != nil {
			return fmt.Errorf("git push --tags: %w", err)
		}
	}

	_, _ = fmt.Fprintf(p.out, "changelog updated for %s\n", result.Tag)
	return nil
}

func (p *ChangelogPipeline) gitCommitChangelog(result versioning.Result) error {
	file := p.cfg.ChangelogFile
	if file == "" {
		file = "CHANGELOG.md"
	}
	if err := p.run("git", "add", file); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	msg := p.commitMessage(result.Version)
	if err := p.run("git", "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	if err := p.run("git", "push"); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

func (p *ChangelogPipeline) commitMessage(version string) string {
	tmpl := p.cfg.CommitMessage
	if tmpl == "" {
		tmpl = defaultCommitMessage
	}
	return strings.ReplaceAll(tmpl, "${version}", version)
}

func (p *ChangelogPipeline) gitTag(tag, version string) error {
	if p.cfg.AnnotatedTags {
		return p.run("git", "tag", "-a", tag, "-m", p.commitMessage(version))
	}
	return p.run("git", "tag", tag)
}

func (p *ChangelogPipeline) run(name string, args ...string) error {
	_, _, err := p.runner.Run(name, args...)
	return err
}

func (p *ChangelogPipeline) dryRunOutput(result versioning.Result) error {
	_, _ = fmt.Fprintf(p.out, "[dry-run] would generate changelog for %s\n", result.Tag)
	if p.cfg.Commit || p.cfg.Tag {
		_, _ = fmt.Fprintf(p.out, "[dry-run] would commit → push\n")
	}
	if p.cfg.Tag {
		_, _ = fmt.Fprintf(p.out, "[dry-run] would tag %s and push\n", result.Tag)
	}
	return nil
}
