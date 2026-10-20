package pipeline

import (
	"errors"
	"fmt"
	"io"

	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/adaouat/heraut/internal/versioning"
)

// releaseStep describes one action of a release once, for the three things that need it: Run
// executes it, a dry run renders it, and Plan lists it. Keeping them on one value is what makes
// the executed steps, the dry-run output and the --json plan agree on which steps exist and in
// what order.
type releaseStep struct {
	// name is the label the reporter shows for the step.
	name string
	// gate, when set, makes this an unreported check that runs in a real run and a dry run alike.
	gate func() error
	// exec performs the step.
	exec func() (detail string, subs []string, err error)
	// after runs once exec succeeded and the reporter is done with the step, so its output is not
	// drawn over the spinner.
	after func()
	// dry describes what exec would do, for the reporter. It is evaluated before the step is
	// reported, so a hook template that does not render aborts the dry run instead of showing a
	// failed step. Nil means the step has nothing to show.
	dry func() (detail string, subs []string, err error)
	// plain returns the lines a dry run prints when there is no reporter. Nil means none.
	plain func() ([]string, error)
	// plan returns the entries Plan lists for the step. Nil means none.
	plan func() ([]PlanStep, error)
}

// releaseFlow is the ordered list of steps of one release. The prelude runs before the dry-run
// header and the body after it; both are executed, rendered or planned the same way.
type releaseFlow struct {
	prelude    []releaseStep
	body       []releaseStep
	prerelease bool
	// hookFailed collects the platforms whose pre_release or post_release hook failed during Run.
	hookFailed []string
}

func (f *releaseFlow) steps() []releaseStep {
	return append(append([]releaseStep{}, f.prelude...), f.body...)
}

// releaseFlow builds the steps of a release for a resolved version. Nothing runs here.
func (p *Pipeline) releaseFlow(result versioning.Result) *releaseFlow {
	// prerelease is derived once from the resolved version, but only under a SemVer strategy
	// (ADR-0064): a SemVer pre-release (e.g. 2.0.0-rc.1) publishes as a pre-release; a final
	// version never does. cfg.SemVerStrategy gates this — a CalVer version can itself parse as a
	// SemVer pre-release (e.g. a calver `format: YYYY.MM.SS-PATCH` resolving "2026.10.2-0"), so
	// the version's shape alone cannot be trusted; the active strategy decides whether the concept
	// even applies.
	flow := &releaseFlow{prerelease: p.cfg.SemVerStrategy && isPreRelease(result.Version)}
	vars := p.hookVars(result)

	// stage collects the files hooks asked to commit with the changelog (ADR-0061): post_bump's,
	// then pre_changelog's.
	var stage []string
	var committed bool
	var notes string

	if s, ok := p.hookStep("post_bump", p.cfg.PostBumpHooks, vars, &stage); ok {
		if p.cfg.Changelog == nil || p.cfg.DisableChangelog {
			s = warnIfStageUncommitted(s, p.out, &stage)
		}
		flow.prelude = append(flow.prelude, s)
	}
	// Refuse unpublishable assets before anything is committed or pushed: the publish step runs
	// after the tag is pushed, so a refusal there would leave a tag without a release. Placed after
	// post_bump so a hook that builds the assets has run, and before the dry-run return so a
	// dry run reports the same refusal.
	flow.prelude = append(flow.prelude, releaseStep{gate: func() error {
		for _, plat := range p.cfg.Platforms {
			if err := plat.CheckAssets(); err != nil {
				// A dry run renders hooks without running them, so the files a hook would build do
				// not exist: an unmatched pattern is a warning there, and the real run applies the
				// rule (ADR-0076).
				if p.dryRun && errors.Is(err, port.ErrAssetUnmatched) {
					_, _ = fmt.Fprintln(p.out, ui.Warn(p.out, fmt.Sprintf("[dry-run] platform %s: %v", plat.Name(), err)))
					continue
				}
				return fmt.Errorf("platform %s: %w", plat.Name(), err)
			}
		}
		return nil
	}})

	// The committed changelog is singular and tied to origin, so it resolves links from the
	// ambient CI host (ADR-0022), falling back to the single configured platform context for
	// local/non-CI runs.
	if p.cfg.Changelog != nil && !p.cfg.DisableChangelog {
		file := resolvedChangelogFile(p.cfg.Changelog, p.cfg.ChangelogFile)
		if s, ok := p.hookStep("pre_changelog", p.cfg.PreChangelogHooks, vars, &stage); ok {
			flow.body = append(flow.body, s)
		}
		flow.body = append(flow.body,
			releaseStep{
				name: "Generate changelog",
				exec: func() (string, []string, error) {
					if _, err := p.cfg.Changelog.Generate(result.Tag, p.changelogLinkContext()); err != nil {
						return "", nil, fmt.Errorf("generating changelog: %w", err)
					}
					detail, subs := changelogGenResult(p.cfg.Changelog)
					return detail, subs, nil
				},
				dry: func() (string, []string, error) { return "[dry-run] would write " + file, nil, nil },
				plain: func() ([]string, error) {
					return []string{"[dry-run] would generate changelog → commit → push"}, nil
				},
				plan: func() ([]PlanStep, error) { return []PlanStep{{Action: "changelog", Target: file}}, nil },
			},
			releaseStep{
				name: "Commit changelog",
				exec: func() (string, []string, error) {
					var err error
					committed, err = p.git.commitChangelog(append([]string{file}, stage...), commitMessage(p.cfg.CommitMessage, result.Version), true)
					if err != nil {
						return "", nil, fmt.Errorf("committing changelog: %w", err)
					}
					return "", nil, nil
				},
				after: func() {
					if !committed {
						warnNothingToCommit(p.out, file)
					}
				},
				dry:  func() (string, []string, error) { return "[dry-run] would commit and push", nil, nil },
				plan: func() ([]PlanStep, error) { return []PlanStep{{Action: "commit_changelog", Target: file}}, nil },
			})
	}

	if s, ok := p.hookStep("pre_tag", p.cfg.PreTagHooks, vars, nil); ok {
		flow.body = append(flow.body, s)
	}
	flow.body = append(flow.body,
		releaseStep{
			name: fmt.Sprintf("Create tag %s", result.Tag),
			exec: func() (string, []string, error) {
				if err := p.git.tag(result.Tag, commitMessage(p.cfg.CommitMessage, result.Version), p.cfg.AnnotatedTags, p.cfg.SignTags); err != nil {
					return "", nil, fmt.Errorf("git tag: %w", err)
				}
				return "", nil, nil
			},
			dry: func() (string, []string, error) { return "[dry-run] would tag", nil, nil },
			plain: func() ([]string, error) {
				return []string{fmt.Sprintf("[dry-run] would tag %s and push", result.Tag)}, nil
			},
			plan: func() ([]PlanStep, error) { return []PlanStep{{Action: "tag", Target: result.Tag}}, nil },
		},
		releaseStep{
			name: "Push tag",
			exec: func() (string, []string, error) { return "", nil, p.git.pushTag(result.Tag) },
			dry: func() (string, []string, error) {
				return fmt.Sprintf("[dry-run] would push %s", result.Tag), nil, nil
			},
			plan: func() ([]PlanStep, error) { return []PlanStep{{Action: "push_tag", Target: result.Tag}}, nil },
		})
	if s, ok := p.hookStep("post_tag", p.cfg.PostTagHooks, vars, nil); ok {
		flow.body = append(flow.body, s)
	}

	// With a single platform, notes are generated once up front with that platform's resolved link
	// context. With several, they are regenerated inside each publish step with that platform's
	// LinkContext so links carry the right host/path shape (ADR-0021 / ADR-0022).
	notesEnabled := p.cfg.Notes != nil && !p.cfg.DisableNotes
	multiPlatform := len(p.cfg.Platforms) > 1
	if notesEnabled && !multiPlatform {
		flow.body = append(flow.body, releaseStep{
			name: "Generate release notes",
			exec: func() (string, []string, error) {
				var err error
				notes, err = p.cfg.Notes.Generate(result.Tag, p.singlePlatformLinkContext())
				if err != nil {
					return "", nil, fmt.Errorf("generating release notes: %w", err)
				}
				return "", degradedSubs(p.cfg.Notes), nil
			},
			dry:  func() (string, []string, error) { return "[dry-run] would generate", nil, nil },
			plan: func() ([]PlanStep, error) { return []PlanStep{{Action: "notes"}}, nil },
		})
	}

	for _, plat := range p.cfg.Platforms {
		flow.body = append(flow.body, p.publishStep(plat, result, vars, flow.prerelease, notesEnabled && multiPlatform, notesEnabled, &notes))
	}
	return flow
}

// hookStep builds the step for one hook point. ok is false when the point has nothing to run:
// nothing configured, or --no-hooks. When stage is non-nil the Stage patterns the commands declare
// are appended to it as they run.
func (p *Pipeline) hookStep(point string, steps []HookStep, vars hookVars, stage *[]string) (releaseStep, bool) {
	return newHookStep(p.cfg.NoHooks, &p.git, point, steps, vars, stage)
}

// newHookStep is hookStep for any pipeline: noHooks and git come from the caller's config.
func newHookStep(noHooks bool, git *gitHelper, point string, steps []HookStep, vars hookVars, stage *[]string) (releaseStep, bool) {
	if noHooks || len(steps) == 0 {
		return releaseStep{}, false
	}
	return releaseStep{
		name: "Run " + point + " hooks",
		exec: func() (string, []string, error) {
			rendered, err := runHookPointSteps(git.interactiveOrRunner(), steps, vars)
			if err != nil {
				return "", nil, err
			}
			if stage != nil {
				*stage = append(*stage, stagePatterns(rendered)...)
			}
			return "", nil, nil
		},
		dry: func() (string, []string, error) {
			lines, err := dryRunHookLines(steps, vars)
			if err != nil || len(lines) == 0 {
				return "", nil, err
			}
			return lines[0], lines[1:], nil
		},
		plain: func() ([]string, error) { return dryRunHookLines(steps, vars) },
		plan: func() ([]PlanStep, error) {
			entry, err := hookPlanStep(point, "", steps, vars)
			return []PlanStep{entry}, err
		},
	}, true
}

// warnIfStageUncommitted makes a hook step report the stage patterns it declared when this run has
// no changelog commit to carry them (ADR-0061 addendum). Only the patterns this step added are
// named, so two hook points never repeat each other.
func warnIfStageUncommitted(s releaseStep, w io.Writer, stage *[]string) releaseStep {
	inner := s.exec
	start := 0
	s.exec = func() (string, []string, error) {
		start = len(*stage)
		return inner()
	}
	s.after = func() {
		if added := (*stage)[start:]; len(added) > 0 {
			warnStageNotCommitted(w, added)
		}
	}
	return s
}

// hookPlanStep renders one hook point's commands and stage patterns for Plan.
func hookPlanStep(point, target string, steps []HookStep, vars hookVars) (PlanStep, error) {
	rendered, err := renderHookSteps(steps, vars)
	if err != nil {
		return PlanStep{}, err
	}
	entry := PlanStep{Action: "hook", Point: point, Target: target, Stage: stagePatterns(rendered)}
	for _, r := range rendered {
		entry.Commands = append(entry.Commands, r.Run)
	}
	return entry, nil
}

// publishStep builds the step that publishes to one platform, with that platform's pre_release and
// post_release hooks folded in. A hook failure there is isolated to the platform (ADR-0053),
// unlike a real publish failure, which aborts the whole loop.
func (p *Pipeline) publishStep(plat port.Platform, result versioning.Result, vars hookVars, prerelease, regenerateNotes, notesEnabled bool, sharedNotes *string) releaseStep {
	platVars := vars
	platVars.Platform = plat.Name()
	hooksOn := !p.cfg.NoHooks

	return releaseStep{
		name: fmt.Sprintf("Publish to %s", plat.Name()),
		exec: func() (string, []string, error) {
			if hooksOn && len(p.cfg.PreReleaseHooks) > 0 {
				if _, err := runHookPointSteps(p.git.interactiveOrRunner(), p.cfg.PreReleaseHooks, platVars); err != nil {
					return "", nil, &hookFailureError{platform: plat.Name(), err: err}
				}
			}

			var subs []string
			notes := *sharedNotes
			lc := p.platformLinkContext(plat)
			if regenerateNotes {
				generated, err := p.cfg.Notes.Generate(result.Tag, lc)
				if err != nil {
					return "", nil, fmt.Errorf("platform %s: generating release notes: %w", plat.Name(), err)
				}
				notes = generated
				subs = append(subs, "notes generated")
				subs = append(subs, degradedSubs(p.cfg.Notes)...)
			}
			if err := plat.CreateRelease(result.Tag, notes, prerelease); err != nil {
				return "", nil, fmt.Errorf("platform %s: create release: %w", plat.Name(), err)
			}
			if plat.HasAssets() {
				subs = append(subs, "assets uploaded")
			}

			if hooksOn && len(p.cfg.PostReleaseHooks) > 0 {
				if _, err := runHookPointSteps(p.git.interactiveOrRunner(), p.cfg.PostReleaseHooks, platVars); err != nil {
					return "", nil, &hookFailureError{platform: plat.Name(), err: err}
				}
			}
			return plat.ReleaseURLFromContext(result.Tag, lc), subs, nil
		},
		dry: func() (string, []string, error) {
			var subs []string
			pre, err := p.dryRunHookLinesOrNil(p.cfg.PreReleaseHooks, platVars)
			if err != nil {
				return "", nil, err
			}
			subs = append(subs, pre...)
			if regenerateNotes {
				subs = append(subs, "[dry-run] would generate notes")
			}
			if plat.HasAssets() {
				subs = append(subs, "[dry-run] would upload assets")
			}
			post, err := p.dryRunHookLinesOrNil(p.cfg.PostReleaseHooks, platVars)
			if err != nil {
				return "", nil, err
			}
			return "[dry-run] would create release", append(subs, post...), nil
		},
		plain: func() ([]string, error) {
			pre, err := p.dryRunHookLinesOrNil(p.cfg.PreReleaseHooks, platVars)
			if err != nil {
				return nil, err
			}
			post, err := p.dryRunHookLinesOrNil(p.cfg.PostReleaseHooks, platVars)
			if err != nil {
				return nil, err
			}
			lines := append(pre, "[dry-run] would publish to "+plat.Name())
			return append(lines, post...), nil
		},
		plan: func() ([]PlanStep, error) {
			var entries []PlanStep
			if hooksOn && len(p.cfg.PreReleaseHooks) > 0 {
				entry, err := hookPlanStep("pre_release", plat.Name(), p.cfg.PreReleaseHooks, platVars)
				if err != nil {
					return nil, err
				}
				entries = append(entries, entry)
			}
			entries = append(entries, PlanStep{Action: "publish", Target: plat.Name(), Notes: notesEnabled, Assets: plat.HasAssets(), PreRelease: prerelease})
			if hooksOn && len(p.cfg.PostReleaseHooks) > 0 {
				entry, err := hookPlanStep("post_release", plat.Name(), p.cfg.PostReleaseHooks, platVars)
				if err != nil {
					return nil, err
				}
				entries = append(entries, entry)
			}
			return entries, nil
		},
	}
}

// dryRunHookLinesOrNil renders steps into "[dry-run] would run: <cmd>" lines, or nil when
// --no-hooks is set or nothing is configured.
func (p *Pipeline) dryRunHookLinesOrNil(steps []HookStep, vars hookVars) ([]string, error) {
	if p.cfg.NoHooks {
		return nil, nil
	}
	return dryRunHookLines(steps, vars)
}

// stepRunner executes, or in a dry run renders, releaseSteps. Both pipelines build one from their own
// settings, so the two share a single implementation of "run it, or show what it would do".
type stepRunner struct {
	dryRun   bool
	reporter ui.StepFn
	out      io.Writer
	// debug logs step boundaries; nil when the pipeline has no logger.
	debug func(msg string, args ...any)
}

// runStep calls fn via the reporter when one is set, or directly when nil.
// Errors returned by fn are propagated verbatim so callers can use errors.Is/As.
func (r stepRunner) runStep(name string, fn func() (string, []string, error)) error {
	if r.debug != nil {
		r.debug("step started", "step", name)
	}
	logged := func() (string, []string, error) {
		out, sub, err := fn()
		if err == nil && r.debug != nil {
			r.debug("step completed", "step", name)
		}
		return out, sub, err
	}
	if r.reporter == nil {
		_, _, err := logged()
		return err
	}
	return r.reporter(name, logged)
}

// runSteps executes, or in a dry run renders, steps in order.
func (r stepRunner) runSteps(steps []releaseStep, hookFailed *[]string) error {
	for _, s := range steps {
		if s.gate != nil {
			if err := s.gate(); err != nil {
				// A gate runs outside any reported step, and the command swaps the error for a
				// one-line summary once a reporter has run, so show the reason here.
				if r.reporter != nil {
					_, _ = fmt.Fprintln(r.out, ui.Err(r.out, err.Error()))
				}
				return err
			}
			continue
		}
		if r.dryRun {
			if err := r.renderStep(s); err != nil {
				return err
			}
			continue
		}
		if err := r.runStep(s.name, s.exec); err != nil {
			var hfe *hookFailureError
			if errors.As(err, &hfe) && hookFailed != nil {
				*hookFailed = append(*hookFailed, hfe.platform)
				continue
			}
			return err
		}
		if s.after != nil {
			s.after()
		}
	}
	return nil
}

// renderStep shows what s would do: one reported step when there is a reporter, otherwise the
// plain [dry-run] lines for CI-friendly output.
func (r stepRunner) renderStep(s releaseStep) error {
	if r.reporter == nil {
		if s.plain == nil {
			return nil
		}
		lines, err := s.plain()
		if err != nil {
			return err
		}
		for _, l := range lines {
			_, _ = fmt.Fprintln(r.out, l)
		}
		return nil
	}
	if s.dry == nil {
		return nil
	}
	detail, subs, err := s.dry()
	if err != nil {
		return err
	}
	if detail == "" && len(subs) == 0 {
		return nil
	}
	return r.runStep(s.name, func() (string, []string, error) { return detail, subs, nil })
}
