package app

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
)

func pushOptionsConfig() *config.Config {
	return &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env", TagFormat: "{env}/{version}"},
		Git: &config.Git{PushOptions: &config.PushOptions{
			Commit: []string{"ci.skip"},
			Tag:    []string{"ci.variable=DEPLOY=true"},
		}},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto"},
			"prod": {Bump: "auto", Git: &config.Git{PushOptions: &config.PushOptions{Commit: []string{}}}},
		},
	}
}

func TestBuildPipelineConfigs_PushOptionsFollowTheEnvironment(t *testing.T) {
	testutil.ClearCIEnv(t)
	tests := []struct {
		env        string
		wantCommit []string
	}{
		{"dev", []string{"ci.skip"}},
		{"prod", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			cfg := pushOptionsConfig()

			pCfg, err := buildReleasePipelineConfig(exectest.NewMockRunner(), exectest.NewMockRunner(), cfg, tc.env, "", false, false, false)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCommit, pCfg.CommitPushOptions, "release commit push")
			assert.Equal(t, []string{"ci.variable=DEPLOY=true"}, pCfg.TagPushOptions, "release tag push")

			cCfg, err := buildChangelogPipelineConfig(exectest.NewMockRunner(), exectest.NewMockRunner(), cfg, PipelineOpts{Env: tc.env, Tag: true}, false)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCommit, cCfg.CommitPushOptions, "changelog commit push")
			assert.Equal(t, []string{"ci.variable=DEPLOY=true"}, cCfg.TagPushOptions, "changelog tag push")
		})
	}
}
