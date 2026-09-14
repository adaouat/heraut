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
// method. commit.trailers is skipped entirely: unlike every other commit.* key it is a list of
// rules, not a snippet string, so it can't flow through this flat map — Rendering.UnmarshalYAML
// extracts it separately from the same raw node (ADR-0060).
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
				if keyNode.Value == "commit" && subKeyNode.Value == "trailers" {
					continue
				}
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

// UnmarshalYAML decodes Rendering's excludes/templates fields, then carves
// rendering.templates.commit.trailers out of the same raw templates node into Commit (ADR-0060)
// — see the type's doc comment for why. A custom Unmarshaler bypasses the outer decoder's
// KnownFields strictness for its own node (forgeconfig.Decode's strict mode does not propagate
// into a type's own UnmarshalYAML), so this reimplements "unknown field" rejection for
// Rendering's own two keys itself, rather than silently accepting a typo.
func (r *Rendering) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("rendering: expected a mapping, got %v", value.Tag)
	}
	var templatesNode *yaml.Node
	for i := 0; i < len(value.Content); i += 2 {
		keyNode, valNode := value.Content[i], value.Content[i+1]
		switch keyNode.Value {
		case "excludes":
			if err := valNode.Decode(&r.Excludes); err != nil {
				return fmt.Errorf("rendering.excludes: %w", err)
			}
		case "templates":
			templatesNode = valNode
			if err := valNode.Decode(&r.Templates); err != nil {
				return err
			}
		default:
			return fmt.Errorf("line %d: field %q not found in type config.Rendering", keyNode.Line, keyNode.Value)
		}
	}
	trailers, err := extractCommitTrailers(templatesNode)
	if err != nil {
		return err
	}
	if len(trailers) > 0 {
		r.Commit = &RenderingCommit{Trailers: trailers}
	}
	return nil
}

// extractCommitTrailers finds rendering.templates.commit.trailers in the raw templates node, if
// present, and decodes it as a []FooterRule (ADR-0060). templatesNode may be nil (rendering.
// templates unset) or lack a commit object — both return (nil, nil).
func extractCommitTrailers(templatesNode *yaml.Node) ([]FooterRule, error) {
	if templatesNode == nil || templatesNode.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i < len(templatesNode.Content); i += 2 {
		if templatesNode.Content[i].Value != "commit" {
			continue
		}
		commitNode := templatesNode.Content[i+1]
		if commitNode.Kind != yaml.MappingNode {
			return nil, nil
		}
		for j := 0; j < len(commitNode.Content); j += 2 {
			if commitNode.Content[j].Value != "trailers" {
				continue
			}
			var trailers []FooterRule
			if err := commitNode.Content[j+1].Decode(&trailers); err != nil {
				return nil, fmt.Errorf("rendering.templates.commit.trailers: %w", err)
			}
			return trailers, nil
		}
		return nil, nil
	}
	return nil, nil
}
