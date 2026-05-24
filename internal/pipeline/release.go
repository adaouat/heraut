package pipeline

import (
	"fmt"
	"io"
	"strings"

	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
)

const defaultCommitMessage = "chore(release): ${version}"

// Pipeline executes the full release flow.
type Pipeline struct {
	runner   port.Runner
	resolver versioning.Resolver
	cfg      *Config
	out      io.Writer
	dryRun   bool
}

// New constructs a release Pipeline.
func New(runner port.Runner, resolver versioning.Resolver, cfg *Config, out io.Writer, dryRun bool) *Pipeline {
	return &Pipeline{runner: runner, resolver: resolver, cfg: cfg, out: out, dryRun: dryRun}
}

// Check verifies all generators and platforms are usable before running.
func (p *Pipeline) Check() error {
	if p.cfg.Changelog != nil {
		if err := p.cfg.Changelog.Check(); err != nil {
			return fmt.Errorf("changelog generator: %w", err)
		}
	}
	if p.cfg.Notes != nil {
		if err := p.cfg.Notes.Check(); err != nil {
			return fmt.Errorf("release-notes generator: %w", err)
		}
	}
	for _, platform := range p.cfg.Platforms {
		if err := platform.Check(); err != nil {
			return fmt.Errorf("platform %s: %w", platform.Name(), err)
		}
	}
	return nil
}

// Run executes the full release sequence:
//  1. Resolve version
//  2. (if changelog configured and not disabled) Generate changelog → git add → git commit → git push
//  3. git tag → git push --tags
//  4. (if notes configured) Generate release notes
//  5. For each platform: CreateRelease (with notes if available)
//  6. For each platform: UploadAssets (if platform.HasAssets())
func (p *Pipeline) Run() error {
	result, err := p.resolver.Resolve()
	if err != nil {
		return fmt.Errorf("resolving version: %w", err)
	}

	if p.dryRun {
		return p.dryRunOutput(result)
	}

	// Changelog step
	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		if _, err := p.cfg.Changelog.Generate(result.Tag); err != nil {
			return fmt.Errorf("generating changelog: %w", err)
		}
		if err := p.gitCommitChangelog(result); err != nil {
			return err
		}
	}

	// Tag and push
	if err := p.run("git", "tag", result.Tag); err != nil {
		return fmt.Errorf("git tag: %w", err)
	}
	if err := p.run("git", "push", "--tags"); err != nil {
		return fmt.Errorf("git push --tags: %w", err)
	}

	// Release notes
	var notes string
	if p.cfg.Notes != nil {
		notes, err = p.cfg.Notes.Generate(result.Tag)
		if err != nil {
			return fmt.Errorf("generating release notes: %w", err)
		}
	}

	// Publish to platforms
	for _, platform := range p.cfg.Platforms {
		if err := platform.CreateRelease(result.Tag, notes); err != nil {
			return fmt.Errorf("platform %s: create release: %w", platform.Name(), err)
		}
		if platform.HasAssets() {
			if err := platform.UploadAssets(result.Tag); err != nil {
				return fmt.Errorf("platform %s: upload assets: %w", platform.Name(), err)
			}
		}
	}

	_, _ = fmt.Fprintf(p.out, "released %s\n", result.Tag)
	for _, platform := range p.cfg.Platforms {
		_, _ = fmt.Fprintf(p.out, "  %s: %s\n", platform.Name(), platform.ReleaseURL(result.Tag))
	}
	return nil
}

func (p *Pipeline) gitCommitChangelog(result versioning.Result) error {
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

func (p *Pipeline) commitMessage(version string) string {
	tmpl := p.cfg.CommitMessage
	if tmpl == "" {
		tmpl = defaultCommitMessage
	}
	return strings.ReplaceAll(tmpl, "${version}", version)
}

func (p *Pipeline) run(name string, args ...string) error {
	_, _, err := p.runner.Run(name, args...)
	return err
}

func (p *Pipeline) dryRunOutput(result versioning.Result) error {
	_, _ = fmt.Fprintf(p.out, "[dry-run] would release %s\n", result.Tag)
	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		_, _ = fmt.Fprintf(p.out, "[dry-run] would generate changelog → commit → push\n")
	}
	_, _ = fmt.Fprintf(p.out, "[dry-run] would tag %s and push\n", result.Tag)
	for _, platform := range p.cfg.Platforms {
		_, _ = fmt.Fprintf(p.out, "[dry-run] would publish to %s\n", platform.Name())
	}
	return nil
}
