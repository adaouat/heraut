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
		name string
		cfg  *pipeline.Config
		want int
	}{
		// base = resolve + tag + push = 3
		{"single platform, no notes", &pipeline.Config{Platforms: plats(1)}, 3 + 1},
		{"single platform, notes", &pipeline.Config{Notes: gen(), Platforms: plats(1)}, 3 + 1 + 1},
		{"multi platform, no notes", &pipeline.Config{Platforms: plats(2)}, 3 + 2},
		// multi-platform + notes: NO standalone notes step (folded), just the 2 publish steps
		{"multi platform, notes", &pipeline.Config{Notes: gen(), Platforms: plats(2)}, 3 + 2},
		{"changelog + notes, single", &pipeline.Config{
			Changelog: gen(), Notes: gen(), Platforms: plats(1),
		}, 3 + 2 + 1 + 1},
		{"changelog + notes, multi", &pipeline.Config{
			Changelog: gen(), Notes: gen(), Platforms: plats(2),
		}, 3 + 2 + 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, releaseStepTotal(tc.cfg))
		})
	}
}
