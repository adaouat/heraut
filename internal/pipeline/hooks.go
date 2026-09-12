package pipeline

import (
	"fmt"
	"runtime"
	"strings"
	"text/template"

	"github.com/adaouat/heraut/internal/port"
)

// hookVars are the Go text/template fields available in hook command strings (ADR-0053).
// Platform is set only for pre_release/post_release; empty at every other point.
type hookVars struct {
	Version     string
	Tag         string
	PreviousTag string
	Platform    string
}

// renderHookCmd renders tmplStr (a hook command string) as a Go text/template against vars.
// vars is a struct, so text/template already errors on an unknown field (e.g. {{ .Typo }})
// without needing the missingkey option, which only affects map lookups.
func renderHookCmd(tmplStr string, vars hookVars) (string, error) {
	tmpl, err := template.New("hook").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing hook command %q: %w", tmplStr, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("rendering hook command %q: %w", tmplStr, err)
	}
	return buf.String(), nil
}

// renderHookCmds renders each command in cmds against vars, in order, stopping at the first
// render error.
func renderHookCmds(cmds []string, vars hookVars) ([]string, error) {
	rendered := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		r, err := renderHookCmd(cmd, vars)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, r)
	}
	return rendered, nil
}

// shouldRunHooks reports whether a hook point should actually execute: never during --dry-run
// (hooks are arbitrary shell commands — dry-run must never execute anything for real) or when
// --no-hooks/NoHooks is set, and only when at least one command is configured.
func shouldRunHooks(dryRun, noHooks bool, cmds []string) bool {
	return !dryRun && !noHooks && len(cmds) > 0
}

// runHookPoint renders cmds against vars and executes them in order via r, stopping at the
// first render or execution error.
func runHookPoint(r port.Runner, cmds []string, vars hookVars) error {
	rendered, err := renderHookCmds(cmds, vars)
	if err != nil {
		return err
	}
	return runHooks(r, rendered)
}

// dryRunHookLines renders cmds against vars into "[dry-run] would run: <cmd>" lines (ADR-0053,
// T271) — the rendered command, never the raw template, matching how dry-run already shows real
// resolved tag names elsewhere in the pipeline. Returns nil, nil when cmds is empty. A render
// error is returned rather than swallowed: dry-run promises to show what would happen, and a
// broken hook template is real, actionable information the caller should surface (and, for the
// reporter path, abort on) rather than a run that only fails once it's no longer a dry one.
func dryRunHookLines(cmds []string, vars hookVars) ([]string, error) {
	if len(cmds) == 0 {
		return nil, nil
	}
	rendered, err := renderHookCmds(cmds, vars)
	if err != nil {
		return nil, err
	}
	lines := make([]string, len(rendered))
	for i, cmd := range rendered {
		lines[i] = "[dry-run] would run: " + cmd
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

// runHooks executes cmds in order via runHook, stopping at the first failure.
func runHooks(r port.Runner, cmds []string) error {
	for _, cmd := range cmds {
		if err := runHook(r, cmd); err != nil {
			return err
		}
	}
	return nil
}
