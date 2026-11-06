package pipeline_test

import (
	"bytes"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
)

// A detached HEAD (a GitLab CI checkout) pushes the changelog commit to the CI job's branch; the
// tag push is unchanged.
func TestRun_CommitPushBranch_NamesTheBranch(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueCommitAndTag(mr)
	cfg := &pipeline.Config{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Platforms:         []port.Platform{&testutil.MockPlatform{PlatformName: "gitlab"}},
		CommitPushBranch:  "main",
		CommitPushOptions: commitOpts,
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run(t.Context()))

	require.Len(t, mr.Calls, 6)
	assert.Equal(t, []string{"push", "--push-option=ci.skip", "origin", "HEAD:refs/heads/main"}, mr.Calls[3].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[5].Args)
}

func TestChangelogRun_CommitPushBranch_NamesTheBranch(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueCommitAndTag(mr)
	cfg := &pipeline.ChangelogConfig{
		Changelog:        &testutil.MockGenerator{},
		ChangelogFile:    "CHANGELOG.md",
		Tag:              true,
		CommitPushBranch: "release/1.3",
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run(t.Context()))

	require.Len(t, mr.Calls, 6)
	assert.Equal(t, []string{"push", "origin", "HEAD:refs/heads/release/1.3"}, mr.Calls[3].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[5].Args)
}
