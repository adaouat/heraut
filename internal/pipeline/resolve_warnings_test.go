package pipeline_test

import (
	"bytes"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const heldBackWarning = "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (pass --allow-major to release 1.0.0)\n  - feat!: break"

func TestRun_PrintsResolveWarnings(t *testing.T) {
	var out bytes.Buffer
	res := resolvedResult("v0.69.0")
	res.Warnings = []string{heldBackWarning}
	cfg := &pipeline.Config{Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}}}

	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: res}, cfg, &out, true)
	require.NoError(t, p.Run())

	assert.Contains(t, out.String(), "! "+heldBackWarning+"\n")
}

func TestRun_NoResolveWarnings_PrintsNoWarningLine(t *testing.T) {
	var out bytes.Buffer
	cfg := &pipeline.Config{Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}}}

	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, true)
	require.NoError(t, p.Run())

	assert.NotContains(t, out.String(), "held back")
}

func TestChangelogRun_PrintsResolveWarnings(t *testing.T) {
	var out bytes.Buffer
	res := resolvedResult("v0.69.0")
	res.Warnings = []string{heldBackWarning}

	p := pipeline.NewChangelog(exectest.NewMockRunner(), &fakeResolver{result: res}, &pipeline.ChangelogConfig{}, &out, true)
	require.NoError(t, p.Run())

	assert.Contains(t, out.String(), "! "+heldBackWarning+"\n")
}
