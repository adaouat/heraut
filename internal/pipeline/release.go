package pipeline

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/adaouat/heraut/internal/versioning"
)

// Pipeline executes the full release flow.
type Pipeline struct {
	git      gitHelper
	resolver versioning.Resolver
	cfg      *Config
	out      io.Writer
	dryRun   bool
	reporter ui.StepFn
	logger   *slog.Logger
}

// New constructs a release Pipeline.
func New(runner port.Runner, resolver versioning.Resolver, cfg *Config, out io.Writer, dryRun bool) *Pipeline {
	return &Pipeline{git: gitHelper{runner: runner}, resolver: resolver, cfg: cfg, out: out, dryRun: dryRun}
}

// WithReporter sets the step reporter and returns p for chaining.
// When reporter is nil (the zero value), Run() behaves identically to the
// pre-reporter implementation — no output beyond the final summary.
func (p *Pipeline) WithReporter(fn ui.StepFn) *Pipeline {
	p.reporter = fn
	return p
}

// WithLogger sets the operator-debug diagnostic logger and returns p for chaining.
// When unset, diagnostics are discarded. See forge ADR-0011.
func (p *Pipeline) WithLogger(l *slog.Logger) *Pipeline {
	p.logger = l
	return p
}

// debug emits an operator-debug log line when a logger is set.
func (p *Pipeline) debug(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}

// runStep calls fn via the reporter when one is set, or directly when nil.
// Errors returned by fn are propagated verbatim so callers can use errors.Is/As.
func (p *Pipeline) runStep(name string, fn func() (string, []string, error)) error {
	p.debug("step started", "step", name)
	logged := func() (string, []string, error) {
		out, sub, err := fn()
		if err == nil {
			p.debug("step completed", "step", name)
		}
		return out, sub, err
	}
	if p.reporter == nil {
		_, _, err := logged()
		return err
	}
	return p.reporter(name, logged)
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
//  6. For each platform: UploadAssets (if platform.HasAssets()) — reported as sub-result
func (p *Pipeline) Run() error {
	// Step 1: Resolve version.
	var result versioning.Result
	if err := p.runStep("Resolve version", func() (string, []string, error) {
		r, err := p.resolver.Resolve()
		if err != nil {
			return "", nil, fmt.Errorf("resolving version: %w", err)
		}
		result = r
		p.debug("resolved version", "tag", r.Tag, "version", r.Version, "current_tag", r.CurrentTag)
		return r.Tag, nil, nil
	}); err != nil {
		return err
	}

	if p.dryRun {
		return p.dryRunOutput(result)
	}

	// Step 2+3: Generate and commit changelog (conditional).
	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		if err := p.runStep("Generate changelog", func() (string, []string, error) {
			if _, err := p.cfg.Changelog.Generate(result.Tag, nil); err != nil {
				return "", nil, fmt.Errorf("generating changelog: %w", err)
			}
			return "", nil, nil
		}); err != nil {
			return err
		}

		file := p.cfg.ChangelogFile
		if file == "" {
			file = "CHANGELOG.md"
		}
		if err := p.runStep("Commit changelog", func() (string, []string, error) {
			if err := p.git.commitChangelog(file, commitMessage(p.cfg.CommitMessage, result.Version), true); err != nil {
				return "", nil, fmt.Errorf("committing changelog: %w", err)
			}
			return "", nil, nil
		}); err != nil {
			return err
		}
	}

	// Step 4: Create tag.
	if err := p.runStep(fmt.Sprintf("Create tag %s", result.Tag), func() (string, []string, error) {
		if err := p.git.tag(result.Tag, commitMessage(p.cfg.CommitMessage, result.Version), p.cfg.AnnotatedTags, p.cfg.SignTags); err != nil {
			return "", nil, fmt.Errorf("git tag: %w", err)
		}
		return "", nil, nil
	}); err != nil {
		return err
	}

	// Step 5: Push tag.
	if err := p.runStep("Push tag", func() (string, []string, error) {
		if err := p.git.run("git", "push", "origin", "--tags"); err != nil {
			return "", nil, fmt.Errorf("git push --tags: %w", err)
		}
		return "", nil, nil
	}); err != nil {
		return err
	}

	// Step 6: Generate release notes (conditional).
	var notes string
	if p.cfg.Notes != nil && !p.cfg.DisableNotes {
		if err := p.runStep("Generate release notes", func() (string, []string, error) {
			var genErr error
			notes, genErr = p.cfg.Notes.Generate(result.Tag, nil)
			if genErr != nil {
				return "", nil, fmt.Errorf("generating release notes: %w", genErr)
			}
			return "", nil, nil
		}); err != nil {
			return err
		}
	}

	// Step 7: Publish to each platform; asset upload is a sub-result of this step.
	for _, platform := range p.cfg.Platforms {
		plat := platform // capture loop variable
		if err := p.runStep(fmt.Sprintf("Publish to %s", plat.Name()), func() (string, []string, error) {
			if err := plat.CreateRelease(result.Tag, notes); err != nil {
				return "", nil, fmt.Errorf("platform %s: create release: %w", plat.Name(), err)
			}
			var subs []string
			if plat.HasAssets() {
				if err := plat.UploadAssets(result.Tag); err != nil {
					return "", nil, fmt.Errorf("platform %s: upload assets: %w", plat.Name(), err)
				}
				subs = []string{"assets uploaded"}
			}
			return plat.ReleaseURL(result.Tag), subs, nil
		}); err != nil {
			return err
		}
	}

	p.printSummary(result)
	return nil
}

// dryRunOutput reports what would happen without performing any mutations.
// When a reporter is set it emits one step per action; otherwise it falls back
// to the plain [dry-run] lines for CI-friendly output.
func (p *Pipeline) dryRunOutput(result versioning.Result) error {
	if p.reporter == nil {
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

	// Reporter path: emit one informational step per would-be action.
	file := p.cfg.ChangelogFile
	if file == "" {
		file = "CHANGELOG.md"
	}

	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		_ = p.runStep("Generate changelog", func() (string, []string, error) {
			return "[dry-run] would write " + file, nil, nil
		})
		_ = p.runStep("Commit changelog", func() (string, []string, error) {
			return "[dry-run] would commit and push", nil, nil
		})
	}

	_ = p.runStep(fmt.Sprintf("Create tag %s", result.Tag), func() (string, []string, error) {
		return "[dry-run] would tag", nil, nil
	})
	_ = p.runStep("Push tag", func() (string, []string, error) {
		return "[dry-run] would push", nil, nil
	})

	if p.cfg.Notes != nil && !p.cfg.DisableNotes {
		_ = p.runStep("Generate release notes", func() (string, []string, error) {
			return "[dry-run] would generate", nil, nil
		})
	}

	for _, platform := range p.cfg.Platforms {
		plat := platform
		_ = p.runStep(fmt.Sprintf("Publish to %s", plat.Name()), func() (string, []string, error) {
			var subs []string
			if plat.HasAssets() {
				subs = []string{"[dry-run] would upload assets"}
			}
			return "[dry-run] would create release", subs, nil
		})
	}
	return nil
}

// printSummary writes the post-run summary to p.out.
// With a reporter it uses the styled block; without one it keeps the original
// single-line format so existing plain callers are unaffected.
func (p *Pipeline) printSummary(result versioning.Result) {
	if p.reporter != nil {
		_, _ = fmt.Fprintf(p.out, "\nReleased %s\n", result.Tag)
		for _, platform := range p.cfg.Platforms {
			_, _ = fmt.Fprintf(p.out, "  › %-8s %s\n", platform.Name(), platform.ReleaseURL(result.Tag))
		}
		return
	}
	_, _ = fmt.Fprintf(p.out, "released %s\n", result.Tag)
	for _, platform := range p.cfg.Platforms {
		_, _ = fmt.Fprintf(p.out, "  %s: %s\n", platform.Name(), platform.ReleaseURL(result.Tag))
	}
}
