package native

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
)

// nullBlockBody replaces a snippet that should render as nothing. Go's text/template.Parse
// documents that a {{define}} body containing only white space and comments is treated as empty
// and will not replace an existing non-empty template — so encoding "nulled" as literal empty or
// whitespace text silently keeps the built-in body. A real (if inert) action node defeats that
// check while still producing no output.
const nullBlockBody = "{{if false}}{{end}}"

// buildTemplateSet parses the built-in blocks, then user inline snippets, then the optional
// full template file — in precedence order (later parses redefine earlier blocks). The result
// is a template set ready to ExecuteTemplate by root-block name (ADR-0037). A snippet or file
// that fails to parse aborts with an error naming the offending block key / path. An
// empty-or-whitespace-only snippet nulls the block (renders nothing) for every block name, not
// just title/subtitle.
func buildTemplateSet(builtin string, snippets map[string]string, filePath string) (*template.Template, error) {
	ts := template.New("native").Funcs(templateFuncs())
	if _, err := ts.Parse(builtin); err != nil {
		return nil, fmt.Errorf("parsing built-in template: %w", err)
	}
	// Inline snippets: each defines one block. Order the keys so a parse error is deterministic.
	keys := make([]string, 0, len(snippets))
	for k := range snippets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		body := snippets[k]
		if strings.TrimSpace(body) == "" {
			body = nullBlockBody
		}
		if _, err := ts.Parse(fmt.Sprintf("{{ define %q }}%s{{ end }}", k, body)); err != nil {
			return nil, fmt.Errorf("parsing rendering.templates.%s: %w", k, err)
		}
	}
	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading template %q: %w", filePath, err)
		}
		if _, err := ts.Parse(string(b)); err != nil {
			return nil, fmt.Errorf("parsing template %q: %w", filePath, err)
		}
	}
	return ts, nil
}
