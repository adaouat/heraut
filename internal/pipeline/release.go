package pipeline

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
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
// (ADR-0053). Platform is left empty here — only pre_release/post_release (T270) set it. Env
// (ADR-0055) comes from p.cfg.Env regardless of hook point.
func (p *Pipeline) hookVars(result versioning.Result) hookVars {
	return hookVars{Version: result.Version, Tag: result.Tag, PreviousTag: result.CurrentTag, Env: p.cfg.Env}
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

// Run executes the full release sequence, or with a dry-run Pipeline renders it instead. The
// sequence is the step list releaseFlow builds:
//  1. Resolve version, then the post_bump hooks and the asset preflight
//  2. (if changelog configured and not disabled) Generate changelog → git add → git commit → git push
//  3. git tag → git push origin <tag>
//  4. (if notes configured, single platform only) Generate release notes
//  5. For each platform: (if notes configured + multi-platform) regenerate notes with the
//     platform's LinkContext, then CreateRelease, which also attaches any assets — reported as
//     a sub-result when platform.HasAssets()
func (p *Pipeline) Run() error {
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
	printResolveWarnings(p.out, result.Warnings)

	flow := p.releaseFlow(result)
	if err := p.runFlowSteps(flow.prelude, flow); err != nil {
		return err
	}
	if p.dryRun && p.reporter == nil {
		_, _ = fmt.Fprintf(p.out, "[dry-run] would release %s\n", result.Tag)
	}
	if err := p.runFlowSteps(flow.body, flow); err != nil {
		return err
	}
	if p.dryRun {
		return nil
	}

	if len(flow.hookFailed) > 0 {
		return fmt.Errorf("hook failed for platform(s): %s", strings.Join(flow.hookFailed, ", "))
	}

	p.printSummary(result)
	return nil
}

// isPreRelease reports whether version carries SemVer pre-release identifiers (ADR-0064). It is
// a shape check only — a CalVer version can itself parse as a SemVer pre-release (e.g.
// "2026.10.2-0" under a `format: YYYY.MM.SS-PATCH`) — so callers must gate it on the active
// strategy being SemVer-based (pipeline.Config.SemVerStrategy); CalVer has no pre-release concept
// regardless of what this function alone reports.
func isPreRelease(version string) bool {
	v, err := semver.Parse(version)
	if err != nil {
		return false
	}
	return v.IsPreRelease()
}

// warnNothingToCommit emits an actionable warning when `git add` staged nothing this run —
// the changelog file, any hook-declared `stage` files (ADR-0061), or both, were already
// byte-identical to the last commit, so there's no single file to pin the cause on. It
// writes to w (both reporter and plain modes) so the diagnostic is visible in CI, where
// this most often surfaces (re-run after a partial release, or no changelog-worthy
// commits). The pipeline continues to tag and publish.
func warnNothingToCommit(w io.Writer, file string) {
	_, _ = fmt.Fprintln(w, ui.Warn(w, fmt.Sprintf(
		"nothing to commit — %s and any hook-staged files are unchanged; skipping commit, continuing to tag and release", file)))
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
