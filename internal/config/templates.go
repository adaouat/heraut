package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// TemplateOverrides is rendering.templates' value type: user overrides for native's overridable
// template blocks (ADR-0037/ADR-0048/ADR-0049), keyed by the block name they replace. Since
// ADR-0059, the release- and commit-cadence blocks are namespaced in YAML (release: {section:
// ..., ...}, commit: {message: ..., ...}) but flatten to dotted keys here (release.section,
// commit.message, ...) — every other layer (mergeRendering's deep-merge, effectiveTemplates,
// native's buildTemplateSet) still deals in a flat map[string]string, unaware the YAML surface
// is nested.
type TemplateOverrides map[string]string

// namespacedTemplateKeys are the two top-level rendering.templates keys that nest into their own
// YAML object (ADR-0059) instead of holding a template snippet string directly.
var namespacedTemplateKeys = map[string]bool{"release": true, "commit": true}

// UnmarshalYAML flattens rendering.templates' release:/commit: namespaces into dotted keys. Any
// other top-level key decodes as a plain snippet string, exactly as before ADR-0059. A key named
// "release"/"commit" whose value isn't itself a mapping (e.g. a plain string) is not
// special-cased — it decodes as an ordinary flat key instead, since it is not a valid block name
// on its own; validateTemplateSnippets' "unknown template block" error catches it later, not this
// method.
func (t *TemplateOverrides) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("rendering.templates: expected a mapping, got %v", value.Tag)
	}
	out := make(TemplateOverrides, len(value.Content)/2)
	for i := 0; i < len(value.Content); i += 2 {
		keyNode, valNode := value.Content[i], value.Content[i+1]
		if namespacedTemplateKeys[keyNode.Value] && valNode.Kind == yaml.MappingNode {
			for j := 0; j < len(valNode.Content); j += 2 {
				subKeyNode, subValNode := valNode.Content[j], valNode.Content[j+1]
				var snippet string
				if err := subValNode.Decode(&snippet); err != nil {
					return fmt.Errorf("rendering.templates.%s.%s: %w", keyNode.Value, subKeyNode.Value, err)
				}
				out[keyNode.Value+"."+subKeyNode.Value] = snippet
			}
			continue
		}
		var snippet string
		if err := valNode.Decode(&snippet); err != nil {
			return fmt.Errorf("rendering.templates.%s: %w", keyNode.Value, err)
		}
		out[keyNode.Value] = snippet
	}
	*t = out
	return nil
}
