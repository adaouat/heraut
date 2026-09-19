package pipeline

import (
	"fmt"
	"runtime"
	"strings"
	"text/template"

	"github.com/adaouat/heraut/internal/port"
)

// hookVars are the Go text/template fields available in hook command strings (ADR-0053).
// Platform is set only for pre_release/post_release; empty at every other point. Env (ADR-0055)
// is the active --env value at every point, empty when the run isn't targeting one.
type hookVars struct {
	Version     string
	Tag         string
	PreviousTag string
	Platform    string
	Env         string
}

// HookStep is one command in a hook point's list, translated from config.HookStep by
// internal/app (ADR-0053, ADR-0061) — this package never imports internal/config.
type HookStep struct {
	Run   string
	Stage []string
}

// renderedHookStep is a HookStep after template substitution.
type renderedHookStep struct {
	Run   string
	Stage []string
}

// renderHookCmd renders tmplStr as a Go text/template against vars. label identifies what
// tmplStr is in any error text — "hook command" for a Run string, "stage pattern" for a
// Stage entry — so a broken template's error names the field a user actually wrote instead
// of always saying "hook command" regardless of which one failed. vars is a struct, so
// text/template already errors on an unknown field (e.g. {{ .Typo }}) without needing the
// missingkey option, which only affects map lookups.
func renderHookCmd(tmplStr string, vars hookVars, label string) (string, error) {
	tmpl, err := template.New("hook").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing %s %q: %w", label, tmplStr, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("rendering %s %q: %w", label, tmplStr, err)
	}
	return buf.String(), nil
}

// renderHookStep renders step's Run and every Stage entry against vars.
func renderHookStep(step HookStep, vars hookVars) (renderedHookStep, error) {
	run, err := renderHookCmd(step.Run, vars, "hook command")
	if err != nil {
		return renderedHookStep{}, err
	}
	var stage []string
	for _, s := range step.Stage {
		rendered, err := renderHookCmd(s, vars, "stage pattern")
		if err != nil {
			return renderedHookStep{}, err
		}
		stage = append(stage, rendered)
	}
	return renderedHookStep{Run: run, Stage: stage}, nil
}

// renderHookSteps renders each step in order, stopping at the first render error.
func renderHookSteps(steps []HookStep, vars hookVars) ([]renderedHookStep, error) {
	rendered := make([]renderedHookStep, 0, len(steps))
	for _, s := range steps {
		r, err := renderHookStep(s, vars)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, r)
	}
	return rendered, nil
}

// shouldRunHooks reports whether a hook point should actually execute: never during --dry-run
// (hooks are arbitrary shell commands — dry-run must never execute anything for real) or when
// --no-hooks/NoHooks is set, and only when at least one step is configured.
func shouldRunHooks(dryRun, noHooks bool, steps []HookStep) bool {
	return !dryRun && !noHooks && len(steps) > 0
}

// runHookPointSteps renders steps against vars and executes each rendered Run command in order
// via r, stopping at the first render or execution error. Returns the rendered steps so callers
// that need declared Stage patterns (post_bump/pre_changelog — T298) can collect them; the other
// four points simply discard the return value (config validation, T296, already guarantees their
// Stage is always empty).
func runHookPointSteps(r port.Runner, steps []HookStep, vars hookVars) ([]renderedHookStep, error) {
	rendered, err := renderHookSteps(steps, vars)
	if err != nil {
		return nil, err
	}
	for _, s := range rendered {
		if err := runHook(r, s.Run); err != nil {
			return nil, err
		}
	}
	return rendered, nil
}

// stagePatterns flattens every Stage pattern across rendered steps, in order.
func stagePatterns(steps []renderedHookStep) []string {
	var patterns []string
	for _, s := range steps {
		patterns = append(patterns, s.Stage...)
	}
	return patterns
}

// dryRunHookLines renders steps against vars into "[dry-run] would run: <cmd>" lines (ADR-0053,
// T271), plus one "[dry-run] would stage: <pattern>" line per non-empty Stage entry (Design §5,
// 2026-09-17) — the rendered command/pattern, never the raw template, matching how dry-run
// already shows real resolved tag names elsewhere in the pipeline. Returns nil, nil when steps is
// empty. A render error is returned rather than swallowed: dry-run promises to show what would
// happen, and a broken hook template is real, actionable information the caller should surface
// (and, for the reporter path, abort on) rather than a run that only fails once it's no longer a
// dry one.
func dryRunHookLines(steps []HookStep, vars hookVars) ([]string, error) {
	if len(steps) == 0 {
		return nil, nil
	}
	rendered, err := renderHookSteps(steps, vars)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, s := range rendered {
		lines = append(lines, "[dry-run] would run: "+s.Run)
		for _, pattern := range s.Stage {
			lines = append(lines, "[dry-run] would stage: "+pattern)
		}
	}
	return lines, nil
}

// hookFailureError marks an error as originating from a pre_release/post_release hook for one
// publish target, so the per-platform publish loop (release.go) can isolate it — skip or warn
// for that platform only and continue — instead of aborting the whole release the way a real
// publish failure still does (ADR-0053's one deliberate asymmetry).
type hookFailureError struct {
	platform string
	err      error
}

func (e *hookFailureError) Error() string {
	return fmt.Sprintf("platform %s: hook failed: %v", e.platform, e.err)
}

func (e *hookFailureError) Unwrap() error { return e.err }

// hookShellInvocation returns the binary and args used to run cmd through a shell on
// goos (ADR-0054). POSIX systems get "sh -c" — its exit code is the invoked command's own,
// unchanged from ADR-0053. Windows gets "cmd /D /C": /C runs cmd and re-exits with its
// ERRORLEVEL the same way sh -c does (powershell's $LASTEXITCODE does not propagate to its
// own exit code by default, which would silently break hook failure detection); /D
// disables cmd's AutoRun registry hook, the cmd.exe analogue of sh -c never sourcing an rc
// file. A pure function so both branches are unit-tested deterministically regardless of
// the host OS running the test.
func hookShellInvocation(goos, cmd string) (name string, args []string) {
	if goos == "windows" {
		return "cmd", []string{"/D", "/C", cmd}
	}
	return "sh", []string{"-c", cmd}
}

// runHook executes cmd via a shell (ADR-0054) through r, which should be an interactive
// runner (stdin/stdout/stderr connected to the real terminal, forge's CmdRunner.Interactive
// mode — see gitHelper.interactiveOrRunner) so hook output streams live rather than
// being captured. dir is always the repository root ("" — RunDir's current-dir default).
func runHook(r port.Runner, cmd string) error {
	name, args := hookShellInvocation(runtime.GOOS, cmd)
	if _, _, err := r.RunDir("", nil, name, args...); err != nil {
		return fmt.Errorf("hook %q: %w", cmd, err)
	}
	return nil
}
