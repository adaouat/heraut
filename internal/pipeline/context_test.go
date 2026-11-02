package pipeline_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runKey struct{}

// The command's context, which Ctrl-C cancels, reaches every publish, so a driver can stop a request
// and clean up what it created.
func TestRun_PassesItsContextToEveryPublish(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	gl := &testutil.MockPlatform{PlatformName: "gitlab"}
	gh := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{Platforms: []port.Platform{gl, gh}}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)

	ctx := context.WithValue(t.Context(), runKey{}, "this run")
	require.NoError(t, p.Run(ctx))

	for _, plat := range []*testutil.MockPlatform{gl, gh} {
		require.Len(t, plat.CreateReleaseCalls, 1)
		assert.Equal(t, "this run", plat.CreateReleaseCalls[0].Ctx.Value(runKey{}), plat.PlatformName)
	}
}

// The preflight check gets the command's context too, so Ctrl-C stops a token check that stalls.
func TestCheck_PassesItsContextToEveryPlatform(t *testing.T) {
	gl := &testutil.MockPlatform{PlatformName: "gitlab"}
	gh := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{Platforms: []port.Platform{gl, gh}}
	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)

	ctx := context.WithValue(t.Context(), runKey{}, "this run")
	require.NoError(t, p.Check(ctx))

	for _, plat := range []*testutil.MockPlatform{gl, gh} {
		require.Len(t, plat.CheckContexts, 1)
		assert.Equal(t, "this run", plat.CheckContexts[0].Value(runKey{}), plat.PlatformName)
	}
}

// The changelog and the release notes fetch pull-request details from the forge: the command's
// context reaches every generation, so Ctrl-C stops those requests too.
func TestRun_PassesItsContextToEveryGeneration(t *testing.T) {
	for _, platforms := range []int{1, 2} {
		t.Run(fmt.Sprintf("%d platform(s)", platforms), func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse("", "", nil)               // git add
			mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only
			mr.QueueResponse("", "", nil)               // git commit
			mr.QueueResponse("", "", nil)               // git push
			mr.QueueResponse("", "", nil)               // git tag
			mr.QueueResponse("", "", nil)               // git push <tag>

			changelog, notes := &testutil.MockGenerator{}, &testutil.MockGenerator{}
			cfg := &pipeline.Config{Changelog: changelog, ChangelogFile: "CHANGELOG.md", Notes: notes}
			for i := range platforms {
				cfg.Platforms = append(cfg.Platforms, &testutil.MockPlatform{PlatformName: fmt.Sprintf("p%d", i)})
			}
			p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)

			ctx := context.WithValue(t.Context(), runKey{}, "this run")
			require.NoError(t, p.Run(ctx))

			require.Len(t, changelog.GenerateCtxs, 1)
			assert.Equal(t, "this run", changelog.GenerateCtxs[0].Value(runKey{}))
			require.Len(t, notes.GenerateCtxs, platforms)
			for _, c := range notes.GenerateCtxs {
				assert.Equal(t, "this run", c.Value(runKey{}))
			}
		})
	}
}

func TestChangelogRun_PassesItsContextToTheGenerator(t *testing.T) {
	gen := &testutil.MockGenerator{}
	cfg := &pipeline.ChangelogConfig{Changelog: gen, ChangelogFile: "CHANGELOG.md"}
	p := pipeline.NewChangelog(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false)

	ctx := context.WithValue(t.Context(), runKey{}, "this run")
	require.NoError(t, p.Run(ctx))

	require.Len(t, gen.GenerateCtxs, 1)
	assert.Equal(t, "this run", gen.GenerateCtxs[0].Value(runKey{}))
}
