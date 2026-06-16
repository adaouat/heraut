package app_test

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func perEnvCfg(envBranches map[string]string) *config.Config {
	envs := make(map[string]config.Environment, len(envBranches))
	for name, branch := range envBranches {
		envs[name] = config.Environment{Bump: "auto", Branch: branch}
	}
	return &config.Config{
		Versioning:   config.Versioning{Strategy: "semver-per-env"},
		Environments: envs,
	}
}

func TestResolveEnv_Passthrough_NonAuto(t *testing.T) {
	mr := exectest.NewMockRunner()
	result, err := app.ResolveEnv("prod", perEnvCfg(map[string]string{"prod": "main"}), mr)
	require.NoError(t, err)
	assert.Equal(t, "prod", result)
	assert.Empty(t, mr.Calls, "non-auto env must not call git")
}

func TestResolveEnv_Passthrough_Empty(t *testing.T) {
	mr := exectest.NewMockRunner()
	result, err := app.ResolveEnv("", perEnvCfg(map[string]string{"prod": "main"}), mr)
	require.NoError(t, err)
	assert.Equal(t, "", result)
	assert.Empty(t, mr.Calls)
}

func TestResolveEnv_Auto_NilConfig(t *testing.T) {
	mr := exectest.NewMockRunner()
	_, err := app.ResolveEnv("auto", nil, mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
	assert.Empty(t, mr.Calls)
}

func TestResolveEnv_Auto_NonPerEnvStrategy(t *testing.T) {
	for _, strategy := range []string{"semver", "calver"} {
		t.Run(strategy, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			cfg := &config.Config{Versioning: config.Versioning{Strategy: strategy}}
			_, err := app.ResolveEnv("auto", cfg, mr)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "per-env")
			assert.Empty(t, mr.Calls, "non-per-env should fail before calling git")
		})
	}
}

func TestResolveEnv_Auto_GitError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("not a git repo"))
	_, err := app.ResolveEnv("auto", perEnvCfg(map[string]string{"prod": "main"}), mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "current git branch")
}

func TestResolveEnv_Auto_DetachedHEAD(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("HEAD\n", "", nil)
	_, err := app.ResolveEnv("auto", perEnvCfg(map[string]string{"prod": "main"}), mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "detached")
	assert.Contains(t, err.Error(), "--env")
}

func TestResolveEnv_Auto_NoBranchMatch(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("feat/new-thing\n", "", nil)
	_, err := app.ResolveEnv("auto", perEnvCfg(map[string]string{"prod": "main", "staging": "develop"}), mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feat/new-thing")
	assert.Contains(t, err.Error(), "--env")
}

func TestResolveEnv_Auto_SingleMatch(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("main\n", "", nil)
	cfg := perEnvCfg(map[string]string{"prod": "main", "staging": "develop"})
	result, err := app.ResolveEnv("auto", cfg, mr)
	require.NoError(t, err)
	assert.Equal(t, "prod", result)
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"rev-parse", "--abbrev-ref", "HEAD"}, mr.Calls[0].Args)
}

func TestResolveEnv_Auto_MultipleMatch(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("main\n", "", nil)
	cfg := perEnvCfg(map[string]string{"prod": "main", "hotfix": "main"})
	_, err := app.ResolveEnv("auto", cfg, mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prod")
	assert.Contains(t, err.Error(), "hotfix")
	assert.Contains(t, err.Error(), "main")
	assert.Contains(t, err.Error(), "--env")
}
