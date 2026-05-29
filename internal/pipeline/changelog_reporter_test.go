package pipeline_test

// Reporter-specific tests for ChangelogPipeline.Run — T42.
// capturedStep, capturingStepFn, and stepNames are defined in
// release_reporter_test.go (same package pipeline_test).

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChangelogRun_Reporter_GenerateOnly verifies step sequence when only
// changelog generation is configured (no commit, no tag).
func TestChangelogRun_Reporter_GenerateOnly(t *testing.T) {
	mr := testutil.NewMockRunner()
	gen := &testutil.MockGenerator{GenerateOut: "content"}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
	}

	var captured []capturedStep
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	assert.Equal(t, []string{"Resolve version", "Generate changelog"}, stepNames(captured))
	assert.Equal(t, "v1.2.3", captured[0].result)
}

// TestChangelogRun_Reporter_WithCommit verifies generate + commit steps appear
// when Commit is true.
func TestChangelogRun_Reporter_WithCommit(t *testing.T) {
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

	var captured []capturedStep
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	assert.Equal(t, []string{
		"Resolve version",
		"Generate changelog",
		"Commit changelog",
	}, stepNames(captured))
}

// TestChangelogRun_Reporter_WithTag verifies the full step sequence when Tag is true.
func TestChangelogRun_Reporter_WithTag(t *testing.T) {
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

	var captured []capturedStep
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	assert.Equal(t, []string{
		"Resolve version",
		"Generate changelog",
		"Commit changelog",
		fmt.Sprintf("Create tag %s", "v1.2.3"),
		"Push tags",
	}, stepNames(captured))
}

// TestChangelogRun_Reporter_TagWithoutChangelog verifies tag-only when no generator
// is configured: resolve + create tag + push tags.
func TestChangelogRun_Reporter_TagWithoutChangelog(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	cfg := &pipeline.ChangelogConfig{Tag: true}

	var captured []capturedStep
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	assert.Equal(t, []string{
		"Resolve version",
		"Create tag v1.2.3",
		"Push tags",
	}, stepNames(captured))
}

// TestChangelogRun_Reporter_DryRunSequence verifies that dry-run with a reporter
// shows all expected steps but makes no real git or generator calls.
func TestChangelogRun_Reporter_DryRunSequence(t *testing.T) {
	mr := testutil.NewMockRunner()
	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
		Commit:        true,
		Tag:           true,
	}

	var captured []capturedStep
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	// No real operations.
	assert.Len(t, mr.Calls, 0)
	assert.Len(t, gen.GenerateCalls, 0)

	// All expected steps appear.
	names := stepNames(captured)
	assert.Contains(t, names, "Resolve version")
	assert.Contains(t, names, "Generate changelog")
	assert.Contains(t, names, "Commit changelog")
	assert.Contains(t, names, fmt.Sprintf("Create tag %s", "v1.2.3"))
	assert.Contains(t, names, "Push tags")
}

// TestChangelogRun_Reporter_DisabledChangelog verifies that the output contains
// "disabled" when DisableChangelog is true, regardless of reporter.
func TestChangelogRun_Reporter_DisabledChangelog(t *testing.T) {
	mr := testutil.NewMockRunner()
	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:        gen,
		ChangelogFile:    "CHANGELOG.md",
		DisableChangelog: true,
	}

	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, false).
		WithReporter(capturingStepFn(new([]capturedStep)))

	require.NoError(t, p.Run())

	assert.Len(t, gen.GenerateCalls, 0)
	assert.Len(t, mr.Calls, 0)
	assert.Contains(t, out.String(), "disabled")
}

// TestChangelogRun_Reporter_DisabledChangelog_WithTag verifies that when both
// DisableChangelog and Tag are true, only resolve + tag steps appear in the reporter.
func TestChangelogRun_Reporter_DisabledChangelog_WithTag(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:        gen,
		ChangelogFile:    "CHANGELOG.md",
		DisableChangelog: true,
		Tag:              true,
	}

	var captured []capturedStep
	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	assert.Len(t, gen.GenerateCalls, 0)
	assert.Equal(t, []string{
		"Resolve version",
		"Create tag v1.2.3",
		"Push tags",
	}, stepNames(captured))
	assert.Contains(t, out.String(), "disabled")
}

// TestChangelogRun_DryRun_DisabledChangelog_WithTag verifies that dry-run with
// DisableChangelog+Tag shows only the tag steps, not the changelog steps.
func TestChangelogRun_DryRun_DisabledChangelog_WithTag(t *testing.T) {
	mr := testutil.NewMockRunner()
	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:        gen,
		ChangelogFile:    "CHANGELOG.md",
		DisableChangelog: true,
		Tag:              true,
	}

	var captured []capturedStep
	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, true).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	assert.Len(t, mr.Calls, 0)
	assert.Len(t, gen.GenerateCalls, 0)
	names := stepNames(captured)
	assert.NotContains(t, names, "Generate changelog")
	assert.NotContains(t, names, "Commit changelog")
	assert.Contains(t, names, fmt.Sprintf("Create tag %s", "v1.2.3"))
	assert.Contains(t, names, "Push tags")
}

// TestChangelogRun_Reporter_SummaryContainsTag verifies the output contains the
// tag after a successful run (regression guard on summary format).
func TestChangelogRun_Reporter_SummaryContainsTag(t *testing.T) {
	mr := testutil.NewMockRunner()
	gen := &testutil.MockGenerator{}

	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
	}

	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, false).
		WithReporter(capturingStepFn(new([]capturedStep)))

	require.NoError(t, p.Run())
	assert.Contains(t, out.String(), "v1.2.3")
}

// TestChangelogRun_Reporter_NilIsTransparent documents that nil reporter produces
// the same output as a pipeline created without WithReporter.
func TestChangelogRun_Reporter_NilIsTransparent(t *testing.T) {
	gen := &testutil.MockGenerator{}
	cfg := &pipeline.ChangelogConfig{
		Changelog:     gen,
		ChangelogFile: "CHANGELOG.md",
	}

	mr1 := testutil.NewMockRunner()
	out1 := &bytes.Buffer{}
	p1 := pipeline.NewChangelog(mr1, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out1, false)

	mr2 := testutil.NewMockRunner()
	out2 := &bytes.Buffer{}
	p2 := pipeline.NewChangelog(mr2, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out2, false)

	require.NoError(t, p1.Run())
	require.NoError(t, p2.Run())

	assert.Equal(t, out1.String(), out2.String())
}
