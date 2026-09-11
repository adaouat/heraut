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
		PostBumpHooks: []string{"echo {{ .Version }}"},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "sh", mr.Calls[0].Name)
	assert.Equal(t, []string{"-c", "echo 1.2.3"}, mr.Calls[0].Args)
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
		PostBumpHooks:    []string{"echo post-bump"},
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
		PreChangelogHooks: []string{"echo pre-changelog"},
	}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls)
}

func TestChangelogRun_PreChangelogHook_FiresBeforeChangelogGeneration(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (pre_changelog)

	gen := &testutil.MockGenerator{}
	cfg := &pipeline.ChangelogConfig{
		Changelog:         gen,
		PreChangelogHooks: []string{"make lint"},
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
		PreTagHooks: []string{"go build ./..."},
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
		PostTagHooks: []string{"npm publish"},
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
		PostTagHooks: []string{"echo done"},
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
		PostBumpHooks: []string{"echo post-bump"},
		PreTagHooks:   []string{"echo pre-tag"},
		PostTagHooks:  []string{"echo post-tag"},
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
		PostBumpHooks: []string{"echo post-bump"},
		PreTagHooks:   []string{"echo pre-tag"},
		PostTagHooks:  []string{"echo post-tag"},
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
			cfg:  &pipeline.ChangelogConfig{Tag: true, PostBumpHooks: []string{"exit 1"}},
			queue: func(mr *exectest.MockRunner) {
				mr.QueueResponse("", "", errors.New("exit status 1"))
			},
			wantCall: 1,
		},
		{
			name: "pre_tag failure aborts before tag",
			cfg:  &pipeline.ChangelogConfig{Tag: true, PreTagHooks: []string{"exit 1"}},
			queue: func(mr *exectest.MockRunner) {
				mr.QueueResponse("", "", errors.New("exit status 1"))
			},
			wantCall: 1,
		},
		{
			name: "post_tag failure returned after tag+push already happened",
			cfg:  &pipeline.ChangelogConfig{Tag: true, PostTagHooks: []string{"exit 1"}},
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
