package ui

import (
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// Success returns "✓ <msg>" styled green when w supports color, plain otherwise.
func Success(w io.Writer, msg string) string {
	if !hasColor(w) {
		return "✓ " + msg
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E")).Bold(true).Render("✓") + " " + msg
}

// Err returns "✗ <msg>" styled red when w supports color, plain otherwise.
func Err(w io.Writer, msg string) string {
	if !hasColor(w) {
		return "✗ " + msg
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true).Render("✗") + " " + msg
}

// Warn returns "! <msg>" styled yellow when w supports color, plain otherwise.
// Use for skipped checks and non-critical notices.
func Warn(w io.Writer, msg string) string {
	if !hasColor(w) {
		return "! " + msg
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Render("!") + " " + msg
}

// Info returns "  <msg>" dimmed when w supports color, plain otherwise.
// Use for hints and supplementary context below a primary status line.
func Info(w io.Writer, msg string) string {
	if !hasColor(w) {
		return "  " + msg
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("  " + msg)
}

// hasColor reports whether w supports ANSI color output.
// It delegates to colorprofile.Detect which honours NO_COLOR, TERM=dumb, and
// whether w is a terminal.
func hasColor(w io.Writer) bool {
	return colorprofile.Detect(w, os.Environ()) >= colorprofile.ANSI
}
