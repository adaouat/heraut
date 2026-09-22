package pipeline_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const heldBackWarning = "synthetic held-back warning for pipeline printing tests — not production wording\n  - feat!: break"

const (
	firstResolveWarning  = "first resolver warning"
	secondResolveWarning = "second resolver warning\n  - detail line"
)

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
	for _, line := range strings.Split(out.String(), "\n") {
		assert.False(t, strings.HasPrefix(line, "! "), "no warning line expected, got %q", line)
	}
}

func TestChangelogRun_PrintsResolveWarnings(t *testing.T) {
	var out bytes.Buffer
	res := resolvedResult("v0.69.0")
	res.Warnings = []string{heldBackWarning}

	p := pipeline.NewChangelog(exectest.NewMockRunner(), &fakeResolver{result: res}, &pipeline.ChangelogConfig{}, &out, true)
	require.NoError(t, p.Run())

	assert.Contains(t, out.String(), "! "+heldBackWarning+"\n")
}

func TestRun_PrintsSeveralResolveWarningsInOrder(t *testing.T) {
	var out bytes.Buffer
	res := resolvedResult("v0.69.0")
	res.Warnings = []string{firstResolveWarning, secondResolveWarning}
	cfg := &pipeline.Config{Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}}}

	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: res}, cfg, &out, true)
	require.NoError(t, p.Run())

	first := strings.Index(out.String(), "! "+firstResolveWarning+"\n")
	second := strings.Index(out.String(), "! "+secondResolveWarning+"\n")
	require.NotEqual(t, -1, first, "first warning printed as its own headline:\n%s", out.String())
	require.NotEqual(t, -1, second, "second warning printed as its own headline:\n%s", out.String())
	assert.Less(t, first, second, "warnings print in the order the resolver produced them")
}

func TestRun_NonDryRun_PrintsResolveWarnings(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>
	platform := &testutil.MockPlatform{PlatformName: "github"}
	res := resolvedResult("v0.69.0")
	res.Warnings = []string{firstResolveWarning, secondResolveWarning}
	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

	var out bytes.Buffer
	p := pipeline.New(mr, &fakeResolver{result: res}, cfg, &out, false)
	require.NoError(t, p.Run())

	require.Len(t, platform.CreateReleaseCalls, 1, "the run really executed rather than stopping at a dry-run")
	first := strings.Index(out.String(), "! "+firstResolveWarning+"\n")
	second := strings.Index(out.String(), "! "+secondResolveWarning+"\n")
	require.NotEqual(t, -1, first, "first warning printed:\n%s", out.String())
	require.NotEqual(t, -1, second, "second warning printed:\n%s", out.String())
	assert.Less(t, first, second)
}

func TestChangelogRun_PrintsSeveralResolveWarningsInOrder(t *testing.T) {
	var out bytes.Buffer
	res := resolvedResult("v0.69.0")
	res.Warnings = []string{firstResolveWarning, secondResolveWarning}

	p := pipeline.NewChangelog(exectest.NewMockRunner(), &fakeResolver{result: res}, &pipeline.ChangelogConfig{}, &out, true)
	require.NoError(t, p.Run())

	first := strings.Index(out.String(), "! "+firstResolveWarning+"\n")
	second := strings.Index(out.String(), "! "+secondResolveWarning+"\n")
	require.NotEqual(t, -1, first, "first warning printed as its own headline:\n%s", out.String())
	require.NotEqual(t, -1, second, "second warning printed as its own headline:\n%s", out.String())
	assert.Less(t, first, second, "warnings print in the order the resolver produced them")
}

// With changelog generation disabled and no --tag, Run returns straight after the post_bump hooks,
// so the warning has to be printed before that early return, not after it.
func TestChangelogRun_DisableChangelogWithoutTag_PrintsWarningBeforeDisabledLine(t *testing.T) {
	mr := exectest.NewMockRunner()
	gen := &testutil.MockGenerator{}
	res := resolvedResult("v0.69.0")
	res.Warnings = []string{heldBackWarning}
	cfg := &pipeline.ChangelogConfig{Changelog: gen, ChangelogFile: "CHANGELOG.md", DisableChangelog: true, Tag: false}

	var out bytes.Buffer
	p := pipeline.NewChangelog(mr, &fakeResolver{result: res}, cfg, &out, false)
	require.NoError(t, p.Run())

	assert.Empty(t, gen.GenerateCalls)
	assert.Empty(t, mr.Calls)
	warning := strings.Index(out.String(), "! "+heldBackWarning+"\n")
	disabled := strings.Index(out.String(), "changelog disabled")
	require.NotEqual(t, -1, warning, "warning printed on the early-return path:\n%s", out.String())
	require.NotEqual(t, -1, disabled, "the disabled line is printed:\n%s", out.String())
	assert.Less(t, warning, disabled, "warning comes before the changelog-disabled line")
}
