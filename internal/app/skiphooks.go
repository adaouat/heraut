package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adaouat/heraut/internal/pipeline"
)

var (
	releaseHookPoints   = []string{"post_bump", "pre_changelog", "pre_tag", "post_tag", "pre_release", "post_release"}
	changelogHookPoints = []string{"post_bump", "pre_changelog", "pre_tag", "post_tag"}
)

// ReleaseHookPoints returns every hook point `heraut release` runs, in pipeline order (ADR-0053).
func ReleaseHookPoints() []string { return slices.Clone(releaseHookPoints) }

// ChangelogHookPoints returns the hook points `heraut changelog` runs — the release list minus
// pre_release/post_release, since the changelog-only pipeline never publishes.
func ChangelogHookPoints() []string { return slices.Clone(changelogHookPoints) }

// ValidateSkipHooks normalizes a --skip-hook / HERAUT_SKIP_HOOKS list (trims whitespace, drops
// blank entries, removes duplicates while keeping first-seen order) and checks every entry against
// allowed, the hook points the invoking command runs. A point that exists but that this command
// never runs is reported differently from a typo, since the fix is different.
func ValidateSkipHooks(values, allowed []string) ([]string, error) {
	var out []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || slices.Contains(out, v) {
			continue
		}
		if !slices.Contains(allowed, v) {
			valid := strings.Join(allowed, ", ")
			if slices.Contains(releaseHookPoints, v) {
				return nil, fmt.Errorf("hook point %q does not apply to this command — valid points here: %s", v, valid)
			}
			return nil, fmt.Errorf("unknown hook point %q — valid points: %s", v, valid)
		}
		out = append(out, v)
	}
	return out, nil
}

// dropSkippedHooks empties each named point's step list. Emptying (rather than adding a parallel
// "skipped" flag) is deliberate: every consumer already treats an empty list as "nothing
// configured" — shouldRunHooks, the [N/total] step counters, dry-run hook lines, and the
// collection of declared `stage` patterns — so a skipped point behaves exactly like an absent one
// with no change to internal/pipeline.
func dropSkippedHooks(skip []string, points map[string]*[]pipeline.HookStep) {
	for _, point := range skip {
		if steps, ok := points[point]; ok {
			*steps = nil
		}
	}
}

func applySkipHooks(cfg *pipeline.Config, skip []string) {
	dropSkippedHooks(skip, map[string]*[]pipeline.HookStep{
		"post_bump":     &cfg.PostBumpHooks,
		"pre_changelog": &cfg.PreChangelogHooks,
		"pre_tag":       &cfg.PreTagHooks,
		"post_tag":      &cfg.PostTagHooks,
		"pre_release":   &cfg.PreReleaseHooks,
		"post_release":  &cfg.PostReleaseHooks,
	})
}

func applyChangelogSkipHooks(cfg *pipeline.ChangelogConfig, skip []string) {
	dropSkippedHooks(skip, map[string]*[]pipeline.HookStep{
		"post_bump":     &cfg.PostBumpHooks,
		"pre_changelog": &cfg.PreChangelogHooks,
		"pre_tag":       &cfg.PreTagHooks,
		"post_tag":      &cfg.PostTagHooks,
	})
}
