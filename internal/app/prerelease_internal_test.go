package app

import (
	"bytes"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPreReleaseRun(t *testing.T) {
	relPrefix := "rel-"
	semverCfg := func() *config.Config {
		return &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"}}
	}
	prefixedCfg := func() *config.Config {
		c := semverCfg()
		c.Versioning.TagPrefix = &relPrefix
		return c
	}
	perEnvCfg := func() *config.Config {
		return &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver-per-env", TagFormat: "{env}/{version}"},
		}
	}
	calverCfg := func() *config.Config {
		return &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
		}
	}

	tests := []struct {
		name string
		cfg  *config.Config
		env  string
		opts PipelineOpts
		want bool
	}{
		{"label set", semverCfg(), "", PipelineOpts{PreReleaseLabel: "rc"}, true},
		{"override pre-release", semverCfg(), "", PipelineOpts{VersionOverride: "1.4.0-rc.1"}, true},
		{"override pre-release with v", semverCfg(), "", PipelineOpts{VersionOverride: "v1.4.0-rc.1"}, true},
		{"override pre-release with custom prefix", prefixedCfg(), "", PipelineOpts{VersionOverride: "rel-1.4.0-rc.1"}, true},
		{"override final", semverCfg(), "", PipelineOpts{VersionOverride: "1.4.0"}, false},
		{"override final with build metadata", semverCfg(), "", PipelineOpts{VersionOverride: "1.4.0+5"}, false},
		{"semver-per-env override pre-release", perEnvCfg(), "prod", PipelineOpts{VersionOverride: "7.4.1-rc.1"}, true},
		{"calver override shaped like a pre-release", calverCfg(), "", PipelineOpts{VersionOverride: "2026.10.2-0"}, false},
		{"unparsable override", semverCfg(), "", PipelineOpts{VersionOverride: "not-a-version"}, false},
		{"nothing set", semverCfg(), "", PipelineOpts{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isPreReleaseRun(tc.cfg, tc.env, tc.opts))
		})
	}
}

type staticResolver struct{ result versioning.Result }

func (r staticResolver) Resolve() (versioning.Result, error) { return r.result, nil }

func preReleaseChangelogCfg() *config.Config {
	return &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{Output: "CHANGELOG.md"},
	}
}

func TestPreReleaseRun_StepTotalExcludesChangelog(t *testing.T) {
	runner := exectest.NewMockRunner()

	final, err := buildReleasePipelineConfig(runner, runner, preReleaseChangelogCfg(), "", "", false, false, false)
	require.NoError(t, err)
	pre, err := buildReleasePipelineConfig(runner, runner, preReleaseChangelogCfg(), "", "", false, false, true)
	require.NoError(t, err)

	assert.False(t, final.DisableChangelog)
	assert.True(t, pre.DisableChangelog)
	assert.Equal(t, releaseStepTotal(final)-2, releaseStepTotal(pre), "generate + commit changelog steps dropped")

	opts := PipelineOpts{Tag: true}
	finalCl, err := buildChangelogPipelineConfig(runner, runner, preReleaseChangelogCfg(), opts, false)
	require.NoError(t, err)
	preCl, err := buildChangelogPipelineConfig(runner, runner, preReleaseChangelogCfg(), opts, true)
	require.NoError(t, err)

	assert.False(t, finalCl.PreRelease)
	assert.True(t, preCl.PreRelease)
	assert.True(t, preCl.DisableChangelog)
	assert.Equal(t, changelogStepTotal(finalCl)-2, changelogStepTotal(preCl), "generate + commit changelog steps dropped")
}

// TestBuildPipeline_PreRelease_DryRunShowsNoChangelog pins that a pre-release dry run renders
// neither changelog lines nor pre_changelog hooks and that the step counter matches the steps run.
func TestBuildPipeline_PreRelease_DryRunShowsNoChangelog(t *testing.T) {
	testutil.ClearCIEnv(t)
	cfg := preReleaseChangelogCfg()
	cfg.Hooks = &config.Hooks{PreChangelog: []config.HookStep{{Run: "echo pre-changelog-marker"}}}
	res := staticResolver{result: versioning.Result{Version: "1.4.0-rc.1", Tag: "v1.4.0-rc.1"}}

	var out bytes.Buffer
	p, err := BuildPipeline(exectest.NewMockRunner(), cfg, res, PipelineOpts{
		DryRun: true, Out: &out, PreReleaseLabel: "rc",
	})
	require.NoError(t, err)
	require.NoError(t, p.Run())

	assert.NotContains(t, out.String(), "hangelog")
	assert.NotContains(t, out.String(), "pre-changelog-marker")
	assert.Contains(t, out.String(), "[3/3]", "resolve + tag + push")
	assert.NotContains(t, out.String(), "/4]")
}
