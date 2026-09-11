package pipeline

import (
	"fmt"
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

// runHook executes cmd via `sh -c` through r, which should be an interactive runner
// (stdin/stdout/stderr connected to the real terminal, forge's CmdRunner.Interactive
// mode — see gitHelper.interactiveOrRunner) so hook output streams live rather than
// being captured. dir is always the repository root ("" — RunDir's current-dir default).
func runHook(r port.Runner, cmd string) error {
	if _, _, err := r.RunDir("", nil, "sh", "-c", cmd); err != nil {
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
