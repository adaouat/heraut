package pipeline_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_PostBumpHook_FiresAfterResolve(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (post_bump)
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.Config{
		PostBumpHooks: []string{"echo {{ .Version }}"},
		Platforms:     []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 3)
	assert.Equal(t, "sh", mr.Calls[0].Name)
	assert.Equal(t, []string{"-c", "echo 1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, "git", mr.Calls[1].Name)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[1].Args)
}

func TestRun_PreChangelogHook_FiresBeforeChangelogGeneration(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (pre_changelog)
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git diff --cached (no staged changes)
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	gen := &testutil.MockGenerator{}
	cfg := &pipeline.Config{
		Changelog:         gen,
		PreChangelogHooks: []string{"make lint"},
		Platforms:         []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 5)
	assert.Equal(t, "sh", mr.Calls[0].Name)
	assert.Equal(t, []string{"-c", "make lint"}, mr.Calls[0].Args)
	assert.Equal(t, "git", mr.Calls[1].Name)
	assert.Equal(t, "add", mr.Calls[1].Args[0])
	require.Len(t, gen.GenerateCalls, 1)
}

func TestRun_PreTagHook_FiresBeforeTagCreation(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (pre_tag)
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.Config{
		PreTagHooks: []string{"go build ./..."},
		Platforms:   []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 3)
	assert.Equal(t, []string{"-c", "go build ./..."}, mr.Calls[0].Args)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[1].Args)
}

func TestRun_PostTagHook_FiresAfterPush(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>
	mr.QueueResponse("", "", nil) // sh -c (post_tag)

	cfg := &pipeline.Config{
		PostTagHooks: []string{"npm publish"},
		Platforms:    []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 3)
	assert.Equal(t, []string{"tag", "v1.2.3"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"push", "origin", "v1.2.3"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"-c", "npm publish"}, mr.Calls[2].Args)
}

func TestRun_Hooks_NoHooksSkipsAllConfiguredHooks(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.Config{
		NoHooks:       true,
		PostBumpHooks: []string{"echo post-bump"},
		PreTagHooks:   []string{"echo pre-tag"},
		PostTagHooks:  []string{"echo post-tag"},
		Platforms:     []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, "git", mr.Calls[1].Name)
}

func TestRun_Hooks_DryRunNeverExecutesHooks(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &pipeline.Config{
		PostBumpHooks: []string{"echo post-bump"},
		PreTagHooks:   []string{"echo pre-tag"},
		PostTagHooks:  []string{"echo post-tag"},
		Platforms:     []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true)
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls, "dry-run must never execute a hook for real")
}

func TestRun_Hooks_FailureAbortsRun(t *testing.T) {
	tests := []struct {
		name     string
		cfg      func() *pipeline.Config
		queue    func(mr *exectest.MockRunner)
		wantCall int // number of MockRunner calls expected before abort
	}{
		{
			name: "post_bump failure aborts before tag",
			cfg: func() *pipeline.Config {
				return &pipeline.Config{PostBumpHooks: []string{"exit 1"}}
			},
			queue: func(mr *exectest.MockRunner) {
				mr.QueueResponse("", "", errors.New("exit status 1"))
			},
			wantCall: 1,
		},
		{
			name: "pre_tag failure aborts before tag",
			cfg: func() *pipeline.Config {
				return &pipeline.Config{PreTagHooks: []string{"exit 1"}}
			},
			queue: func(mr *exectest.MockRunner) {
				mr.QueueResponse("", "", errors.New("exit status 1"))
			},
			wantCall: 1,
		},
		{
			name: "post_tag failure returned after tag+push already happened",
			cfg: func() *pipeline.Config {
				return &pipeline.Config{PostTagHooks: []string{"exit 1"}}
			},
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
			cfg := tc.cfg()
			cfg.Platforms = []port.Platform{&testutil.MockPlatform{PlatformName: "github"}}

			p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)
			err := p.Run()
			require.Error(t, err)
			assert.Len(t, mr.Calls, tc.wantCall)
		})
	}
}
