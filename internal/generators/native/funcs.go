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
		"indent":     func(n int, s string) string { return strings.Repeat(" ", n) + s },
		"trim":       strings.TrimSpace,
	}
}
