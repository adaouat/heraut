package branch_test

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning/branch"
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
	mr := exectest.NewMockRunner()
	mr.QueueResponse("main\n", "", nil)
	require.NoError(t, branch.CheckBranch(mr, branchCfg("prod", "main"), "prod", false))
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"rev-parse", "--abbrev-ref", "HEAD"}, mr.Calls[0].Args)
}

func TestCheckBranch_Mismatch(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("feature/x\n", "", nil)
	err := branch.CheckBranch(mr, branchCfg("prod", "main"), "prod", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "main")
	assert.Contains(t, err.Error(), "feature/x")
	assert.Contains(t, err.Error(), "--force")
}

func TestCheckBranch_ForceBypasses(t *testing.T) {
	mr := exectest.NewMockRunner()
	// force → no git call, no error even though we never check.
	require.NoError(t, branch.CheckBranch(mr, branchCfg("prod", "main"), "prod", true))
	assert.Empty(t, mr.Calls, "force should skip the branch check entirely")
}

func TestCheckBranch_NoEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	require.NoError(t, branch.CheckBranch(mr, branchCfg("prod", "main"), "", false))
	assert.Empty(t, mr.Calls)
}

func TestCheckBranch_EnvWithoutBranch(t *testing.T) {
	mr := exectest.NewMockRunner()
	require.NoError(t, branch.CheckBranch(mr, branchCfg("prod", ""), "prod", false))
	assert.Empty(t, mr.Calls)
}

func TestCheckBranch_GitError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("not a git repo"))
	err := branch.CheckBranch(mr, branchCfg("prod", "main"), "prod", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "current")
}

// On a detached checkout, the guard reads the branch the CI job runs for, as the push and the branch
// rules do: CI systems check out a commit, not a branch.
func TestCheckBranch_DetachedHeadUsesTheCIBranch(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{name: "GitLab CI", env: map[string]string{"GITLAB_CI": "true", "CI_COMMIT_BRANCH": "main"}},
		{name: "GitHub Actions", env: map[string]string{"GITHUB_REF_NAME": "main", "GITHUB_REF_TYPE": "branch"}},
		{name: "Azure Pipelines", env: map[string]string{"BUILD_SOURCEBRANCH": "refs/heads/main"}},
		{name: "CI branch differs", env: map[string]string{"CI_COMMIT_BRANCH": "develop"}, wantErr: `current branch is "develop"`},
		{name: "a tag build is no branch", env: map[string]string{"GITHUB_REF_NAME": "v1.0.0", "GITHUB_REF_TYPE": "tag"}, wantErr: `current branch is "HEAD"`},
		{name: "outside CI", wantErr: `current branch is "HEAD"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testutil.ClearCIEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			mr := exectest.NewMockRunner()
			mr.QueueResponse("HEAD\n", "", nil)

			err := branch.CheckBranch(mr, branchCfg("prod", "main"), "prod", false)

			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			var mismatch *branch.BranchMismatchError
			require.ErrorAs(t, err, &mismatch)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
