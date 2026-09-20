package app_test

import (
	"bytes"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildPipeline_SkipHooks_DryRunOmitsSkippedPoint proves PipelineOpts.SkipHooks reaches the
// release pipeline end to end: the skipped point's command never shows up in the dry-run plan,
// while an unskipped point configured beside it still does.
func TestBuildPipeline_SkipHooks_DryRunOmitsSkippedPoint(t *testing.T) {
	cfg := semverCfg()
	cfg.Hooks = &config.Hooks{
		PreTag:  []config.HookStep{{Run: "echo pre-tag-marker"}},
		PostTag: []config.HookStep{{Run: "echo post-tag-marker"}},
	}
	var out bytes.Buffer
	opts := app.PipelineOpts{Out: &out, DryRun: true, SkipHooks: []string{"pre_tag"}}

	p, err := app.BuildPipeline(exectest.NewMockRunner(), cfg, defaultResolver, opts)
	require.NoError(t, err)
	require.NoError(t, p.Run())

	assert.NotContains(t, out.String(), "pre-tag-marker", "a skipped point must not appear in the dry-run plan")
	assert.Contains(t, out.String(), "post-tag-marker", "an unskipped point must still run")
}

func TestBuildPipeline_NoSkipHooks_DryRunShowsEveryPoint(t *testing.T) {
	cfg := semverCfg()
	cfg.Hooks = &config.Hooks{
		PreTag:  []config.HookStep{{Run: "echo pre-tag-marker"}},
		PostTag: []config.HookStep{{Run: "echo post-tag-marker"}},
	}
	var out bytes.Buffer
	opts := app.PipelineOpts{Out: &out, DryRun: true}

	p, err := app.BuildPipeline(exectest.NewMockRunner(), cfg, defaultResolver, opts)
	require.NoError(t, err)
	require.NoError(t, p.Run())

	assert.Contains(t, out.String(), "pre-tag-marker")
	assert.Contains(t, out.String(), "post-tag-marker")
}
