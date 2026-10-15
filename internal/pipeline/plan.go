package pipeline

import "fmt"

// PlanStep is one action a real release would take. Action is one of hook, changelog,
// commit_changelog, tag, push_tag, notes or publish; the other fields apply per action.
type PlanStep struct {
	Action     string   `json:"action"`
	Point      string   `json:"point,omitempty"`    // hook: post_bump, pre_changelog, pre_tag, post_tag, pre_release, post_release
	Target     string   `json:"target,omitempty"`   // changelog file, tag, or platform name
	Commands   []string `json:"commands,omitempty"` // hook: the rendered commands
	Stage      []string `json:"stage,omitempty"`    // hook: the rendered stage patterns
	Notes      bool     `json:"notes,omitempty"`    // publish: release notes are generated for it
	Assets     bool     `json:"assets,omitempty"`   // publish: assets are uploaded
	PreRelease bool     `json:"pre_release,omitempty"`
}

// Plan is what Run would do, computed without executing, writing or contacting anything.
type Plan struct {
	Tag        string
	Version    string
	CurrentTag string
	PreRelease bool
	Warnings   []string
	Steps      []PlanStep
}

// Plan resolves the version and lists the actions Run would take, in order, from the same step
// list Run executes. Hooks are rendered, never executed.
func (p *Pipeline) Plan() (Plan, error) {
	result, err := p.resolver.Resolve()
	if err != nil {
		return Plan{}, fmt.Errorf("resolving version: %w", err)
	}
	flow := p.releaseFlow(result)
	plan := Plan{Tag: result.Tag, Version: result.Version, CurrentTag: result.CurrentTag, PreRelease: flow.prerelease, Warnings: result.Warnings}
	for _, s := range flow.steps() {
		if s.plan == nil {
			continue
		}
		entries, err := s.plan()
		if err != nil {
			return Plan{}, err
		}
		plan.Steps = append(plan.Steps, entries...)
	}
	return plan, nil
}
