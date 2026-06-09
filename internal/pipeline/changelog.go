package pipeline

import (
	"fmt"
	"io"

	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/ui"
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
	// NoPush keeps the commit and tag local: the changelog is committed (and the
	// tag created) but neither `git push origin HEAD` nor `git push origin --tags`
	// runs. The zero value (false) preserves the default push behaviour.
	NoPush bool
	// AnnotatedTags creates annotated git tags (-a -m <commit_message>).
	// When false, lightweight tags are created. Defaults to false (set by app layer).
	AnnotatedTags bool
	// SignTags creates GPG-signed tags (-s -m <commit_message>), overriding AnnotatedTags.
	// Populated from git config tag.gpgSign by the app layer.
	SignTags bool
}

// ChangelogPipeline executes the changelog-only flow.
type ChangelogPipeline struct {
	git      gitHelper
	resolver versioning.Resolver
	cfg      *ChangelogConfig
	out      io.Writer
	dryRun   bool
	reporter ui.StepFn
}

// NewChangelog constructs a ChangelogPipeline.
func NewChangelog(runner port.Runner, resolver versioning.Resolver, cfg *ChangelogConfig, out io.Writer, dryRun bool) *ChangelogPipeline {
	return &ChangelogPipeline{git: gitHelper{runner: runner}, resolver: resolver, cfg: cfg, out: out, dryRun: dryRun}
}

// WithReporter sets the step reporter and returns p for chaining.
// When reporter is nil (the zero value), Run() behaves identically to the
// pre-reporter implementation — no output beyond the final summary.
func (p *ChangelogPipeline) WithReporter(fn ui.StepFn) *ChangelogPipeline {
	p.reporter = fn
	return p
}

// runStep calls fn via the reporter when one is set, or directly when nil.
// Errors returned by fn are propagated verbatim so callers can use errors.Is/As.
func (p *ChangelogPipeline) runStep(name string, fn func() (string, []string, error)) error {
	if p.reporter == nil {
		_, _, err := fn()
		return err
	}
	return p.reporter(name, fn)
}

// Run executes the changelog sequence:
//  1. Resolve version
//  2. If DisableChangelog: print info and return
//  3. If Changelog configured: generate changelog
//  4. If Commit or Tag (and Changelog configured): git add → git commit → git push
//  5. If Tag: git tag → git push --tags
//
// When NoPush is set, both pushes (HEAD and --tags) are skipped — the commit and
// tag are created locally only.
func (p *ChangelogPipeline) Run() error {
	// Step 1: Resolve version.
	var result versioning.Result
	if err := p.runStep("Resolve version", func() (string, []string, error) {
		r, err := p.resolver.Resolve()
		if err != nil {
			return "", nil, fmt.Errorf("resolving version: %w", err)
		}
		result = r
		return r.Tag, nil, nil
	}); err != nil {
		return err
	}

	if p.cfg.DisableChangelog {
		if p.reporter != nil {
			_, _ = fmt.Fprintln(p.out, ui.Warn(p.out, "changelog disabled"))
		} else {
			_, _ = fmt.Fprintf(p.out, "changelog disabled for %s\n", result.Tag)
		}
		if !p.cfg.Tag {
			return nil
		}
		// Tag is true: skip changelog steps but proceed to tag.
	}

	if p.dryRun {
		return p.dryRunOutput(result)
	}

	// Step 2: Generate changelog (skipped when DisableChangelog is true). The committed
	// changelog is tied to origin, so it resolves links from the ambient CI host (ADR-0022).
	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		changelogCtx := ambientLinkContext()
		if err := p.runStep("Generate changelog", func() (string, []string, error) {
			if _, err := p.cfg.Changelog.Generate(result.Tag, changelogCtx); err != nil {
				return "", nil, fmt.Errorf("generating changelog: %w", err)
			}
			return "", nil, nil
		}); err != nil {
			return err
		}

		// Step 3: Commit changelog (conditional).
		if p.cfg.Commit || p.cfg.Tag {
			file := p.cfg.ChangelogFile
			if file == "" {
				file = "CHANGELOG.md"
			}
			if err := p.runStep("Commit changelog", func() (string, []string, error) {
				if err := p.git.commitChangelog(file, commitMessage(p.cfg.CommitMessage, result.Version), !p.cfg.NoPush); err != nil {
					return "", nil, fmt.Errorf("committing changelog: %w", err)
				}
				return "", nil, nil
			}); err != nil {
				return err
			}
		}
	}

	// Step 4+5: Tag the commit (conditional).
	if p.cfg.Tag {
		if err := p.runStep(fmt.Sprintf("Create tag %s", result.Tag), func() (string, []string, error) {
			if err := p.git.tag(result.Tag, commitMessage(p.cfg.CommitMessage, result.Version), p.cfg.AnnotatedTags, p.cfg.SignTags); err != nil {
				return "", nil, fmt.Errorf("git tag: %w", err)
			}
			return "", nil, nil
		}); err != nil {
			return err
		}

		if !p.cfg.NoPush {
			if err := p.runStep("Push tags", func() (string, []string, error) {
				if err := p.git.run("git", "push", "origin", "--tags"); err != nil {
					return "", nil, fmt.Errorf("git push: %w", err)
				}
				return "", nil, nil
			}); err != nil {
				return err
			}
		}
	}

	p.printSummary(result)
	return nil
}

// dryRunOutput reports what would happen without performing any mutations.
// When a reporter is set it emits one step per action with [dry-run] result
// prefixes; otherwise it falls back to plain [dry-run] lines.
func (p *ChangelogPipeline) dryRunOutput(result versioning.Result) error {
	if p.reporter == nil {
		if !p.cfg.DisableChangelog {
			_, _ = fmt.Fprintf(p.out, "[dry-run] would generate changelog for %s\n", result.Tag)
			if p.cfg.Commit || p.cfg.Tag {
				if p.cfg.NoPush {
					_, _ = fmt.Fprintf(p.out, "[dry-run] would commit (no push)\n")
				} else {
					_, _ = fmt.Fprintf(p.out, "[dry-run] would commit → push\n")
				}
			}
		}
		if p.cfg.Tag {
			if p.cfg.NoPush {
				_, _ = fmt.Fprintf(p.out, "[dry-run] would tag %s (no push)\n", result.Tag)
			} else {
				_, _ = fmt.Fprintf(p.out, "[dry-run] would tag %s and push\n", result.Tag)
			}
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
		if p.cfg.Commit || p.cfg.Tag {
			_ = p.runStep("Commit changelog", func() (string, []string, error) {
				if p.cfg.NoPush {
					return "[dry-run] would commit (no push)", nil, nil
				}
				return "[dry-run] would commit and push", nil, nil
			})
		}
	}

	if p.cfg.Tag {
		_ = p.runStep(fmt.Sprintf("Create tag %s", result.Tag), func() (string, []string, error) {
			return "[dry-run] would tag", nil, nil
		})
		if !p.cfg.NoPush {
			_ = p.runStep("Push tags", func() (string, []string, error) {
				return "[dry-run] would push", nil, nil
			})
		}
	}
	return nil
}

// printSummary writes the post-run summary to p.out.
// With a reporter it uses the styled block; without one it keeps the original
// single-line format so existing plain callers are unaffected.
func (p *ChangelogPipeline) printSummary(result versioning.Result) {
	if p.reporter != nil {
		_, _ = fmt.Fprintf(p.out, "\nChangelog updated for %s\n", result.Tag)
		if (p.cfg.Commit || p.cfg.Tag) && p.cfg.Changelog != nil {
			file := p.cfg.ChangelogFile
			if file == "" {
				file = "CHANGELOG.md"
			}
			if p.cfg.NoPush {
				_, _ = fmt.Fprintf(p.out, "  %s committed (not pushed)\n", file)
			} else {
				_, _ = fmt.Fprintf(p.out, "  %s committed and pushed\n", file)
			}
		}
		return
	}
	_, _ = fmt.Fprintf(p.out, "changelog updated for %s\n", result.Tag)
}
