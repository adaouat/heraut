package pipeline_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChangelogRun_GenerateOnly verifies changelog is generated but no git ops when
// neither --commit nor --tag is set.
func TestChangelogRun_GenerateOnly(t *testing.T) {
	mr := testutil.NewMockRunner()
	gen := &testutil.MockGenerator{GenerateOut: "changelog content"}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
	}

	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, false)
	require.NoError(t, p.Run())

	require.Len(t, gen.GenerateCalls, 1)
	assert.Equal(t, "v1.2.3", gen.GenerateCalls[0])
	assert.Len(t, mr.Calls, 0)
}

// TestChangelogRun_WithCommit verifies generate + git add + commit + push.
func TestChangelogRun_WithCommit(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, gen.GenerateCalls, 1)
	require.Len(t, mr.Calls, 3)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, "commit", mr.Calls[1].Args[0])
	assert.Equal(t, []string{"push"}, mr.Calls[2].Args)
}

// TestChangelogRun_WithTag verifies --tag implies commit: generate + commit + push + tag + push --tags.
func TestChangelogRun_WithTag(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Tag:           true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, gen.GenerateCalls, 1)
	require.Len(t, mr.Calls, 5)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, "commit", mr.Calls[1].Args[0])
	assert.Equal(t, []string{"push"}, mr.Calls[2].Args)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[3].Args)
	assert.Equal(t, []string{"push", "--tags"}, mr.Calls[4].Args)
}

// TestChangelogRun_TagWithoutChangelog verifies tag-only when no generator is configured.
func TestChangelogRun_TagWithoutChangelog(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	cfg := &pipeline.ChangelogConfig{
		Tag: true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"push", "--tags"}, mr.Calls[1].Args)
}

// TestChangelogRun_DryRun verifies no git mutations or generator calls in dry-run.
func TestChangelogRun_DryRun(t *testing.T) {
	mr := testutil.NewMockRunner()
	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
		Tag:           true,
	}

	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, true)
	require.NoError(t, p.Run())

	assert.Len(t, mr.Calls, 0)
	assert.Len(t, gen.GenerateCalls, 0)
	assert.Contains(t, out.String(), "v1.2.3")
}

// TestChangelogRun_DisabledChangelog exits 0 with an info message; no generate or git ops.
func TestChangelogRun_DisabledChangelog(t *testing.T) {
	mr := testutil.NewMockRunner()
	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:        gen,
		ChangelogFile:    "CHANGELOG.md",
		DisableChangelog: true,
	}

	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, false)
	require.NoError(t, p.Run())

	assert.Len(t, gen.GenerateCalls, 0)
	assert.Len(t, mr.Calls, 0)
	assert.Contains(t, out.String(), "disabled")
}

// TestChangelogRun_ResolverError propagates resolver failures.
func TestChangelogRun_ResolverError(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &pipeline.ChangelogConfig{}

	p := pipeline.NewChangelog(mr, &fakeResolver{err: errors.New("no commits since last tag")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no commits")
}

// TestChangelogRun_DefaultCommitMessage verifies the default commit message contains the version.
func TestChangelogRun_DefaultCommitMessage(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	commitCall := mr.Calls[1]
	assert.Equal(t, "commit", commitCall.Args[0])
	assert.Equal(t, "-m", commitCall.Args[1])
	assert.Contains(t, commitCall.Args[2], "1.2.3")
}

// TestChangelogRun_CustomCommitMessage verifies commit message template substitution.
func TestChangelogRun_CustomCommitMessage(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
		CommitMessage: "docs: update changelog for v${version}",
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	assert.Equal(t, "docs: update changelog for v1.2.3", mr.Calls[1].Args[2])
}
