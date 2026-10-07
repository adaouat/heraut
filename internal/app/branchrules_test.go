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

func clearBranchEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CI_COMMIT_BRANCH", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "BUILD_SOURCEBRANCHNAME"} {
		t.Setenv(k, "")
	}
}

func branchRulesCfg(rules ...config.BranchRule) *config.Config {
	return &config.Config{Versioning: config.Versioning{Strategy: "semver", Branches: rules}}
}

var testRules = []config.BranchRule{
	{Name: "main"},
	{Name: "release/*"},
	{Name: "lts", Range: "2.x"},
}

func TestCurrentBranch(t *testing.T) {
	tests := []struct {
		name    string
		gitOut  string
		gitErr  error
		env     map[string]string
		want    string
		wantOK  bool
		wantErr string
	}{
		{name: "attached", gitOut: "main\n", want: "main", wantOK: true},
		{name: "detached, GitLab", gitOut: "HEAD\n", env: map[string]string{"CI_COMMIT_BRANCH": "release/1.3"}, want: "release/1.3", wantOK: true},
		{name: "detached, GitHub branch", gitOut: "HEAD\n", env: map[string]string{"GITHUB_REF_NAME": "release/1.3", "GITHUB_REF_TYPE": "branch"}, want: "release/1.3", wantOK: true},
		{name: "detached, GitHub tag ref ignored", gitOut: "HEAD\n", env: map[string]string{"GITHUB_REF_NAME": "v1.3.1", "GITHUB_REF_TYPE": "tag"}},
		{name: "detached, Azure", gitOut: "HEAD\n", env: map[string]string{"BUILD_SOURCEBRANCHNAME": "release-1.3"}, want: "release-1.3", wantOK: true},
		{name: "detached, nothing", gitOut: "HEAD\n"},
		{name: "git error", gitErr: errors.New("boom"), wantErr: "determining current git branch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearBranchEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.gitOut, "", tc.gitErr)

			got, ok, err := app.CurrentBranch(mr)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantOK, ok)
			require.Len(t, mr.Calls, 1)
			assert.Equal(t, "git", mr.Calls[0].Name)
			assert.Equal(t, []string{"rev-parse", "--abbrev-ref", "HEAD"}, mr.Calls[0].Args)
		})
	}
}

func TestMatchBranchRule(t *testing.T) {
	minor3 := uint64(3)
	tests := []struct {
		name      string
		rules     []config.BranchRule
		branch    string
		unknown   bool
		needRange bool
		wantKind  app.BranchKind
		wantRange config.BranchRange
		wantRule  int
		wantErr   error
		wantMsg   string
	}{
		{name: "main", branch: "main", needRange: true, wantKind: app.BranchRelease, wantRule: 0},
		{name: "derived", branch: "release/1.3", needRange: true, wantKind: app.BranchMaintenance, wantRange: config.BranchRange{Major: 1, Minor: &minor3}, wantRule: 1},
		{name: "explicit", branch: "lts", needRange: true, wantKind: app.BranchMaintenance, wantRange: config.BranchRange{Major: 2}, wantRule: 2},
		{name: "nested not matched", branch: "release/1.3/hotfix", needRange: true, wantKind: app.BranchUnlisted, wantRule: -1},
		{name: "unlisted", branch: "feature/x", needRange: true, wantKind: app.BranchUnlisted, wantRule: -1},
		{name: "unknown", unknown: true, needRange: true, wantKind: app.BranchUnlisted, wantRule: -1},
		{name: "underivable", branch: "release/legacy", needRange: true, wantErr: app.ErrUnderivableRange,
			wantMsg: `branch "release/legacy" matches versioning.branches[1] ("release/*") but its name carries no N.x / N.N.x / N.N version — add range: to the entry`},
		{name: "underivable, set-version", branch: "release/7.8.0", needRange: false, wantKind: app.BranchMaintenance, wantRule: 1},
		{name: "ambiguous", rules: append(append([]config.BranchRule{}, testRules...), config.BranchRule{Name: "release/1.3"}), branch: "release/1.3", needRange: true,
			wantErr: app.ErrAmbiguousBranch, wantMsg: `entries "release/*", "release/1.3" all match branch "release/1.3"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rules := tc.rules
			if rules == nil {
				rules = testRules
			}
			m, err := app.MatchBranchRule(branchRulesCfg(rules...), tc.branch, !tc.unknown, tc.needRange)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Contains(t, err.Error(), tc.wantMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantKind, m.Kind)
			assert.Equal(t, tc.wantRange, m.Range)
			assert.Equal(t, tc.wantRule, m.Rule)
			if !tc.unknown {
				assert.Equal(t, tc.branch, m.Branch)
			}
		})
	}
}

func TestCheckReleaseBranch(t *testing.T) {
	t.Run("no block, no git call", func(t *testing.T) {
		clearBranchEnv(t)
		mr := exectest.NewMockRunner()
		require.NoError(t, app.CheckReleaseBranch(mr, branchRulesCfg(), false))
		assert.Empty(t, mr.Calls)
	})
	t.Run("listed", func(t *testing.T) {
		clearBranchEnv(t)
		mr := exectest.NewMockRunner()
		mr.QueueResponse("release/1.3\n", "", nil)
		require.NoError(t, app.CheckReleaseBranch(mr, branchRulesCfg(testRules...), false))
	})
	t.Run("unlisted", func(t *testing.T) {
		clearBranchEnv(t)
		mr := exectest.NewMockRunner()
		mr.QueueResponse("feature/x\n", "", nil)
		err := app.CheckReleaseBranch(mr, branchRulesCfg(testRules...), false)
		require.ErrorIs(t, err, app.ErrUnlistedBranch)
		assert.Contains(t, err.Error(), `branch "feature/x" (pass --force to release anyway)`)
		assert.Contains(t, err.Error(), "\n")
		assert.Contains(t, err.Error(), `"main"`)
		assert.Contains(t, err.Error(), `"release/*"`)
		assert.Contains(t, err.Error(), `"lts"`)
	})
	t.Run("unlisted with force, no git call", func(t *testing.T) {
		clearBranchEnv(t)
		mr := exectest.NewMockRunner()
		require.NoError(t, app.CheckReleaseBranch(mr, branchRulesCfg(testRules...), true))
		assert.Empty(t, mr.Calls)
	})
	t.Run("unknown branch names detached HEAD", func(t *testing.T) {
		clearBranchEnv(t)
		mr := exectest.NewMockRunner()
		mr.QueueResponse("HEAD\n", "", nil)
		err := app.CheckReleaseBranch(mr, branchRulesCfg(testRules...), false)
		require.ErrorIs(t, err, app.ErrUnlistedBranch)
		assert.Contains(t, err.Error(), "detached HEAD")
	})
	t.Run("git failure", func(t *testing.T) {
		clearBranchEnv(t)
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", errors.New("boom"))
		err := app.CheckReleaseBranch(mr, branchRulesCfg(testRules...), false)
		require.Error(t, err)
		assert.NotErrorIs(t, err, app.ErrUnlistedBranch)
	})
	t.Run("ambiguous entries propagate", func(t *testing.T) {
		clearBranchEnv(t)
		mr := exectest.NewMockRunner()
		mr.QueueResponse("release/1.3\n", "", nil)
		cfg := branchRulesCfg(config.BranchRule{Name: "release/*"}, config.BranchRule{Name: "release/1.3"})
		require.ErrorIs(t, app.CheckReleaseBranch(mr, cfg, false), app.ErrAmbiguousBranch)
	})
}
