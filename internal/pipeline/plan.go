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

// Plan resolves the version and lists the actions Run would take, in order. Hooks are rendered,
// never executed.
func (p *Pipeline) Plan() (Plan, error) {
	result, err := p.resolver.Resolve()
	if err != nil {
		return Plan{}, fmt.Errorf("resolving version: %w", err)
	}
	vars := p.hookVars(result)
	prerelease := p.cfg.SemVerStrategy && isPreRelease(result.Version)
	plan := Plan{Tag: result.Tag, Version: result.Version, CurrentTag: result.CurrentTag, PreRelease: prerelease, Warnings: result.Warnings}

	hook := func(point, target string, steps []HookStep, v hookVars) error {
		if p.cfg.NoHooks || len(steps) == 0 {
			return nil
		}
		rendered, err := renderHookSteps(steps, v)
		if err != nil {
			return err
		}
		step := PlanStep{Action: "hook", Point: point, Target: target}
		for _, r := range rendered {
			step.Commands = append(step.Commands, r.Run)
		}
		step.Stage = stagePatterns(rendered)
		plan.Steps = append(plan.Steps, step)
		return nil
	}

	if err := hook("post_bump", "", p.cfg.PostBumpHooks, vars); err != nil {
		return Plan{}, err
	}
	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)
		if err := hook("pre_changelog", "", p.cfg.PreChangelogHooks, vars); err != nil {
			return Plan{}, err
		}
		plan.Steps = append(plan.Steps,
			PlanStep{Action: "changelog", Target: file},
			PlanStep{Action: "commit_changelog", Target: file})
	}
	if err := hook("pre_tag", "", p.cfg.PreTagHooks, vars); err != nil {
		return Plan{}, err
	}
	plan.Steps = append(plan.Steps,
		PlanStep{Action: "tag", Target: result.Tag},
		PlanStep{Action: "push_tag", Target: result.Tag})
	if err := hook("post_tag", "", p.cfg.PostTagHooks, vars); err != nil {
		return Plan{}, err
	}

	notes := p.cfg.Notes != nil && !p.cfg.DisableNotes
	if notes && len(p.cfg.Platforms) <= 1 {
		plan.Steps = append(plan.Steps, PlanStep{Action: "notes"})
	}
	for _, plat := range p.cfg.Platforms {
		platVars := vars
		platVars.Platform = plat.Name()
		if err := hook("pre_release", plat.Name(), p.cfg.PreReleaseHooks, platVars); err != nil {
			return Plan{}, err
		}
		plan.Steps = append(plan.Steps, PlanStep{Action: "publish", Target: plat.Name(), Notes: notes, Assets: plat.HasAssets(), PreRelease: prerelease})
		if err := hook("post_release", plat.Name(), p.cfg.PostReleaseHooks, platVars); err != nil {
			return Plan{}, err
		}
	}
	return plan, nil
}
