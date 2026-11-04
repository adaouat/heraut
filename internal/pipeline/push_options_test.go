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

var (
	commitOpts = []string{"ci.skip"}
	tagOpts    = []string{"ci.variable=DEPLOY=true", "-x"}
)

// queueCommitAndTag queues add → diff --cached (staged) → commit → push HEAD → tag → push <tag>.
func queueCommitAndTag(mr *exectest.MockRunner) {
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("CHANGELOG.md\n", "", nil)
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("", "", nil)
}

func TestRun_PushOptions_ReachBothPushes(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueCommitAndTag(mr)
	cfg := &pipeline.Config{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Platforms:         []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
		CommitPushOptions: commitOpts,
		TagPushOptions:    tagOpts,
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run(t.Context()))

	require.Len(t, mr.Calls, 6)
	assert.Equal(t, []string{"push", "--push-option=ci.skip", "origin", "HEAD"}, mr.Calls[3].Args)
	assert.Equal(t, []string{"push", "--push-option=ci.variable=DEPLOY=true", "--push-option=-x", "origin", "v1.2.3"}, mr.Calls[5].Args)
}

func TestChangelogRun_PushOptions_ReachBothPushes(t *testing.T) {
	mr := exectest.NewMockRunner()
	queueCommitAndTag(mr)
	cfg := &pipeline.ChangelogConfig{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Tag:               true,
		CommitPushOptions: commitOpts,
		TagPushOptions:    tagOpts,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run(t.Context()))

	require.Len(t, mr.Calls, 6)
	assert.Equal(t, []string{"push", "--push-option=ci.skip", "origin", "HEAD"}, mr.Calls[3].Args)
	assert.Equal(t, []string{"push", "--push-option=ci.variable=DEPLOY=true", "--push-option=-x", "origin", "v1.2.3"}, mr.Calls[5].Args)
}

func TestChangelogRun_PushOptions_NoPushPushesNothing(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git tag
	cfg := &pipeline.ChangelogConfig{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Tag:               true,
		NoPush:            true,
		CommitPushOptions: commitOpts,
		TagPushOptions:    tagOpts,
	}

	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run(t.Context()))

	for _, c := range mr.Calls {
		assert.NotEqual(t, "push", c.Args[0])
	}
}

func TestRun_DryRun_PushOptionsInPlainLines(t *testing.T) {
	cfg := &pipeline.Config{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Platforms:         []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
		CommitPushOptions: commitOpts,
		TagPushOptions:    tagOpts,
	}
	var out bytes.Buffer
	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, true)
	require.NoError(t, p.Run(t.Context()))

	assert.Contains(t, out.String(), "[dry-run] would generate changelog → commit → push (--push-option=ci.skip)\n")
	assert.Contains(t, out.String(), "[dry-run] would tag v1.2.3 and push (--push-option=ci.variable=DEPLOY=true --push-option=-x)\n")
}

func TestRun_DryRun_PushOptionsInReporterLines(t *testing.T) {
	cfg := &pipeline.Config{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Platforms:         []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
		CommitPushOptions: commitOpts,
		TagPushOptions:    tagOpts,
	}
	var captured []capturedStep
	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run(t.Context()))

	results := map[string]string{}
	for _, s := range captured {
		results[s.name] = s.result
	}
	assert.Equal(t, "[dry-run] would commit and push (--push-option=ci.skip)", results["Commit changelog"])
	assert.Equal(t, "[dry-run] would push v1.2.3 (--push-option=ci.variable=DEPLOY=true --push-option=-x)", results["Push tag"])
}

func TestRun_DryRun_NoPushOptionsKeepsTheLines(t *testing.T) {
	cfg := &pipeline.Config{
		Changelog:     &testutil.MockGenerator{},
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}
	var out bytes.Buffer
	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, true)
	require.NoError(t, p.Run(t.Context()))

	assert.Contains(t, out.String(), "[dry-run] would generate changelog → commit → push\n")
	assert.Contains(t, out.String(), "[dry-run] would tag v1.2.3 and push\n")
}

func TestChangelogRun_DryRun_PushOptionsInPlainLines(t *testing.T) {
	cfg := &pipeline.ChangelogConfig{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Tag:               true,
		CommitPushOptions: commitOpts,
		TagPushOptions:    tagOpts,
	}
	var out bytes.Buffer
	p := pipeline.NewChangelog(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, true)
	require.NoError(t, p.Run(t.Context()))

	assert.Contains(t, out.String(), "[dry-run] would commit → push (--push-option=ci.skip)\n")
	assert.Contains(t, out.String(), "[dry-run] would tag v1.2.3 and push (--push-option=ci.variable=DEPLOY=true --push-option=-x)\n")
}

func TestChangelogRun_DryRun_PushOptionsInReporterLines(t *testing.T) {
	cfg := &pipeline.ChangelogConfig{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Tag:               true,
		CommitPushOptions: commitOpts,
		TagPushOptions:    tagOpts,
	}
	var captured []capturedStep
	p := pipeline.NewChangelog(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run(t.Context()))

	results := map[string]string{}
	for _, s := range captured {
		results[s.name] = s.result
	}
	assert.Equal(t, "[dry-run] would commit and push (--push-option=ci.skip)", results["Commit changelog"])
	assert.Equal(t, "[dry-run] would push v1.2.3 (--push-option=ci.variable=DEPLOY=true --push-option=-x)", results["Push tag"])
}

func TestChangelogRun_DryRun_NoPushHidesPushOptions(t *testing.T) {
	cfg := &pipeline.ChangelogConfig{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Tag:               true,
		NoPush:            true,
		CommitPushOptions: commitOpts,
		TagPushOptions:    tagOpts,
	}
	var out bytes.Buffer
	p := pipeline.NewChangelog(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, true)
	require.NoError(t, p.Run(t.Context()))

	assert.NotContains(t, out.String(), "--push-option")
}

func TestPlan_PushOptionsOnPushSteps(t *testing.T) {
	cfg := &pipeline.Config{
		Changelog:         &testutil.MockGenerator{},
		ChangelogFile:     "CHANGELOG.md",
		Platforms:         []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
		CommitPushOptions: commitOpts,
		TagPushOptions:    tagOpts,
	}

	plan, _, err := planFor(t, cfg, resolvedResult("v1.2.3"))
	require.NoError(t, err)

	byAction := map[string]pipeline.PlanStep{}
	for _, s := range plan.Steps {
		byAction[s.Action] = s
	}
	assert.Equal(t, commitOpts, byAction["commit_changelog"].PushOptions)
	assert.Equal(t, tagOpts, byAction["push_tag"].PushOptions)
	assert.Empty(t, byAction["tag"].PushOptions, "only the push steps carry options")
}
