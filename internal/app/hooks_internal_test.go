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
			PreRelease:   []string{"echo pre-release"},
			PostRelease:  []string{"echo post-release"},
		},
	}

	pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"echo post-bump"}, pCfg.PostBumpHooks)
	assert.Equal(t, []string{"make lint"}, pCfg.PreChangelogHooks)
	assert.Equal(t, []string{"go build ./..."}, pCfg.PreTagHooks)
	assert.Equal(t, []string{"npm publish"}, pCfg.PostTagHooks)
	assert.Equal(t, []string{"echo pre-release"}, pCfg.PreReleaseHooks)
	assert.Equal(t, []string{"echo post-release"}, pCfg.PostReleaseHooks)
}

// TestBuildReleasePipelineConfig_PropagatesEnv proves the --env value reaches pipeline.Config.Env
// so hook commands can branch on {{ .Env }} the same way pre_release/post_release already branch
// on {{ .Platform }}.
func TestBuildReleasePipelineConfig_PropagatesEnv(t *testing.T) {
	testutil.ClearCIEnv(t)
	runner := exectest.NewMockRunner()
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("", "", assertNoOriginErr)

	cfg := &config.Config{
		Version:      "1",
		Versioning:   config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{"staging": {Bump: "patch"}},
	}

	pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "staging", "", false, false)
	require.NoError(t, err)
	assert.Equal(t, "staging", pCfg.Env)
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
	assert.Nil(t, pCfg.PreReleaseHooks)
	assert.Nil(t, pCfg.PostReleaseHooks)
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

// TestBuildChangelogPipelineConfig_PropagatesEnv mirrors the release-pipeline case above for the
// changelog-only pipeline.
func TestBuildChangelogPipelineConfig_PropagatesEnv(t *testing.T) {
	testutil.ClearCIEnv(t)
	runner := exectest.NewMockRunner()
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("", "", assertNoOriginErr)

	cfg := &config.Config{
		Version:      "1",
		Versioning:   config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{"staging": {Bump: "patch"}},
	}

	cCfg, err := buildChangelogPipelineConfig(runner, readRunner, cfg, PipelineOpts{Env: "staging"})
	require.NoError(t, err)
	assert.Equal(t, "staging", cCfg.Env)
}

// TestBuildReleasePipelineConfig_EmptyEnvIsFlatDefault proves Env stays "" when no --env is
// passed — the flat, non-per-env default, matching Platform's "empty elsewhere" framing.
func TestBuildReleasePipelineConfig_EmptyEnvIsFlatDefault(t *testing.T) {
	testutil.ClearCIEnv(t)
	runner := exectest.NewMockRunner()
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("", "", assertNoOriginErr)

	cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"}}

	pCfg, err := buildReleasePipelineConfig(runner, readRunner, cfg, "", "", false, false)
	require.NoError(t, err)
	assert.Equal(t, "", pCfg.Env)
}
