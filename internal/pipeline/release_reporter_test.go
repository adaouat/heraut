package pipeline_test

// Reporter-specific tests for Pipeline.Run — T41.
// All helpers (fakeResolver, resolvedResult) live in release_test.go.
//
// capturedStep records the name and result a StepFn receives.
// capturingStepFn builds a StepFn that records every call without printing.

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedStep struct {
	name   string
	result string
	subs   []string
	err    error
}

func capturingStepFn(out *[]capturedStep) ui.StepFn {
	return func(name string, fn func() (string, []string, error)) error {
		result, subs, err := fn()
		*out = append(*out, capturedStep{name: name, result: result, subs: subs, err: err})
		return err
	}
}

func stepNames(steps []capturedStep) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.name
	}
	return names
}

// TestRun_Reporter_MinimalSequence verifies step names and order for the simplest
// release (no changelog, no notes, one platform without assets).
func TestRun_Reporter_MinimalSequence(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	names := stepNames(captured)
	require.Equal(t, []string{
		"Resolve version",
		"Create tag v1.2.3",
		"Push tag",
		"Publish to github",
	}, names)

	// Resolve step result is the tag.
	assert.Equal(t, "v1.2.3", captured[0].result)

	// Platform publish result is the release URL.
	assert.Equal(t, "https://example.com/releases/v1.2.3", captured[len(captured)-1].result)
}

// TestRun_Reporter_WithChangelog verifies the generate + commit steps appear between
// resolve and tag when changelog is configured.
func TestRun_Reporter_WithChangelog(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git commit
	mr.QueueResponse("", "", nil) // git push
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{platform},
	}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	names := stepNames(captured)
	require.Equal(t, []string{
		"Resolve version",
		"Generate changelog",
		"Commit changelog",
		"Create tag v1.2.3",
		"Push tag",
		"Publish to github",
	}, names)
}

// TestRun_Reporter_WithNotes verifies the release-notes step appears between
// push-tag and publish-to-platform.
func TestRun_Reporter_WithNotes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	notes := &testutil.MockGenerator{GenerateOut: "## Notes\n"}
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{
		Notes:     notes,
		Platforms: []port.Platform{platform},
	}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	names := stepNames(captured)
	require.Equal(t, []string{
		"Resolve version",
		"Create tag v1.2.3",
		"Push tag",
		"Generate release notes",
		"Publish to github",
	}, names)
}

// TestRun_Reporter_AssetUploadSubResult verifies that a platform with assets
// produces a sub-result "assets uploaded" and no extra numbered step.
func TestRun_Reporter_AssetUploadSubResult(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	platform := &testutil.MockPlatform{PlatformName: "github", HasAssetsVal: true}
	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	names := stepNames(captured)
	// Publish to github is still a single numbered step (no extra step for assets).
	assert.Equal(t, []string{"Resolve version", "Create tag v1.2.3", "Push tag", "Publish to github"}, names)

	// The publish step carries "assets uploaded" as a sub-result.
	publishStep := captured[len(captured)-1]
	require.NotEmpty(t, publishStep.subs, "expected sub-results for asset upload")
	assert.Equal(t, "assets uploaded", publishStep.subs[0])
}

// TestRun_Reporter_MultiplePlatformsEachGetStep verifies every platform
// receives its own numbered step.
func TestRun_Reporter_MultiplePlatformsEachGetStep(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	p1 := &testutil.MockPlatform{PlatformName: "github"}
	p2 := &testutil.MockPlatform{PlatformName: "gitlab"}
	cfg := &pipeline.Config{Platforms: []port.Platform{p1, p2}}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	names := stepNames(captured)
	assert.Contains(t, names, "Publish to github")
	assert.Contains(t, names, "Publish to gitlab")
}

// TestRun_Reporter_MultiPlatform_NotesFoldedIntoPublish verifies that with >1 platform
// the standalone "Generate release notes" step is omitted and notes generation folds into
// each publish step as a "notes generated" sub-result (ADR-0021 step semantics).
func TestRun_Reporter_MultiPlatform_NotesFoldedIntoPublish(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	notes := &testutil.MockGenerator{GenerateOut: "## notes\n"}
	p1 := &testutil.MockPlatform{PlatformName: "github"}
	p2 := &testutil.MockPlatform{PlatformName: "gitlab"}
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{p1, p2}}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	names := stepNames(captured)
	assert.NotContains(t, names, "Generate release notes", "multi-platform must not emit a standalone notes step")
	assert.Contains(t, names, "Publish to github")
	assert.Contains(t, names, "Publish to gitlab")

	// Each publish step carries a "notes generated" sub-result.
	for _, s := range captured {
		if s.name == "Publish to github" || s.name == "Publish to gitlab" {
			assert.Contains(t, s.subs, "notes generated", "expected notes sub-result on %s", s.name)
		}
	}
}

// TestRun_Reporter_DegradedNotes_EmitsSubResult verifies that when the notes generator
// fell back to --offline (remote_metadata: optional, fetch failed), the notes step carries
// a degraded sub-result so the omission of PR authors/numbers is visible (T78).
func TestRun_Reporter_DegradedNotes_EmitsSubResult(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	notes := &testutil.MockGenerator{GenerateOut: "## notes\n", DegradedVal: true}
	p1 := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{p1}}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	var notesStep *capturedStep
	for i := range captured {
		if captured[i].name == "Generate release notes" {
			notesStep = &captured[i]
		}
	}
	require.NotNil(t, notesStep, "expected a Generate release notes step")
	assert.Contains(t, notesStep.subs, "remote metadata unavailable — PR authors/numbers omitted")
}

// TestRun_BuildTag_PropagatesToPlatform verifies a build-rendered tag (T57 — heraut
// release --build) flows unchanged from the resolver to the platform's CreateRelease.
func TestRun_BuildTag_PropagatesToPlatform(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	notes := &testutil.MockGenerator{GenerateOut: "## notes\n"}
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{platform}}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("uat/7.4.1-158404")}, cfg, &bytes.Buffer{}, false)
	require.NoError(t, p.Run())

	require.Len(t, platform.CreateReleaseCalls, 1)
	assert.Equal(t, "uat/7.4.1-158404", platform.CreateReleaseCalls[0].Tag)
}

// TestRun_Reporter_SinglePlatform_StandaloneNotesStep verifies the single-platform path
// keeps the standalone "Generate release notes" step (non-regression).
func TestRun_Reporter_SinglePlatform_StandaloneNotesStep(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	notes := &testutil.MockGenerator{GenerateOut: "## notes\n"}
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{platform}}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, false).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	assert.Contains(t, stepNames(captured), "Generate release notes")
}

// TestRun_DryRun_MultiPlatform_NotesFolded verifies the dry-run path mirrors the live
// step structure: no standalone notes step, "would generate notes" folded into publish,
// and no real generator calls.
func TestRun_DryRun_MultiPlatform_NotesFolded(t *testing.T) {
	mr := exectest.NewMockRunner()
	notes := &testutil.MockGenerator{GenerateOut: "n"}
	p1 := &testutil.MockPlatform{PlatformName: "github"}
	p2 := &testutil.MockPlatform{PlatformName: "gitlab"}
	cfg := &pipeline.Config{Notes: notes, Platforms: []port.Platform{p1, p2}}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	assert.NotContains(t, stepNames(captured), "Generate release notes")
	assert.Len(t, notes.GenerateCalls, 0, "dry-run must not call the generator")
	for _, s := range captured {
		if s.name == "Publish to github" || s.name == "Publish to gitlab" {
			assert.Contains(t, s.subs, "[dry-run] would generate notes")
		}
	}
}

// TestRun_Reporter_DryRunSequence verifies that dry-run with a reporter shows
// all expected steps but makes no real git or platform calls.
func TestRun_Reporter_DryRunSequence(t *testing.T) {
	mr := exectest.NewMockRunner()
	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{
		Changelog:     changelog,
		ChangelogFile: "CHANGELOG.md",
		Platforms:     []port.Platform{platform},
	}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))

	require.NoError(t, p.Run())

	// No real git or platform calls.
	assert.Len(t, mr.Calls, 0)
	assert.Len(t, platform.CreateReleaseCalls, 0)

	// All expected steps appear.
	names := stepNames(captured)
	assert.Contains(t, names, "Resolve version")
	assert.Contains(t, names, "Generate changelog")
	assert.Contains(t, names, "Commit changelog")
	assert.Contains(t, names, fmt.Sprintf("Create tag %s", "v1.2.3"))
	assert.Contains(t, names, "Push tag")
	assert.Contains(t, names, "Publish to github")
}

// TestRun_Reporter_SummaryContainsTag verifies that after a successful run
// the output contains the released tag (regression guard on summary format).
func TestRun_Reporter_SummaryContainsTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

	out := &bytes.Buffer{}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, false).
		WithReporter(capturingStepFn(new([]capturedStep)))

	require.NoError(t, p.Run())
	assert.Contains(t, out.String(), "v1.2.3")
}

// TestRun_Reporter_NilIsTransparent documents that a nil reporter is identical
// to not calling WithReporter: no output beyond the summary line.
func TestRun_Reporter_NilIsTransparent(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push --tags

	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{Platforms: []port.Platform{platform}}

	// Without reporter
	out1 := &bytes.Buffer{}
	p1 := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out1, false)

	mr2 := exectest.NewMockRunner()
	mr2.QueueResponse("", "", nil)
	mr2.QueueResponse("", "", nil)
	out2 := &bytes.Buffer{}
	p2 := pipeline.New(mr2, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out2, false)

	require.NoError(t, p1.Run())
	require.NoError(t, p2.Run())

	// Both produce the same output when no reporter is set.
	assert.Equal(t, out1.String(), out2.String())
}
