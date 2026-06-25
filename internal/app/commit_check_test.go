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
			name:   "configured type allowlist rejects out-of-list type",
			stdout: "aaa1111\x01docs: update\x01docs: update\x00",
			cfg: &config.Config{
				CommitLint: &config.CommitLint{Types: []string{"feat", "fix"}},
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
