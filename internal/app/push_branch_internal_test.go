package app

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
)

func TestCommitPushBranch(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		responses [][2]any // stdout, err
		want      string
		wantErr   string
		wantCalls [][]string
	}{
		{
			name:      "branch checked out: push HEAD as is",
			env:       map[string]string{"CI_COMMIT_BRANCH": "main"},
			responses: [][2]any{{"main\n", nil}},
			wantCalls: [][]string{{"symbolic-ref", "-q", "--short", "HEAD"}},
		},
		{
			name:      "detached, GitLab branch fetched",
			env:       map[string]string{"CI_COMMIT_BRANCH": "main"},
			responses: [][2]any{{"", errors.New("exit status 1")}, {"4e1c2a7\n", nil}},
			want:      "main",
			wantCalls: [][]string{{"symbolic-ref", "-q", "--short", "HEAD"}, {"rev-parse", "--verify", "--quiet", "refs/remotes/origin/main"}},
		},
		{
			name:      "detached, no CI branch",
			responses: [][2]any{{"", errors.New("exit status 1")}},
			wantErr:   "HEAD is detached and no CI branch variable names the branch",
			wantCalls: [][]string{{"symbolic-ref", "-q", "--short", "HEAD"}},
		},
		{
			name:      "detached, CI branch not fetched",
			env:       map[string]string{"GITHUB_REF_NAME": "12/merge", "GITHUB_REF_TYPE": "branch"},
			responses: [][2]any{{"", errors.New("exit status 1")}, {"", errors.New("exit status 1")}},
			wantErr:   `origin/12/merge was not fetched`,
			wantCalls: [][]string{{"symbolic-ref", "-q", "--short", "HEAD"}, {"rev-parse", "--verify", "--quiet", "refs/remotes/origin/12/merge"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testutil.ClearCIEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			mr := exectest.NewMockRunner()
			for _, r := range tc.responses {
				err, _ := r[1].(error)
				mr.QueueResponse(r[0].(string), "", err)
			}

			got, err := commitPushBranch(mr)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				assert.ErrorContains(t, err, "git checkout -B", "the hint says how to fix it")
				assert.True(t, IsUnusableCheckout(err), "commands map it to the Runtime code")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
			require.Len(t, mr.Calls, len(tc.wantCalls))
			for i, args := range tc.wantCalls {
				assert.Equal(t, "git", mr.Calls[i].Name)
				assert.Equal(t, args, mr.Calls[i].Args)
			}
		})
	}
}

// TestBuildPipelineConfigs_CommitPushBranch checks that both pipelines carry the branch only when
// they commit and push, so a run that pushes no commit never looks the branch up.
func TestBuildPipelineConfigs_CommitPushBranch(t *testing.T) {
	testutil.ClearCIEnv(t)
	t.Setenv("CI_COMMIT_BRANCH", "main")
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{Output: "CHANGELOG.md"},
		Forges:     []config.Forge{{Name: "gitlab", Type: "gitlab", Project: "acme/app", BaseURL: "https://gitlab.example.com"}},
		Commits:    &config.Commits{EnrichmentPolicy: "disabled"},
	}
	detached := func(origin bool) *exectest.MockRunner {
		mr := exectest.NewMockRunner()
		if origin {
			mr.QueueResponse("https://gitlab.example.com/acme/app.git\n", "", nil) // git remote get-url origin
		}
		mr.QueueResponse("", "", errors.New("exit status 1")) // git symbolic-ref: detached
		mr.QueueResponse("4e1c2a7\n", "", nil)                // origin/main fetched
		return mr
	}

	t.Run("release", func(t *testing.T) {
		read := detached(true)
		pCfg, err := buildReleasePipelineConfig(exectest.NewMockRunner(), read, cfg, "", "", false, false, false)
		require.NoError(t, err)
		assert.Equal(t, "main", pCfg.CommitPushBranch)
	})
	t.Run("changelog --commit", func(t *testing.T) {
		cCfg, err := buildChangelogPipelineConfig(exectest.NewMockRunner(), detached(false), cfg, PipelineOpts{Commit: true}, false)
		require.NoError(t, err)
		assert.Equal(t, "main", cCfg.CommitPushBranch)
	})
	t.Run("changelog --commit --no-push looks nothing up", func(t *testing.T) {
		read := exectest.NewMockRunner()
		cCfg, err := buildChangelogPipelineConfig(exectest.NewMockRunner(), read, cfg, PipelineOpts{Commit: true, NoPush: true}, false)
		require.NoError(t, err)
		assert.Empty(t, cCfg.CommitPushBranch)
		assert.Empty(t, read.Calls)
	})
	t.Run("changelog without --commit looks nothing up", func(t *testing.T) {
		read := exectest.NewMockRunner()
		_, err := buildChangelogPipelineConfig(exectest.NewMockRunner(), read, cfg, PipelineOpts{}, false)
		require.NoError(t, err)
		assert.Empty(t, read.Calls)
	})
	t.Run("release without a changelog looks nothing up", func(t *testing.T) {
		noChangelog := *cfg
		noChangelog.Changelog = nil
		read := exectest.NewMockRunner()
		read.QueueResponse("https://gitlab.example.com/acme/app.git\n", "", nil) // git remote get-url origin
		_, err := buildReleasePipelineConfig(exectest.NewMockRunner(), read, &noChangelog, "", "", false, false, false)
		require.NoError(t, err)
		for _, c := range read.Calls {
			assert.NotEqual(t, "symbolic-ref", c.Args[0])
		}
	})
}
