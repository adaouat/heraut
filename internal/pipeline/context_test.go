package pipeline_test

import (
	"bytes"
	"context"
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
