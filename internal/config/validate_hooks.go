package config

import (
	"fmt"
)

// validateHooks validates every configured hook entry (ADR-0061): run is required everywhere,
// and only post_bump/pre_changelog entries may set stage — pre_tag/post_tag/pre_release/
// post_release all fire after the changelog commit already landed, so a file staged there has no
// commit left to receive it. A stage entry is also an error when no run of the config can produce
// a changelog commit for it to join (ADR-0061 addendum).
func validateHooks(cfg *Config) []ValidationError {
	if cfg.Hooks == nil {
		return nil
	}
	var errs []ValidationError
	noChangelogCommit := changelogNeverRuns(cfg)
	points := []struct {
		name       string
		steps      []HookStep
		allowStage bool
	}{
		{"post_bump", cfg.Hooks.PostBump, true},
		{"pre_changelog", cfg.Hooks.PreChangelog, true},
		{"pre_tag", cfg.Hooks.PreTag, false},
		{"post_tag", cfg.Hooks.PostTag, false},
		{"pre_release", cfg.Hooks.PreRelease, false},
		{"post_release", cfg.Hooks.PostRelease, false},
	}
	for _, p := range points {
		for i, step := range p.steps {
			base := fmt.Sprintf("hooks.%s[%d]", p.name, i)
			if step.Run == "" {
				errs = append(errs, ValidationError{
					Path: base + ".run", Message: "required",
					Hint: `every hook entry needs a run command, e.g. { run: "echo hi" }`,
				})
			}
			if p.allowStage && noChangelogCommit && len(step.Stage) > 0 {
				errs = append(errs, ValidationError{
					Path: base + ".stage", Message: "no changelog commit to join: the changelog never runs in this config",
					Hint: "configure a changelog: block (top level or per environment) and do not disable it everywhere, or remove stage — files staged here would never be committed (ADR-0061)",
				})
			}
			if !p.allowStage && len(step.Stage) > 0 {
				errs = append(errs, ValidationError{
					Path: base + ".stage", Message: "not allowed here",
					Hint: "stage is only valid under post_bump or pre_changelog (files staged there have no commit left to land in)",
				})
			}
		}
	}
	return errs
}

// changelogNeverRuns reports whether no run of cfg can generate and commit a changelog: every
// environment disables it, or neither the top level nor any environment configures one. The
// per-run cases (a single disabled environment, --pre-release, no --commit) are not decidable
// here and only warn at run time.
func changelogNeverRuns(cfg *Config) bool {
	if len(cfg.Environments) > 0 {
		allDisabled := true
		for _, env := range cfg.Environments {
			if !env.DisableChangelog {
				allDisabled = false
				break
			}
		}
		if allDisabled {
			return true
		}
	}
	if cfg.Changelog != nil {
		return false
	}
	for _, env := range cfg.Environments {
		if env.Changelog != nil && !env.DisableChangelog {
			return false
		}
	}
	return true
}
