package ui

import (
	"fmt"
	"io"
	"strings"

	forgeui "github.com/adaouat/forge/ui"
)

// The status helpers and color detection now live in forge; these thin wrappers
// keep heraut's internal/ui call sites (cmd, step, progress) stable.

// Success returns "✓ <msg>" styled green when w supports color, plain otherwise.
func Success(w io.Writer, msg string) string { return forgeui.Success(w, msg) }

// Err returns "✗ <msg>" styled red when w supports color, plain otherwise.
func Err(w io.Writer, msg string) string { return forgeui.Err(w, msg) }

// Warn returns "! <msg>" styled yellow when w supports color, plain otherwise.
func Warn(w io.Writer, msg string) string { return forgeui.Warn(w, msg) }

// WarnLines writes msg to w as a warning: the first line through Warn, every following line
// verbatim (already indented by the caller).
func WarnLines(w io.Writer, msg string) {
	first, rest, hasRest := strings.Cut(msg, "\n")
	_, _ = fmt.Fprintln(w, Warn(w, first))
	if hasRest {
		_, _ = fmt.Fprintln(w, rest)
	}
}

// Info returns "  <msg>" dimmed when w supports color, plain otherwise.
func Info(w io.Writer, msg string) string { return forgeui.Info(w, msg) }

// Header writes a bold section title to w, surrounded by blank lines.
func Header(w io.Writer, title string) { forgeui.Header(w, title) }

// IsTTY reports whether w is an interactive terminal.
func IsTTY(w io.Writer) bool { return forgeui.IsTTY(w) }
