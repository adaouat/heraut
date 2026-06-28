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

func TestCheckCommitRange_NoRange_OmitsRangeArg(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("abc1234\x01feat: a\x01feat: a\x00", "", nil)

	_, err := app.CheckCommitRange(mr, nil, "")
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"log", "--format=%h%x01%s%x01%B%x00"}, mr.Calls[0].Args)
}

func TestCheckCommitRange_WithRange_AppendsRangeArg(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("abc1234\x01feat: a\x01feat: a\x00", "", nil)

	_, err := app.CheckCommitRange(mr, nil, "main..HEAD")
	require.NoError(t, err)

	assert.Equal(t, []string{"log", "main..HEAD", "--format=%h%x01%s%x01%B%x00"}, mr.Calls[0].Args)
}

func TestCheckCommitRange_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		stdout    string
		cfg       *config.Config
		wantCount int
		wantValid []bool // parallel to wantCount, true = Err nil
	}{
		{
			name:      "all valid, default types",
			stdout:    "aaa1111\x01feat: a\x01feat: a\x00bbb2222\x01fix: b\x01fix: b\x00",
			wantCount: 2,
			wantValid: []bool{true, true},
		},
		{
			name:      "invalid commit does not stop the scan (collect-all)",
			stdout:    "aaa1111\x01not conventional\x01not conventional\x00bbb2222\x01fix: b\x01fix: b\x00",
			wantCount: 2,
			wantValid: []bool{false, true},
		},
		{
			name:      "merge and fixup commits are skipped (valid)",
			stdout:    "aaa1111\x01Merge branch 'main'\x01Merge branch 'main' into feature/x\x00bbb2222\x01fixup! feat: a\x01fixup! feat: a\x00",
			wantCount: 2,
			wantValid: []bool{true, true},
		},
		{
			name:   "removed type is rejected by the allow-list",
			stdout: "aaa1111\x01docs: update\x01docs: update\x00",
			cfg: &config.Config{
				Commits: &config.Commits{Types: []config.TypeRule{{Name: "docs", Remove: true}}},
			},
			wantCount: 1,
			wantValid: []bool{false},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.stdout, "", nil)

			results, err := app.CheckCommitRange(mr, tc.cfg, "")
			require.NoError(t, err)
			require.Len(t, results, tc.wantCount)
			for i, wantValid := range tc.wantValid {
				if wantValid {
					assert.NoError(t, results[i].Err, "result %d", i)
				} else {
					assert.Error(t, results[i].Err, "result %d", i)
				}
			}
		})
	}
}

func TestCheckCommitRange_BodyWithBreakingChangeFooterAndBlankLines_ParsesIntact(t *testing.T) {
	mr := exectest.NewMockRunner()
	body := "feat: a\n\nSome body text.\n\nBREAKING CHANGE: this changes the API\x00"
	mr.QueueResponse("aaa1111\x01feat: a\x01"+body, "", nil)

	results, err := app.CheckCommitRange(mr, nil, "")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.NoError(t, results[0].Err)
	assert.Equal(t, "aaa1111", results[0].SHA)
	assert.Equal(t, "feat: a", results[0].Subject)
}

func TestCheckCommitRange_GitLogError_ReturnsWrappedError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "fatal: bad range", errors.New("exit status 128"))

	results, err := app.CheckCommitRange(mr, nil, "bad..range")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad..range")
	assert.Nil(t, results)
}

func TestResolveFromLatestTag(t *testing.T) {
	semverCfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver"},
	}

	tests := []struct {
		name        string
		cfg         *config.Config
		env         string
		queueStdout string
		queueStderr string
		queueErr    error
		wantRange   string
		wantNoTags  bool
		wantErr     bool
	}{
		{
			name:        "cfg present, tag found",
			cfg:         semverCfg,
			queueStdout: "v1.2.3\nv1.2.2\n",
			wantRange:   "v1.2.3..HEAD",
		},
		{
			name:        "cfg present, no tags",
			cfg:         semverCfg,
			queueStdout: "", // git tag -l returns empty → CurrentTag returns "no tags found"
			wantNoTags:  true,
		},
		{
			name:        "cfg nil, tag found via git describe",
			cfg:         nil,
			queueStdout: "v2.0.0\n",
			wantRange:   "v2.0.0..HEAD",
		},
		{
			name:        "cfg nil, no tags (git describe: No names found)",
			cfg:         nil,
			queueStderr: "fatal: No names found, cannot describe anything.",
			queueErr:    errors.New("exit status 128"),
			wantNoTags:  true,
		},
		{
			name:        "cfg nil, no tags (git describe: No tags can describe)",
			cfg:         nil,
			queueStderr: "fatal: No tags can describe 'abc1234'.",
			queueErr:    errors.New("exit status 128"),
			wantNoTags:  true,
		},
		{
			name:        "cfg nil, git describe unexpected error",
			cfg:         nil,
			queueStderr: "fatal: not a git repository",
			queueErr:    errors.New("exit status 128"),
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.queueStdout, tc.queueStderr, tc.queueErr)

			gotRange, gotNoTags, err := app.ResolveFromLatestTag(mr, tc.cfg, tc.env)
			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, "", gotRange)
				assert.False(t, gotNoTags)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRange, gotRange)
			assert.Equal(t, tc.wantNoTags, gotNoTags)
		})
	}
}
