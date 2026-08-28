package pipeline_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChangelogRun_GenerateOnly verifies changelog is generated but no git ops when
// neither --commit nor --tag is set.
func TestChangelogRun_GenerateOnly(t *testing.T) {
	mr := exectest.NewMockRunner()
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
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, gen.GenerateCalls, 1)
	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
	assert.Equal(t, "commit", mr.Calls[2].Args[0])
	assert.Equal(t, []string{"push", "origin", "HEAD"}, mr.Calls[3].Args)
}

// TestChangelogRun_WithCommit_UsesRotatedOutputPath verifies that when the generator reports a
// resolved output path (T247: rotation tokens in changelog.output), the commit step targets that
// concrete file — not the raw, unsubstituted ChangelogFile pattern. ChangelogFile can't be
// resolved at config-build time (it's known only once Generate() runs with the actual tag), so the
// pipeline must prefer the generator's own report when one is available.
func TestChangelogRun_WithCommit_UsesRotatedOutputPath(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)                    // git add
	mr.QueueResponse("CHANGELOG_2026.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)                    // git commit
	mr.QueueResponse("", "", nil)                    // git push

	gen := &testutil.MockGenerator{LastOutputPathV: "CHANGELOG_2026.md"}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG_{YYYY}.md",
		Commit:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("2026.05.0")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"add", "CHANGELOG_2026.md"}, mr.Calls[0].Args,
		"commit step must target the resolved rotated file, not the raw {YYYY} pattern")
}

// TestChangelogRun_WithTag verifies --tag implies commit: generate + commit + push + tag + push --tags.
func TestChangelogRun_WithTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push
	mr.QueueResponse("", "", nil)               // git tag
	mr.QueueResponse("", "", nil)               // git push <tag>

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Tag:           true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, gen.GenerateCalls, 1)
	require.Len(t, mr.Calls, 6)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
	assert.Equal(t, "commit", mr.Calls[2].Args[0])
	assert.Equal(t, []string{"push", "origin", "HEAD"}, mr.Calls[3].Args)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[4].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[5].Args)
}

// TestChangelogRun_TagWithoutChangelog verifies tag-only when no generator is configured.
func TestChangelogRun_TagWithoutChangelog(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.ChangelogConfig{
		Tag: true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[1].Args)
}

// TestChangelogRun_WithCommit_NoPush verifies --no-push commits but does not push.
func TestChangelogRun_WithCommit_NoPush(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
		NoPush:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, gen.GenerateCalls, 1)
	require.Len(t, mr.Calls, 3)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
	assert.Equal(t, "commit", mr.Calls[2].Args[0])
	for _, c := range mr.Calls {
		assert.NotEqual(t, "push", c.Args[0], "no git push expected with NoPush")
	}
}

// TestChangelogRun_WithTag_NoPush verifies --tag --no-push commits and tags but pushes neither.
func TestChangelogRun_WithTag_NoPush(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git tag

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Tag:           true,
		NoPush:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
	assert.Equal(t, "commit", mr.Calls[2].Args[0])
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[3].Args)
	for _, c := range mr.Calls {
		assert.NotEqual(t, "push", c.Args[0], "no git push expected with NoPush")
	}
}

// TestChangelogRun_TagWithoutChangelog_NoPush verifies tag-only with --no-push tags but does not push.
func TestChangelogRun_TagWithoutChangelog_NoPush(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag

	cfg := &pipeline.ChangelogConfig{
		Tag:    true,
		NoPush: true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[0].Args)
}

// TestChangelogRun_DryRun verifies no git mutations or generator calls in dry-run.
func TestChangelogRun_DryRun(t *testing.T) {
	mr := exectest.NewMockRunner()
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
	mr := exectest.NewMockRunner()
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

// TestChangelogRun_AnnotatedTag verifies git tag -a is used when AnnotatedTags is true.
func TestChangelogRun_AnnotatedTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.ChangelogConfig{
		Tag:           true,
		AnnotatedTags: true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "-a", "v1.2.3", "-m", "chore(release): 1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[1].Args)
}

// TestChangelogRun_AnnotatedTagCustomMessage verifies the annotation reuses the commit_message template.
func TestChangelogRun_AnnotatedTagCustomMessage(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.ChangelogConfig{
		Tag:           true,
		AnnotatedTags: true,
		CommitMessage: "docs: v${version}",
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	assert.Equal(t, []string{"tag", "-a", "v1.2.3", "-m", "docs: v1.2.3"}, mr.Calls[0].Args)
}

// TestChangelogRun_SignedTag verifies SignTags:true produces "git tag -s <tag> -m <msg>".
func TestChangelogRun_SignedTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag -s
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.ChangelogConfig{
		Tag:           true,
		SignTags:      true,
		AnnotatedTags: true, // should be ignored when signing
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "-s", "v1.2.3", "-m", "chore(release): 1.2.3"}, mr.Calls[0].Args)
	assert.NotContains(t, mr.Calls[0].Args, "-a")
}

// TestChangelogRun_ResolverError propagates resolver failures.
func TestChangelogRun_ResolverError(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &pipeline.ChangelogConfig{}

	p := pipeline.NewChangelog(mr, &fakeResolver{err: errors.New("no commits since last tag")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no commits")
}

// TestChangelogRun_DefaultCommitMessage verifies the default commit message contains the version.
func TestChangelogRun_DefaultCommitMessage(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	commitCall := mr.Calls[2]
	assert.Equal(t, "commit", commitCall.Args[0])
	assert.Equal(t, "-m", commitCall.Args[1])
	assert.Contains(t, commitCall.Args[2], "1.2.3")
}

// TestChangelogRun_CustomCommitMessage verifies commit message template substitution.
func TestChangelogRun_CustomCommitMessage(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
		CommitMessage: "docs: update changelog for v${version}",
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	assert.Equal(t, "docs: update changelog for v1.2.3", mr.Calls[2].Args[2])
}

// TestChangelogRun_DisabledChangelog_WithTag verifies that when DisableChangelog is true
// but Tag is also true, changelog generation is skipped but the tag is still created.
func TestChangelogRun_DisabledChangelog_WithTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:        gen,
		ChangelogFile:    "CHANGELOG.md",
		DisableChangelog: true,
		Tag:              true,
	}

	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, false)
	require.NoError(t, p.Run())

	assert.Len(t, gen.GenerateCalls, 0)
	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[1].Args)
	assert.Contains(t, out.String(), "disabled")
}

// TestChangelogRun_NothingToCommit verifies that when git add stages nothing (the
// regenerated changelog is byte-identical to the last commit), the commit and its push
// are skipped with a warning, and the tag is still created when Tag is set.
func TestChangelogRun_NothingToCommit(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git diff --cached --name-only (empty: nothing staged)
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Tag:           true,
	}

	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, false)
	require.NoError(t, p.Run())

	for _, c := range mr.Calls {
		require.NotEqual(t, "commit", c.Args[0], "git commit must be skipped when nothing is staged")
	}
	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[2].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[3].Args)

	assert.Contains(t, out.String(), "CHANGELOG.md unchanged")
}

// TestChangelogRun_GitAddError propagates git add failures.
func TestChangelogRun_GitAddError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("not a git repo")) // git add fails

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git add")
}

// TestChangelogRun_GitCommitError propagates git commit failures.
func TestChangelogRun_GitCommitError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)                             // git add OK
	mr.QueueResponse("CHANGELOG.md\n", "", nil)               // git diff --cached (staged)
	mr.QueueResponse("", "", errors.New("nothing to commit")) // git commit fails

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git commit")
}

// TestChangelogRun_GitPushError propagates git push failures.
func TestChangelogRun_GitPushError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)                         // git add OK
	mr.QueueResponse("CHANGELOG.md\n", "", nil)           // git diff --cached (staged)
	mr.QueueResponse("", "", nil)                         // git commit OK
	mr.QueueResponse("", "", errors.New("push rejected")) // git push fails

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git push")
}

// TestChangelogRun_DefaultChangelogFile verifies "CHANGELOG.md" is used when ChangelogFile is empty.
func TestChangelogRun_DefaultChangelogFile(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog: gen,
		// ChangelogFile intentionally empty
		Commit: true,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
}
