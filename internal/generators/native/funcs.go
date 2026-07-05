package native

import (
	"strings"
	"text/template"
	"time"
)

// templateFuncs is the small, safe func map exposed to user + built-in templates (ADR-0037).
// No OS / file / network access.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"upperFirst": upperFirst, // existing helper in render.go
		"date":       func(layout string, t time.Time) string { return t.Format(layout) },
		"join":       func(sep string, s []string) string { return strings.Join(s, sep) },
		"list":       func(items ...string) []string { return items },
		"indent":     indentLines,
		"trim":       strings.TrimSpace,
	}
}

// indentLines prefixes every non-empty line of s with n spaces (empty lines are left bare).
// This matches the built-in release-notes body indentation; a single-line "x" becomes "  x".
func indentLines(n int, s string) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}
