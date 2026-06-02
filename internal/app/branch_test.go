package app_test

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func branchCfg(env, branch string) *config.Config {
	return &config.Config{
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			env: {Bump: "auto", Branch: branch},
		},
	}
}

func TestCheckBranch_Matches(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("main\n", "", nil)
	require.NoError(t, app.CheckBranch(mr, branchCfg("prod", "main"), "prod", false))
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"rev-parse", "--abbrev-ref", "HEAD"}, mr.Calls[0].Args)
}

func TestCheckBranch_Mismatch(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("feature/x\n", "", nil)
	err := app.CheckBranch(mr, branchCfg("prod", "main"), "prod", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "main")
	assert.Contains(t, err.Error(), "feature/x")
	assert.Contains(t, err.Error(), "--force")
}

func TestCheckBranch_ForceBypasses(t *testing.T) {
	mr := testutil.NewMockRunner()
	// force → no git call, no error even though we never check.
	require.NoError(t, app.CheckBranch(mr, branchCfg("prod", "main"), "prod", true))
	assert.Empty(t, mr.Calls, "force should skip the branch check entirely")
}

func TestCheckBranch_NoEnv(t *testing.T) {
	mr := testutil.NewMockRunner()
	require.NoError(t, app.CheckBranch(mr, branchCfg("prod", "main"), "", false))
	assert.Empty(t, mr.Calls)
}

func TestCheckBranch_EnvWithoutBranch(t *testing.T) {
	mr := testutil.NewMockRunner()
	require.NoError(t, app.CheckBranch(mr, branchCfg("prod", ""), "prod", false))
	assert.Empty(t, mr.Calls)
}

func TestCheckBranch_GitError(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", errors.New("not a git repo"))
	err := app.CheckBranch(mr, branchCfg("prod", "main"), "prod", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "current")
}
