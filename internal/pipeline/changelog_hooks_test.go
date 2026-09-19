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

func TestChangelogRun_PostBumpHook_FiresAfterResolve(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (post_bump)

	cfg := &pipeline.ChangelogConfig{
		PostBumpHooks: []pipeline.HookStep{{Run: "echo {{ .Version }}"}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "sh", mr.Calls[0].Name)
	assert.Equal(t, []string{"-c", "echo 1.2.3"}, mr.Calls[0].Args)
}

// TestChangelogRun_PostBumpHook_SubstitutesEnv proves cfg.Env reaches hookVars.Env here too —
// the changelog-only pipeline never sets Platform, but Env is available regardless.
func TestChangelogRun_PostBumpHook_SubstitutesEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (post_bump)

	cfg := &pipeline.ChangelogConfig{
		Env:           "staging",
		PostBumpHooks: []pipeline.HookStep{{Run: "echo {{ .Env }}"}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"-c", "echo staging"}, mr.Calls[0].Args)
}

// TestChangelogRun_PostBumpHook_FiresEvenWhenDisabledAndNoTag proves post_bump fires
// unconditionally on resolve (ADR-0053) — even in the one case where every other step is
// skipped and Run() returns immediately after resolving.
func TestChangelogRun_PostBumpHook_FiresEvenWhenDisabledAndNoTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (post_bump)

	cfg := &pipeline.ChangelogConfig{
		DisableChangelog: true,
		Tag:              false,
		PostBumpHooks:    []pipeline.HookStep{{Run: "echo post-bump"}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"-c", "echo post-bump"}, mr.Calls[0].Args)
}

// TestChangelogRun_PreChangelogHook_NeverFiresWhenChangelogDisabled proves pre_changelog is
// scoped to the changelog step actually running — unlike post_bump above.
func TestChangelogRun_PreChangelogHook_NeverFiresWhenChangelogDisabled(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &pipeline.ChangelogConfig{
		DisableChangelog:  true,
		Tag:               false,
		PreChangelogHooks: []pipeline.HookStep{{Run: "echo pre-changelog"}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls)
}

// TestChangelogRun_PostBumpHook_StagePatternIncludedInCommit proves a post_bump hook's declared
// Stage patterns (ADR-0061) reach the changelog commit's `git add` call alongside the changelog
// file itself, mirroring release.go's equivalent behavior.
func TestChangelogRun_PostBumpHook_StagePatternIncludedInCommit(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // sh -c (post_bump)
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push

	gen := &testutil.MockGenerator{}
	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		Commit:        true,
		PostBumpHooks: []pipeline.HookStep{{Run: "echo bumping", Stage: []string{"extra.txt"}}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 5)
	assert.Equal(t, []string{"add", "CHANGELOG.md", "extra.txt"}, mr.Calls[1].Args)
}

// TestChangelogRun_PostBumpAndPreChangelogHooks_StagePatternsCombinedInCommit proves both hook
// points' Stage patterns reach the same `git add` call, in the documented order — post_bump's
// patterns first, then pre_changelog's, mirroring release.go's equivalent behavior.
func TestChangelogRun_PostBumpAndPreChangelogHooks_StagePatternsCombinedInCommit(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // sh -c (post_bump)
	mr.QueueResponse("", "", nil)               // sh -c (pre_changelog)
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push

	gen := &testutil.MockGenerator{}
	cfg := &pipeline.ChangelogConfig{
		Changelog:         gen,
		Commit:            true,
		PostBumpHooks:     []pipeline.HookStep{{Run: "echo bumping", Stage: []string{"postbump.txt"}}},
		PreChangelogHooks: []pipeline.HookStep{{Run: "make lint", Stage: []string{"prechangelog.txt"}}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 6)
	assert.Equal(t, []string{"add", "CHANGELOG.md", "postbump.txt", "prechangelog.txt"}, mr.Calls[2].Args)
}

func TestChangelogRun_PreChangelogHook_FiresBeforeChangelogGeneration(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (pre_changelog)

	gen := &testutil.MockGenerator{}
	cfg := &pipeline.ChangelogConfig{
		Changelog:         gen,
		PreChangelogHooks: []pipeline.HookStep{{Run: "make lint"}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"-c", "make lint"}, mr.Calls[0].Args)
	require.Len(t, gen.GenerateCalls, 1)
}

func TestChangelogRun_PreTagHook_FiresBeforeTagCreation(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (pre_tag)
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.ChangelogConfig{
		Tag:         true,
		PreTagHooks: []pipeline.HookStep{{Run: "go build ./..."}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 3)
	assert.Equal(t, []string{"-c", "go build ./..."}, mr.Calls[0].Args)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[1].Args)
}

func TestChangelogRun_PostTagHook_FiresAfterPush(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>
	mr.QueueResponse("", "", nil) // sh -c (post_tag)

	cfg := &pipeline.ChangelogConfig{
		Tag:          true,
		PostTagHooks: []pipeline.HookStep{{Run: "npm publish"}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 3)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"-c", "npm publish"}, mr.Calls[2].Args)
}

func TestChangelogRun_PostTagHook_FiresEvenWithNoPush(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // sh -c (post_tag)

	cfg := &pipeline.ChangelogConfig{
		Tag:          true,
		NoPush:       true,
		PostTagHooks: []pipeline.HookStep{{Run: "echo done"}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"-c", "echo done"}, mr.Calls[1].Args)
}

func TestChangelogRun_Hooks_NoHooksSkipsAllConfiguredHooks(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.ChangelogConfig{
		Tag:           true,
		NoHooks:       true,
		PostBumpHooks: []pipeline.HookStep{{Run: "echo post-bump"}},
		PreTagHooks:   []pipeline.HookStep{{Run: "echo pre-tag"}},
		PostTagHooks:  []pipeline.HookStep{{Run: "echo post-tag"}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, "git", mr.Calls[1].Name)
}

func TestChangelogRun_Hooks_DryRunNeverExecutesHooks(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &pipeline.ChangelogConfig{
		Tag:           true,
		PostBumpHooks: []pipeline.HookStep{{Run: "echo post-bump"}},
		PreTagHooks:   []pipeline.HookStep{{Run: "echo pre-tag"}},
		PostTagHooks:  []pipeline.HookStep{{Run: "echo post-tag"}},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true)
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls, "dry-run must never execute a hook for real")
}

func TestChangelogRun_Hooks_FailureAbortsRun(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *pipeline.ChangelogConfig
		queue    func(mr *exectest.MockRunner)
		wantCall int
	}{
		{
			name: "post_bump failure aborts before anything else",
			cfg:  &pipeline.ChangelogConfig{Tag: true, PostBumpHooks: []pipeline.HookStep{{Run: "exit 1"}}},
			queue: func(mr *exectest.MockRunner) {
				mr.QueueResponse("", "", errors.New("exit status 1"))
			},
			wantCall: 1,
		},
		{
			name: "pre_tag failure aborts before tag",
			cfg:  &pipeline.ChangelogConfig{Tag: true, PreTagHooks: []pipeline.HookStep{{Run: "exit 1"}}},
			queue: func(mr *exectest.MockRunner) {
				mr.QueueResponse("", "", errors.New("exit status 1"))
			},
			wantCall: 1,
		},
		{
			name: "post_tag failure returned after tag+push already happened",
			cfg:  &pipeline.ChangelogConfig{Tag: true, PostTagHooks: []pipeline.HookStep{{Run: "exit 1"}}},
			queue: func(mr *exectest.MockRunner) {
				mr.QueueResponse("", "", nil) // git tag
				mr.QueueResponse("", "", nil) // git push
				mr.QueueResponse("", "", errors.New("exit status 1"))
			},
			wantCall: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			tc.queue(mr)

			p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, tc.cfg, &bytes.Buffer{}, false)
			err := p.Run()
			require.Error(t, err)
			assert.Len(t, mr.Calls, tc.wantCall)
		})
	}
}
