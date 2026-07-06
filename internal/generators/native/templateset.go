package native

import (
	"fmt"
	"os"
	"sort"
	"text/template"
)

// buildTemplateSet parses the built-in blocks, then user inline snippets, then the optional
// full template file — in precedence order (later parses redefine earlier blocks). The result
// is a template set ready to ExecuteTemplate by root-block name (ADR-0037). A snippet or file
// that fails to parse aborts with an error naming the offending block key / path.
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
		if _, err := ts.Parse(fmt.Sprintf("{{ define %q }}%s{{ end }}", k, snippets[k])); err != nil {
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
