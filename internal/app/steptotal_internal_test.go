package app

import (
	"testing"

	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
)

// TestReleaseStepTotal covers the len(Platforms) branch (ADR-0021 / T70b): the standalone
// "Generate release notes" step is counted only for single-platform releases; multi-
// platform folds notes generation into the per-platform publish steps.
func TestReleaseStepTotal(t *testing.T) {
	gen := func() port.Generator { return &testutil.MockGenerator{} }
	plats := func(n int) []port.Platform {
		ps := make([]port.Platform, n)
		for i := range ps {
			ps[i] = &testutil.MockPlatform{}
		}
		return ps
	}

	tests := []struct {
		name   string
		cfg    *pipeline.Config
		dryRun bool
		want   int
	}{
		// base = resolve + tag + push = 3
		{"single platform, no notes", &pipeline.Config{Platforms: plats(1)}, false, 3 + 1},
		{"single platform, notes", &pipeline.Config{Notes: gen(), Platforms: plats(1)}, false, 3 + 1 + 1},
		{"multi platform, no notes", &pipeline.Config{Platforms: plats(2)}, false, 3 + 2},
		// multi-platform + notes: NO standalone notes step (folded), just the 2 publish steps
		{"multi platform, notes", &pipeline.Config{Notes: gen(), Platforms: plats(2)}, false, 3 + 2},
		{"changelog + notes, single", &pipeline.Config{
			Changelog: gen(), Notes: gen(), Platforms: plats(1),
		}, false, 3 + 2 + 1 + 1},
		{"changelog + notes, multi", &pipeline.Config{
			Changelog: gen(), Notes: gen(), Platforms: plats(2),
		}, false, 3 + 2 + 2},
		{"post_bump hook adds a step", &pipeline.Config{
			PostBumpHooks: []string{"echo hi"}, Platforms: plats(1),
		}, false, 3 + 1 + 1},
		{"pre_changelog hook only counted when changelog runs", &pipeline.Config{
			PreChangelogHooks: []string{"echo hi"}, Platforms: plats(1),
		}, false, 3 + 1}, // no changelog configured — hook never fires, no extra step
		{"pre_changelog hook counted alongside changelog", &pipeline.Config{
			Changelog: gen(), PreChangelogHooks: []string{"echo hi"}, Platforms: plats(1),
		}, false, 3 + 2 + 1 + 1},
		{"pre_tag and post_tag hooks each add a step", &pipeline.Config{
			PreTagHooks: []string{"echo hi"}, PostTagHooks: []string{"echo hi"}, Platforms: plats(1),
		}, false, 3 + 1 + 2},
		{"NoHooks suppresses every configured hook step", &pipeline.Config{
			NoHooks:       true,
			PostBumpHooks: []string{"echo hi"}, PreTagHooks: []string{"echo hi"}, PostTagHooks: []string{"echo hi"},
			Platforms: plats(1),
		}, false, 3 + 1},
		{"dry-run suppresses every configured hook step", &pipeline.Config{
			PostBumpHooks: []string{"echo hi"}, PreTagHooks: []string{"echo hi"}, PostTagHooks: []string{"echo hi"},
			Platforms: plats(1),
		}, true, 3 + 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, releaseStepTotal(tc.cfg, tc.dryRun))
		})
	}
}

// TestChangelogStepTotal covers changelogStepTotal's hook-step counting, mirroring
// TestReleaseStepTotal's shape for the changelog-only pipeline.
func TestChangelogStepTotal(t *testing.T) {
	gen := func() port.Generator { return &testutil.MockGenerator{} }

	tests := []struct {
		name   string
		cfg    *pipeline.ChangelogConfig
		dryRun bool
		want   int
	}{
		{"resolve only", &pipeline.ChangelogConfig{}, false, 1},
		{"post_bump hook adds a step even with nothing else", &pipeline.ChangelogConfig{
			PostBumpHooks: []string{"echo hi"},
		}, false, 1 + 1},
		{"changelog only, no commit/tag", &pipeline.ChangelogConfig{Changelog: gen()}, false, 1 + 1},
		{"pre_changelog hook counted alongside changelog", &pipeline.ChangelogConfig{
			Changelog: gen(), PreChangelogHooks: []string{"echo hi"},
		}, false, 1 + 1 + 1},
		{"tag + push, pre_tag and post_tag hooks each add a step", &pipeline.ChangelogConfig{
			Tag: true, PreTagHooks: []string{"echo hi"}, PostTagHooks: []string{"echo hi"},
		}, false, 1 + 1 + 1 + 1 + 1}, // resolve + pre_tag + create tag + push + post_tag
		{"NoHooks suppresses every configured hook step", &pipeline.ChangelogConfig{
			NoHooks: true, Tag: true,
			PostBumpHooks: []string{"echo hi"}, PreTagHooks: []string{"echo hi"}, PostTagHooks: []string{"echo hi"},
		}, false, 1 + 1 + 1}, // resolve + create tag + push
		{"dry-run suppresses every configured hook step", &pipeline.ChangelogConfig{
			Tag:           true,
			PostBumpHooks: []string{"echo hi"}, PreTagHooks: []string{"echo hi"}, PostTagHooks: []string{"echo hi"},
		}, true, 1 + 1 + 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, changelogStepTotal(tc.cfg, tc.dryRun))
		})
	}
}
