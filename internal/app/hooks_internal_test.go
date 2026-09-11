package app

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildReleasePipelineConfig_PropagatesHooks proves cfg.Hooks (ADR-0053) reaches
// pipeline.Config's hook fields — the wiring point release.go's Run() actually reads.
func TestBuildReleasePipelineConfig_PropagatesHooks(t *testing.T) {
	testutil.ClearCIEnv(t)
	runner := exectest.NewMockRunner()
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("", "", assertNoOriginErr)

	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Hooks: &config.Hooks{
			PostBump:     []string{"echo post-bump"},
			PreChangelog: []string{"make lint"},
			PreTag:       []string{"go build ./..."},
			PostTag:      []string{"npm publish"},
		},
	}

	pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"echo post-bump"}, pCfg.PostBumpHooks)
	assert.Equal(t, []string{"make lint"}, pCfg.PreChangelogHooks)
	assert.Equal(t, []string{"go build ./..."}, pCfg.PreTagHooks)
	assert.Equal(t, []string{"npm publish"}, pCfg.PostTagHooks)
}

func TestBuildReleasePipelineConfig_NoHooksConfiguredIsNilSafe(t *testing.T) {
	testutil.ClearCIEnv(t)
	runner := exectest.NewMockRunner()
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("", "", assertNoOriginErr)

	cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"}}

	pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
	require.NoError(t, err)
	assert.Nil(t, pCfg.PostBumpHooks)
	assert.Nil(t, pCfg.PreChangelogHooks)
	assert.Nil(t, pCfg.PreTagHooks)
	assert.Nil(t, pCfg.PostTagHooks)
}

// TestBuildChangelogPipelineConfig_PropagatesHooks mirrors the release-pipeline test above for
// the changelog-only pipeline, plus PipelineOpts.NoHooks reaching ChangelogConfig.NoHooks.
func TestBuildChangelogPipelineConfig_PropagatesHooks(t *testing.T) {
	testutil.ClearCIEnv(t)
	runner := exectest.NewMockRunner()
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("", "", assertNoOriginErr)

	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Hooks: &config.Hooks{
			PostBump: []string{"echo post-bump"},
			PreTag:   []string{"go build ./..."},
			PostTag:  []string{"npm publish"},
		},
	}

	cCfg, err := buildChangelogPipelineConfig(runner, readRunner, cfg, PipelineOpts{NoHooks: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"echo post-bump"}, cCfg.PostBumpHooks)
	assert.Equal(t, []string{"go build ./..."}, cCfg.PreTagHooks)
	assert.Equal(t, []string{"npm publish"}, cCfg.PostTagHooks)
	assert.True(t, cCfg.NoHooks)
}
