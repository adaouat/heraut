package pipeline

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

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

// WithInteractiveRunner sets the runner used for commands that may need a real terminal — a GPG
// pinentry prompt during `git commit`/a signed tag (T260) — and returns p for chaining. When
// unset, those commands fall back to the regular runner, exactly as before this option existed.
func (p *Pipeline) WithInteractiveRunner(r port.Runner) *Pipeline {
	p.git.interactiveRunner = r
	return p
}

// hookVars builds the template variables available to hook commands from a resolved result
// (ADR-0053). Platform is left empty here — only pre_release/post_release (T270) set it.
func (p *Pipeline) hookVars(result versioning.Result) hookVars {
	return hookVars{Version: result.Version, Tag: result.Tag, PreviousTag: result.CurrentTag}
}

// runHookPointStep renders and executes cmds (one hook point's configured commands) as a
// reported step named name, skipping entirely — no step reported — when shouldRunHooks says
// this point shouldn't run (dry-run, --no-hooks, or nothing configured).
func (p *Pipeline) runHookPointStep(name string, cmds []string, vars hookVars) error {
	if !shouldRunHooks(p.dryRun, p.cfg.NoHooks, cmds) {
		return nil
	}
	return p.runStep(name, func() (string, []string, error) {
		return "", nil, runHookPoint(p.git.interactiveOrRunner(), cmds, vars)
	})
}

// runOrRenderHookPoint executes cmds for real, or — during --dry-run — renders what would run
// (as a reporter step or a plain line, matching whichever output mode is active) without ever
// executing anything. Used at call sites reached before dryRunOutput's own dry-run rendering —
// currently only post_bump, since it fires unconditionally on resolve (ADR-0053).
func (p *Pipeline) runOrRenderHookPoint(name string, cmds []string, vars hookVars) error {
	if !p.dryRun {
		return p.runHookPointStep(name, cmds, vars)
	}
	if p.reporter != nil {
		return p.dryRunHookStep(name, cmds, vars)
	}
	return p.printDryRunHookLinesPlain(cmds, vars)
}

// dryRunHookLinesOrNil renders cmds into "[dry-run] would run: <cmd>" lines (T271), or nil, nil
// when NoHooks is set or nothing is configured — the dry-run counterpart to shouldRunHooks.
func (p *Pipeline) dryRunHookLinesOrNil(cmds []string, vars hookVars) ([]string, error) {
	if p.cfg.NoHooks {
		return nil, nil
	}
	return dryRunHookLines(cmds, vars)
}

// dryRunHookStep reports a would-run hook point as its own step, named identically to the real
// step runHookPointStep would report, so dry-run's step sequence matches a real run's exactly.
// A render error aborts the dry-run (propagated, not discarded) — dry-run promises to show what
// would happen, and a broken hook template is real, actionable information.
func (p *Pipeline) dryRunHookStep(name string, cmds []string, vars hookVars) error {
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
func (p *Pipeline) printDryRunHookLinesPlain(cmds []string, vars hookVars) error {
	lines, err := p.dryRunHookLinesOrNil(cmds, vars)
	if err != nil {
		return err
	}
	for _, l := range lines {
		_, _ = fmt.Fprintln(p.out, l)
	}
	return nil
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
//  3. git tag → git push origin <tag>
//  4. (if notes configured, single platform only) Generate release notes
//  5. For each platform: (if notes configured + multi-platform) regenerate notes with the
//     platform's LinkContext, then CreateRelease
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

	// post_bump hooks fire on every resolve (ADR-0053) — independent of dry-run/disable-changelog
	// branching below, so this sits right after Step 1 rather than after the dry-run check.
	// runOrRenderHookPoint (not this placement) is what actually prevents execution during
	// --dry-run — it renders instead, since dryRunOutput below never handles post_bump itself.
	if err := p.runOrRenderHookPoint("Run post_bump hooks", p.cfg.PostBumpHooks, p.hookVars(result)); err != nil {
		return err
	}

	if p.dryRun {
		return p.dryRunOutput(result)
	}

	// Step 2+3: Generate and commit changelog (conditional). The committed changelog is
	// singular and tied to origin, so it resolves links from the ambient CI host (ADR-0022).
	// Falls back to the single configured platform context for local/non-CI runs.
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

		file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)
		var committed bool
		if err := p.runStep("Commit changelog", func() (string, []string, error) {
			var cerr error
			committed, cerr = p.git.commitChangelog(file, commitMessage(p.cfg.CommitMessage, result.Version), true)
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

	if err := p.runHookPointStep("Run pre_tag hooks", p.cfg.PreTagHooks, p.hookVars(result)); err != nil {
		return err
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
		if err := p.git.pushTag(result.Tag); err != nil {
			return "", nil, err
		}
		return "", nil, nil
	}); err != nil {
		return err
	}

	if err := p.runHookPointStep("Run post_tag hooks", p.cfg.PostTagHooks, p.hookVars(result)); err != nil {
		return err
	}

	// Steps 6+7: release notes + publish. With a single platform, notes are generated
	// once up front with that platform's resolved link context. With multiple platforms,
	// notes are regenerated inside each publish step with that platform's LinkContext so
	// links carry the right host/path shape (ADR-0021 / ADR-0022).
	notesEnabled := p.cfg.Notes != nil && !p.cfg.DisableNotes
	multiPlatform := len(p.cfg.Platforms) > 1

	var notes string
	if notesEnabled && !multiPlatform {
		lc := p.singlePlatformLinkContext()
		if err := p.runStep("Generate release notes", func() (string, []string, error) {
			var genErr error
			notes, genErr = p.cfg.Notes.Generate(result.Tag, lc)
			if genErr != nil {
				return "", nil, fmt.Errorf("generating release notes: %w", genErr)
			}
			return "", degradedSubs(p.cfg.Notes), nil
		}); err != nil {
			return err
		}
	}

	// hookFailedPlatforms collects platforms skipped or warned about due to a pre_release/
	// post_release hook failure (ADR-0053) — isolated per platform, unlike a real publish
	// failure, which still aborts the whole loop via the plain (unwrapped) errors below.
	var hookFailedPlatforms []string
	for _, plat := range p.cfg.Platforms {
		platVars := p.hookVars(result)
		platVars.Platform = plat.Name()

		err := p.runStep(fmt.Sprintf("Publish to %s", plat.Name()), func() (string, []string, error) {
			if shouldRunHooks(p.dryRun, p.cfg.NoHooks, p.cfg.PreReleaseHooks) {
				if err := runHookPoint(p.git.interactiveOrRunner(), p.cfg.PreReleaseHooks, platVars); err != nil {
					return "", nil, &hookFailureError{platform: plat.Name(), err: err}
				}
			}

			var subs []string
			platNotes := notes
			lc := p.platformLinkContext(plat)
			if notesEnabled && multiPlatform {
				generated, genErr := p.cfg.Notes.Generate(result.Tag, lc)
				if genErr != nil {
					return "", nil, fmt.Errorf("platform %s: generating release notes: %w", plat.Name(), genErr)
				}
				platNotes = generated
				subs = append(subs, "notes generated")
				subs = append(subs, degradedSubs(p.cfg.Notes)...)
			}
			if err := plat.CreateRelease(result.Tag, platNotes); err != nil {
				return "", nil, fmt.Errorf("platform %s: create release: %w", plat.Name(), err)
			}
			if plat.HasAssets() {
				if err := plat.UploadAssets(result.Tag); err != nil {
					return "", nil, fmt.Errorf("platform %s: upload assets: %w", plat.Name(), err)
				}
				subs = append(subs, "assets uploaded")
			}

			if shouldRunHooks(p.dryRun, p.cfg.NoHooks, p.cfg.PostReleaseHooks) {
				if err := runHookPoint(p.git.interactiveOrRunner(), p.cfg.PostReleaseHooks, platVars); err != nil {
					return "", nil, &hookFailureError{platform: plat.Name(), err: err}
				}
			}

			return plat.ReleaseURLFromContext(result.Tag, lc), subs, nil
		})
		if err != nil {
			var hfe *hookFailureError
			if errors.As(err, &hfe) {
				hookFailedPlatforms = append(hookFailedPlatforms, hfe.platform)
				continue
			}
			return err
		}
	}

	if len(hookFailedPlatforms) > 0 {
		return fmt.Errorf("hook failed for platform(s): %s", strings.Join(hookFailedPlatforms, ", "))
	}

	p.printSummary(result)
	return nil
}

// dryRunOutput reports what would happen without performing any mutations.
// When a reporter is set it emits one step per action; otherwise it falls back
// to the plain [dry-run] lines for CI-friendly output.
func (p *Pipeline) dryRunOutput(result versioning.Result) error {
	vars := p.hookVars(result)

	if p.reporter == nil {
		_, _ = fmt.Fprintf(p.out, "[dry-run] would release %s\n", result.Tag)
		if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
			if err := p.printDryRunHookLinesPlain(p.cfg.PreChangelogHooks, vars); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(p.out, "[dry-run] would generate changelog → commit → push\n")
		}
		if err := p.printDryRunHookLinesPlain(p.cfg.PreTagHooks, vars); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(p.out, "[dry-run] would tag %s and push\n", result.Tag)
		if err := p.printDryRunHookLinesPlain(p.cfg.PostTagHooks, vars); err != nil {
			return err
		}
		for _, platform := range p.cfg.Platforms {
			platVars := vars
			platVars.Platform = platform.Name()
			if err := p.printDryRunHookLinesPlain(p.cfg.PreReleaseHooks, platVars); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(p.out, "[dry-run] would publish to %s\n", platform.Name())
			if err := p.printDryRunHookLinesPlain(p.cfg.PostReleaseHooks, platVars); err != nil {
				return err
			}
		}
		return nil
	}

	// Reporter path: emit one informational step per would-be action. post_bump is handled at
	// its Run() call site (runOrRenderHookPoint), not here — it fires before this method is
	// ever reached.
	file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)

	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		if err := p.dryRunHookStep("Run pre_changelog hooks", p.cfg.PreChangelogHooks, vars); err != nil {
			return err
		}
		_ = p.runStep("Generate changelog", func() (string, []string, error) {
			return "[dry-run] would write " + file, nil, nil
		})
		_ = p.runStep("Commit changelog", func() (string, []string, error) {
			return "[dry-run] would commit and push", nil, nil
		})
	}

	if err := p.dryRunHookStep("Run pre_tag hooks", p.cfg.PreTagHooks, vars); err != nil {
		return err
	}

	_ = p.runStep(fmt.Sprintf("Create tag %s", result.Tag), func() (string, []string, error) {
		return "[dry-run] would tag", nil, nil
	})
	_ = p.runStep("Push tag", func() (string, []string, error) {
		return fmt.Sprintf("[dry-run] would push %s", result.Tag), nil, nil
	})

	if err := p.dryRunHookStep("Run post_tag hooks", p.cfg.PostTagHooks, vars); err != nil {
		return err
	}

	notesEnabled := p.cfg.Notes != nil && !p.cfg.DisableNotes
	multiPlatform := len(p.cfg.Platforms) > 1

	if notesEnabled && !multiPlatform {
		_ = p.runStep("Generate release notes", func() (string, []string, error) {
			return "[dry-run] would generate", nil, nil
		})
	}

	for _, plat := range p.cfg.Platforms {
		platVars := vars
		platVars.Platform = plat.Name()
		if err := p.runStep(fmt.Sprintf("Publish to %s", plat.Name()), func() (string, []string, error) {
			var subs []string
			preLines, err := p.dryRunHookLinesOrNil(p.cfg.PreReleaseHooks, platVars)
			if err != nil {
				return "", nil, err
			}
			subs = append(subs, preLines...)
			if notesEnabled && multiPlatform {
				subs = append(subs, "[dry-run] would generate notes")
			}
			if plat.HasAssets() {
				subs = append(subs, "[dry-run] would upload assets")
			}
			postLines, err := p.dryRunHookLinesOrNil(p.cfg.PostReleaseHooks, platVars)
			if err != nil {
				return "", nil, err
			}
			subs = append(subs, postLines...)
			return "[dry-run] would create release", subs, nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// warnNothingToCommit emits an actionable warning, naming the changelog file, when a
// regenerated changelog is byte-identical to the last commit so nothing was staged. It
// writes to w (both reporter and plain modes) so the diagnostic is visible in CI, where
// this most often surfaces (re-run after a partial release, or no changelog-worthy
// commits). The pipeline continues to tag and publish.
func warnNothingToCommit(w io.Writer, file string) {
	_, _ = fmt.Fprintln(w, ui.Warn(w, fmt.Sprintf(
		"%s unchanged — no new entries to commit; skipping commit, continuing to tag and release", file)))
}

// printSummary writes the post-run summary to p.out.
// With a reporter it uses the styled block; without one it keeps the original
// single-line format so existing plain callers are unaffected.
func (p *Pipeline) printSummary(result versioning.Result) {
	if p.reporter != nil {
		_, _ = fmt.Fprintf(p.out, "\nReleased %s\n", result.Tag)
		for _, platform := range p.cfg.Platforms {
			lc := p.platformLinkContext(platform)
			_, _ = fmt.Fprintf(p.out, "  › %-8s %s\n", platform.Name(), platform.ReleaseURLFromContext(result.Tag, lc))
		}
		return
	}
	_, _ = fmt.Fprintf(p.out, "released %s\n", result.Tag)
	for _, platform := range p.cfg.Platforms {
		lc := p.platformLinkContext(platform)
		_, _ = fmt.Fprintf(p.out, "  %s: %s\n", platform.Name(), platform.ReleaseURLFromContext(result.Tag, lc))
	}
}
