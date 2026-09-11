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
	// ForgeIdentity is the resolved enrichment forge (ADR-0043), consumed ahead of the
	// ambient CI-host fallback.
	ForgeIdentity *port.ForgeIdentity
	// CommitMessage is the git commit message template. Defaults to "chore(release): ${version}".
	CommitMessage string
	// DisableChangelog skips all steps and exits 0 with an info message.
	DisableChangelog bool
	// Commit causes the generated changelog to be committed and pushed.
	Commit bool
	// Tag creates a git tag after committing (implies Commit).
	Tag bool
	// NoPush keeps the commit and tag local: the changelog is committed (and the
	// tag created) but neither `git push origin HEAD` nor `git push origin <tag>`
	// runs. The zero value (false) preserves the default push behaviour.
	NoPush bool
	// AnnotatedTags creates annotated git tags (-a -m <commit_message>).
	// When false, lightweight tags are created. Defaults to false (set by app layer).
	AnnotatedTags bool
	// SignTags creates GPG-signed tags (-s -m <commit_message>), overriding AnnotatedTags.
	// Populated from git config tag.gpgSign by the app layer.
	SignTags bool
	// RegenerateChangelog mirrors the native generator's --regenerate mode: when true, the
	// changelog step re-enriches every section rather than splicing only the new one.
	RegenerateChangelog bool
	// NoHooks skips every configured hook for this run (--no-hooks), without touching config.
	NoHooks bool
	// PostBumpHooks run immediately after the next version is resolved. Empty = no hooks.
	PostBumpHooks []string
	// PreChangelogHooks run before changelog generation. Empty = no hooks.
	PreChangelogHooks []string
	// PreTagHooks run before the local git tag is created. Empty = no hooks.
	PreTagHooks []string
	// PostTagHooks run after the tag is pushed to origin. Empty = no hooks.
	PostTagHooks []string
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

// WithInteractiveRunner sets the runner used for commands that may need a real terminal — a GPG
// pinentry prompt during `git commit`/a signed tag (T260) — and returns p for chaining. When
// unset, those commands fall back to the regular runner, exactly as before this option existed.
func (p *ChangelogPipeline) WithInteractiveRunner(r port.Runner) *ChangelogPipeline {
	p.git.interactiveRunner = r
	return p
}

// hookVars builds the template variables available to hook commands from a resolved result
// (ADR-0053). Platform is always empty here — this pipeline never publishes.
func (p *ChangelogPipeline) hookVars(result versioning.Result) hookVars {
	return hookVars{Version: result.Version, Tag: result.Tag, PreviousTag: result.CurrentTag}
}

// runHookPointStep renders and executes cmds (one hook point's configured commands) as a
// reported step named name, skipping entirely — no step reported — when shouldRunHooks says
// this point shouldn't run (dry-run, --no-hooks, or nothing configured).
func (p *ChangelogPipeline) runHookPointStep(name string, cmds []string, vars hookVars) error {
	if !shouldRunHooks(p.dryRun, p.cfg.NoHooks, cmds) {
		return nil
	}
	return p.runStep(name, func() (string, []string, error) {
		return "", nil, runHookPoint(p.git.interactiveOrRunner(), cmds, vars)
	})
}

// dryRunHookLinesOrNil renders cmds into "[dry-run] would run: <cmd>" lines (T271), or nil, nil
// when NoHooks is set or nothing is configured — the dry-run counterpart to shouldRunHooks.
func (p *ChangelogPipeline) dryRunHookLinesOrNil(cmds []string, vars hookVars) ([]string, error) {
	if p.cfg.NoHooks {
		return nil, nil
	}
	return dryRunHookLines(cmds, vars)
}

// dryRunHookStep reports a would-run hook point as its own step, named identically to the real
// step runHookPointStep would report, so dry-run's step sequence matches a real run's exactly.
// A render error aborts the dry-run (propagated, not discarded).
func (p *ChangelogPipeline) dryRunHookStep(name string, cmds []string, vars hookVars) error {
	lines, err := p.dryRunHookLinesOrNil(cmds, vars)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return nil
	}
	return p.runStep(name, func() (string, []string, error) {
		return lines[0], lines[1:], nil
	})
}

// printDryRunHookLinesPlain writes would-run hook lines directly to p.out (the no-reporter
// dry-run path), one per line.
func (p *ChangelogPipeline) printDryRunHookLinesPlain(cmds []string, vars hookVars) error {
	lines, err := p.dryRunHookLinesOrNil(cmds, vars)
	if err != nil {
		return err
	}
	for _, l := range lines {
		_, _ = fmt.Fprintln(p.out, l)
	}
	return nil
}

// runOrRenderHookPoint executes cmds for real, or — during --dry-run — renders what would run
// (as a reporter step or a plain line, matching whichever output mode is active) without ever
// executing anything. Used at call sites reached before dryRunOutput's own dry-run rendering —
// currently only post_bump, which must render even in the DisableChangelog && !Tag case where
// Run() returns before the dry-run check is ever reached.
func (p *ChangelogPipeline) runOrRenderHookPoint(name string, cmds []string, vars hookVars) error {
	if !p.dryRun {
		return p.runHookPointStep(name, cmds, vars)
	}
	if p.reporter != nil {
		return p.dryRunHookStep(name, cmds, vars)
	}
	return p.printDryRunHookLinesPlain(cmds, vars)
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
//  5. If Tag: git tag → git push origin <tag>
//
// When NoPush is set, both pushes (HEAD and the tag) are skipped — the commit and
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

	// post_bump hooks fire on every resolve (ADR-0053) — including the DisableChangelog+!Tag
	// case immediately below, which returns before any other step runs and before the dry-run
	// check, so runOrRenderHookPoint (not dryRunOutput) is what renders this during --dry-run.
	if err := p.runOrRenderHookPoint("Run post_bump hooks", p.cfg.PostBumpHooks, p.hookVars(result)); err != nil {
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
	// changelog is tied to origin, so it resolves links from the explicit remote, the resolved
	// forge, or the ambient CI host (ADR-0022 / ADR-0043).
	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		if err := p.runHookPointStep("Run pre_changelog hooks", p.cfg.PreChangelogHooks, p.hookVars(result)); err != nil {
			return err
		}

		changelogCtx := p.changelogLinkContext()
		if err := p.runStep("Generate changelog", func() (string, []string, error) {
			if _, err := p.cfg.Changelog.Generate(result.Tag, changelogCtx); err != nil {
				return "", nil, fmt.Errorf("generating changelog: %w", err)
			}
			detail, subs := changelogGenResult(p.cfg.Changelog)
			return detail, subs, nil
		}); err != nil {
			return err
		}

		// Step 3: Commit changelog (conditional).
		if p.cfg.Commit || p.cfg.Tag {
			file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)
			var committed bool
			if err := p.runStep("Commit changelog", func() (string, []string, error) {
				var cerr error
				committed, cerr = p.git.commitChangelog(file, commitMessage(p.cfg.CommitMessage, result.Version), !p.cfg.NoPush)
				if cerr != nil {
					return "", nil, fmt.Errorf("committing changelog: %w", cerr)
				}
				return "", nil, nil
			}); err != nil {
				return err
			}
			if !committed {
				warnNothingToCommit(p.out, file)
			}
		}
	}

	// Step 4+5: Tag the commit (conditional).
	if p.cfg.Tag {
		if err := p.runHookPointStep("Run pre_tag hooks", p.cfg.PreTagHooks, p.hookVars(result)); err != nil {
			return err
		}

		if err := p.runStep(fmt.Sprintf("Create tag %s", result.Tag), func() (string, []string, error) {
			if err := p.git.tag(result.Tag, commitMessage(p.cfg.CommitMessage, result.Version), p.cfg.AnnotatedTags, p.cfg.SignTags); err != nil {
				return "", nil, fmt.Errorf("git tag: %w", err)
			}
			return "", nil, nil
		}); err != nil {
			return err
		}

		if !p.cfg.NoPush {
			if err := p.runStep("Push tag", func() (string, []string, error) {
				if err := p.git.pushTag(result.Tag); err != nil {
					return "", nil, err
				}
				return "", nil, nil
			}); err != nil {
				return err
			}
		}

		// post_tag fires whether or not the tag was pushed — NoPush changes what "the tag
		// operation completed" means, not whether it happened.
		if err := p.runHookPointStep("Run post_tag hooks", p.cfg.PostTagHooks, p.hookVars(result)); err != nil {
			return err
		}
	}

	p.printSummary(result)
	return nil
}

// dryRunOutput reports what would happen without performing any mutations.
// When a reporter is set it emits one step per action with [dry-run] result
// prefixes; otherwise it falls back to plain [dry-run] lines.
func (p *ChangelogPipeline) dryRunOutput(result versioning.Result) error {
	vars := p.hookVars(result)

	if p.reporter == nil {
		if !p.cfg.DisableChangelog {
			if p.cfg.Changelog != nil {
				if err := p.printDryRunHookLinesPlain(p.cfg.PreChangelogHooks, vars); err != nil {
					return err
				}
			}
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
			if err := p.printDryRunHookLinesPlain(p.cfg.PreTagHooks, vars); err != nil {
				return err
			}
			if p.cfg.NoPush {
				_, _ = fmt.Fprintf(p.out, "[dry-run] would tag %s (no push)\n", result.Tag)
			} else {
				_, _ = fmt.Fprintf(p.out, "[dry-run] would tag %s and push\n", result.Tag)
			}
			if err := p.printDryRunHookLinesPlain(p.cfg.PostTagHooks, vars); err != nil {
				return err
			}
		}
		return nil
	}

	// Reporter path: emit one informational step per would-be action. post_bump is handled at
	// its Run() call site (runOrRenderHookPoint), not here.
	file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)

	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		if err := p.dryRunHookStep("Run pre_changelog hooks", p.cfg.PreChangelogHooks, vars); err != nil {
			return err
		}
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
		if err := p.dryRunHookStep("Run pre_tag hooks", p.cfg.PreTagHooks, vars); err != nil {
			return err
		}
		_ = p.runStep(fmt.Sprintf("Create tag %s", result.Tag), func() (string, []string, error) {
			return "[dry-run] would tag", nil, nil
		})
		if !p.cfg.NoPush {
			_ = p.runStep("Push tag", func() (string, []string, error) {
				return fmt.Sprintf("[dry-run] would push %s", result.Tag), nil, nil
			})
		}
		if err := p.dryRunHookStep("Run post_tag hooks", p.cfg.PostTagHooks, vars); err != nil {
			return err
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
			file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)
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
