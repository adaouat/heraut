package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// HookStep is one command in a hook point's list (ADR-0053, extended by ADR-0061: object-only,
// stage added). Run is required; Stage is optional and only meaningful under post_bump/
// pre_changelog (internal/config/validator.go enforces the scope — see T296).
type HookStep struct {
	Run   string   `yaml:"run"`
	Stage []string `yaml:"stage,omitempty"`
}

// UnmarshalYAML rejects the plain-string shorthand ADR-0053 originally allowed (ADR-0061): every
// hook entry must be a mapping with at least run. Re-marshals node and decodes it through a
// fresh KnownFields(true) decoder rather than calling node.Decode directly — verified empirically
// that node.Decode alone does NOT honor the outer forge/config.Decode call's KnownFields(true),
// so an unknown field inside a hook entry (e.g. a typo'd `stag:`) would otherwise silently pass
// instead of erroring.
func (h *HookStep) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		return fmt.Errorf("expected a mapping, got a plain string %q — wrap it as { run: %q }", node.Value, node.Value)
	}
	type rawHookStep HookStep
	var raw rawHookStep
	b, err := yaml.Marshal(node)
	if err != nil {
		return fmt.Errorf("re-marshaling hook step: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	*h = HookStep(raw)
	return nil
}

// Hooks configures shell commands run at points in the release lifecycle (ADR-0053, ADR-0061).
// post_bump/pre_changelog/pre_tag/post_tag fire in both `heraut release` and
// `heraut changelog --tag`; pre_release/post_release fire only in `heraut release`, once per
// publish target.
type Hooks struct {
	// PostBump runs immediately after the next version is resolved.
	PostBump []HookStep `yaml:"post_bump,omitempty"`
	// PreChangelog runs before changelog generation.
	PreChangelog []HookStep `yaml:"pre_changelog,omitempty"`
	// PreTag runs before the local git tag is created.
	PreTag []HookStep `yaml:"pre_tag,omitempty"`
	// PostTag runs after the tag is pushed to origin.
	PostTag []HookStep `yaml:"post_tag,omitempty"`
	// PreRelease runs before publishing to a platform, once per platform.
	PreRelease []HookStep `yaml:"pre_release,omitempty"`
	// PostRelease runs after publishing to a platform, once per platform.
	PostRelease []HookStep `yaml:"post_release,omitempty"`
}

// PostBumpHooks returns the configured post_bump hook steps, or nil. Nil-safe.
func (c *Config) PostBumpHooks() []HookStep {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PostBump
}

// PreChangelogHooks returns the configured pre_changelog hook steps, or nil. Nil-safe.
func (c *Config) PreChangelogHooks() []HookStep {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PreChangelog
}

// PreTagHooks returns the configured pre_tag hook steps, or nil. Nil-safe.
func (c *Config) PreTagHooks() []HookStep {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PreTag
}

// PostTagHooks returns the configured post_tag hook steps, or nil. Nil-safe.
func (c *Config) PostTagHooks() []HookStep {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PostTag
}

// PreReleaseHooks returns the configured pre_release hook steps, or nil. Nil-safe.
func (c *Config) PreReleaseHooks() []HookStep {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PreRelease
}

// PostReleaseHooks returns the configured post_release hook steps, or nil. Nil-safe.
func (c *Config) PostReleaseHooks() []HookStep {
	if c.Hooks == nil {
		return nil
	}
	return c.Hooks.PostRelease
}
