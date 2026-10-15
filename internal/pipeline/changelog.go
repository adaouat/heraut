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
	// PreRelease marks that the run publishes a SemVer pre-release; set together with
	// DisableChangelog so the skip message says why.
	PreRelease bool
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
	// Env is the active --env value, exposed to hook commands as {{ .Env }} (ADR-0055). Empty
	// when the run isn't targeting an environment.
	Env string
	// PostBumpHooks run immediately after the next version is resolved. Empty = no hooks.
	PostBumpHooks []HookStep
	// PreChangelogHooks run before changelog generation. Empty = no hooks.
	PreChangelogHooks []HookStep
	// PreTagHooks run before the local git tag is created. Empty = no hooks.
	PreTagHooks []HookStep
	// PostTagHooks run after the tag is pushed to origin. Empty = no hooks.
	PostTagHooks []HookStep
}

// ChangelogPipeline executes the changelog-only flow.
type ChangelogPipeline struct {
	git      gitHelper
	resolver versioning.Resolver
	cfg      *ChangelogConfig
	out      io.Writer
	dryRun   bool
	reporter ui.StepFn
	// committed records whether this run created the changelog commit, so the summary does not
	// claim a commit when nothing was staged (T360).
	committed bool
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
// (ADR-0053). Platform is always empty here — this pipeline never publishes. Env (ADR-0055)
// comes from p.cfg.Env regardless of hook point.
func (p *ChangelogPipeline) hookVars(result versioning.Result) hookVars {
	return hookVars{Version: result.Version, Tag: result.Tag, PreviousTag: result.CurrentTag, Env: p.cfg.Env}
}

// runner returns the step executor configured like p.
func (p *ChangelogPipeline) runner() stepRunner {
	return stepRunner{dryRun: p.dryRun, reporter: p.reporter, out: p.out}
}

// runStep reports and runs one step (see stepRunner.runStep).
func (p *ChangelogPipeline) runStep(name string, fn func() (string, []string, error)) error {
	return p.runner().runStep(name, fn)
}

// changelogFlow builds the steps of a changelog run for a resolved version. Nothing runs here.
// The prelude is the post_bump hooks, which fire on every resolve (ADR-0053) — including the
// DisableChangelog run without --tag, which stops right after them; the body is the rest.
func (p *ChangelogPipeline) changelogFlow(result versioning.Result) (prelude, body []releaseStep) {
	vars := p.hookVars(result)
	// stage collects the files hooks asked to commit with the changelog (ADR-0061): post_bump's,
	// then pre_changelog's.
	var stage []string

	if s, ok := newHookStep(p.cfg.NoHooks, &p.git, "post_bump", p.cfg.PostBumpHooks, vars, &stage); ok {
		prelude = append(prelude, s)
	}

	// The committed changelog is tied to origin, so it resolves links from the explicit remote, the
	// resolved forge, or the ambient CI host (ADR-0022 / ADR-0043).
	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)
		if s, ok := newHookStep(p.cfg.NoHooks, &p.git, "pre_changelog", p.cfg.PreChangelogHooks, vars, &stage); ok {
			body = append(body, s)
		}
		body = append(body, releaseStep{
			name: "Generate changelog",
			exec: func() (string, []string, error) {
				if _, err := p.cfg.Changelog.Generate(result.Tag, p.changelogLinkContext()); err != nil {
					return "", nil, fmt.Errorf("generating changelog: %w", err)
				}
				detail, subs := changelogGenResult(p.cfg.Changelog)
				return detail, subs, nil
			},
			dry:   func() (string, []string, error) { return "[dry-run] would write " + file, nil, nil },
			plain: func() ([]string, error) { return []string{"[dry-run] would generate changelog for " + result.Tag}, nil },
		})
		if p.cfg.Commit || p.cfg.Tag {
			body = append(body, releaseStep{
				name: "Commit changelog",
				exec: func() (string, []string, error) {
					committed, err := p.git.commitChangelog(append([]string{file}, stage...), commitMessage(p.cfg.CommitMessage, result.Version), !p.cfg.NoPush)
					if err != nil {
						return "", nil, fmt.Errorf("committing changelog: %w", err)
					}
					p.committed = committed
					return "", nil, nil
				},
				after: func() {
					if !p.committed {
						warnNothingToCommit(p.out, file)
					}
				},
				dry: func() (string, []string, error) {
					if p.cfg.NoPush {
						return "[dry-run] would commit (no push)", nil, nil
					}
					return "[dry-run] would commit and push", nil, nil
				},
				plain: func() ([]string, error) {
					if p.cfg.NoPush {
						return []string{"[dry-run] would commit (no push)"}, nil
					}
					return []string{"[dry-run] would commit → push"}, nil
				},
			})
		}
	}

	if p.cfg.Tag {
		if s, ok := newHookStep(p.cfg.NoHooks, &p.git, "pre_tag", p.cfg.PreTagHooks, vars, nil); ok {
			body = append(body, s)
		}
		body = append(body, releaseStep{
			name: fmt.Sprintf("Create tag %s", result.Tag),
			exec: func() (string, []string, error) {
				if err := p.git.tag(result.Tag, commitMessage(p.cfg.CommitMessage, result.Version), p.cfg.AnnotatedTags, p.cfg.SignTags); err != nil {
					return "", nil, fmt.Errorf("git tag: %w", err)
				}
				return "", nil, nil
			},
			dry: func() (string, []string, error) { return "[dry-run] would tag", nil, nil },
			plain: func() ([]string, error) {
				if p.cfg.NoPush {
					return []string{fmt.Sprintf("[dry-run] would tag %s (no push)", result.Tag)}, nil
				}
				return []string{fmt.Sprintf("[dry-run] would tag %s and push", result.Tag)}, nil
			},
		})
		if !p.cfg.NoPush {
			body = append(body, releaseStep{
				name: "Push tag",
				exec: func() (string, []string, error) { return "", nil, p.git.pushTag(result.Tag) },
				dry: func() (string, []string, error) {
					return fmt.Sprintf("[dry-run] would push %s", result.Tag), nil, nil
				},
			})
		}
		// post_tag fires whether or not the tag was pushed — NoPush changes what "the tag operation
		// completed" means, not whether it happened.
		if s, ok := newHookStep(p.cfg.NoHooks, &p.git, "post_tag", p.cfg.PostTagHooks, vars, nil); ok {
			body = append(body, s)
		}
	}
	return prelude, body
}

// Run executes the changelog sequence:
//  1. Resolve version, then the post_bump hooks
//  2. If DisableChangelog: print info and return (unless --tag)
//  3. If Changelog configured: generate changelog
//  4. If Commit or Tag (and Changelog configured): git add → git commit → git push
//  5. If Tag: git tag → git push origin <tag>
//
// When NoPush is set, both pushes (HEAD and the tag) are skipped — the commit and
// tag are created locally only. With a dry-run Pipeline the same steps are rendered instead.
func (p *ChangelogPipeline) Run() error {
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
	printResolveWarnings(p.out, result.Warnings)

	prelude, body := p.changelogFlow(result)
	if err := p.runner().runSteps(prelude, nil); err != nil {
		return err
	}

	if p.cfg.DisableChangelog {
		switch {
		case p.cfg.PreRelease && p.reporter != nil:
			_, _ = fmt.Fprintln(p.out, ui.Warn(p.out, "pre-release: CHANGELOG.md not updated"))
		case p.cfg.PreRelease:
			_, _ = fmt.Fprintf(p.out, "pre-release %s: CHANGELOG.md not updated\n", result.Tag)
		case p.reporter != nil:
			_, _ = fmt.Fprintln(p.out, ui.Warn(p.out, "changelog disabled"))
		default:
			_, _ = fmt.Fprintf(p.out, "changelog disabled for %s\n", result.Tag)
		}
		if !p.cfg.Tag {
			return nil
		}
		// Tag is true: skip changelog steps but proceed to tag.
	}

	if err := p.runner().runSteps(body, nil); err != nil {
		return err
	}
	if p.dryRun {
		return nil
	}

	p.printSummary(result)
	return nil
}

// printSummary writes the post-run summary to p.out.
// With a reporter it uses the styled block; without one it keeps the original
// single-line format so existing plain callers are unaffected.
func (p *ChangelogPipeline) printSummary(result versioning.Result) {
	if p.cfg.DisableChangelog {
		p.printTagOnlySummary(result)
		return
	}
	if p.reporter != nil {
		_, _ = fmt.Fprintf(p.out, "\nChangelog updated for %s\n", result.Tag)
		if (p.cfg.Commit || p.cfg.Tag) && p.cfg.Changelog != nil {
			file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)
			switch {
			case !p.committed && p.cfg.NoPush:
				_, _ = fmt.Fprintf(p.out, "  %s unchanged, nothing committed\n", file)
			case !p.committed:
				_, _ = fmt.Fprintf(p.out, "  %s unchanged, HEAD pushed\n", file)
			case p.cfg.NoPush:
				_, _ = fmt.Fprintf(p.out, "  %s committed (not pushed)\n", file)
			default:
				_, _ = fmt.Fprintf(p.out, "  %s committed and pushed\n", file)
			}
		}
		return
	}
	_, _ = fmt.Fprintf(p.out, "changelog updated for %s\n", result.Tag)
}

// printTagOnlySummary reports a run whose changelog step was skipped: the only work that can have
// happened is the tag.
func (p *ChangelogPipeline) printTagOnlySummary(result versioning.Result) {
	if !p.cfg.Tag {
		return
	}
	if p.reporter == nil {
		_, _ = fmt.Fprintf(p.out, "tagged %s\n", result.Tag)
		return
	}
	if p.cfg.NoPush {
		_, _ = fmt.Fprintf(p.out, "\nTagged %s (not pushed)\n", result.Tag)
		return
	}
	_, _ = fmt.Fprintf(p.out, "\nTagged %s and pushed\n", result.Tag)
}
