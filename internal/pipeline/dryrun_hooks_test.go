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

func TestRun_DryRun_AllFourSharedHookSteps(t *testing.T) {
	mr := exectest.NewMockRunner()
	changelog := &testutil.MockGenerator{}
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{
		Changelog:         changelog,
		ChangelogFile:     "CHANGELOG.md",
		Platforms:         []port.Platform{platform},
		PostBumpHooks:     []pipeline.HookStep{{Run: "echo {{ .Version }}"}},
		PreChangelogHooks: []pipeline.HookStep{{Run: "make lint"}},
		PreTagHooks:       []pipeline.HookStep{{Run: "go build ./..."}},
		PostTagHooks:      []pipeline.HookStep{{Run: "npm publish"}},
	}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls, "dry-run must not execute anything for real")

	names := stepNames(captured)
	assert.Contains(t, names, "Run post_bump hooks")
	assert.Contains(t, names, "Run pre_changelog hooks")
	assert.Contains(t, names, "Run pre_tag hooks")
	assert.Contains(t, names, "Run post_tag hooks")

	byName := map[string]capturedStep{}
	for _, s := range captured {
		byName[s.name] = s
	}
	assert.Equal(t, "[dry-run] would run: echo 1.2.3", byName["Run post_bump hooks"].result)
	assert.Equal(t, "[dry-run] would run: make lint", byName["Run pre_changelog hooks"].result)
	assert.Equal(t, "[dry-run] would run: go build ./...", byName["Run pre_tag hooks"].result)
	assert.Equal(t, "[dry-run] would run: npm publish", byName["Run post_tag hooks"].result)
}

func TestRun_DryRun_PreReleaseAndPostReleaseHooks_FoldedIntoPublishStep(t *testing.T) {
	mr := exectest.NewMockRunner()
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{
		Platforms:        []port.Platform{platform},
		PreReleaseHooks:  []pipeline.HookStep{{Run: "echo about to publish to {{ .Platform }}"}},
		PostReleaseHooks: []pipeline.HookStep{{Run: "echo released to {{ .Platform }}"}},
	}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	names := stepNames(captured)
	assert.NotContains(t, names, "Run pre_release hooks", "pre/post_release fold into the publish step, not separate steps")
	assert.NotContains(t, names, "Run post_release hooks")

	var publishStep capturedStep
	for _, s := range captured {
		if s.name == "Publish to github" {
			publishStep = s
		}
	}
	assert.Contains(t, publishStep.subs, "[dry-run] would run: echo about to publish to github")
	assert.Contains(t, publishStep.subs, "[dry-run] would run: echo released to github")
}

func TestRun_DryRun_NoHooksSuppressesAllHookOutput(t *testing.T) {
	mr := exectest.NewMockRunner()
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{
		NoHooks:          true,
		Platforms:        []port.Platform{platform},
		PostBumpHooks:    []pipeline.HookStep{{Run: "echo post-bump"}},
		PreTagHooks:      []pipeline.HookStep{{Run: "echo pre-tag"}},
		PostTagHooks:     []pipeline.HookStep{{Run: "echo post-tag"}},
		PreReleaseHooks:  []pipeline.HookStep{{Run: "echo pre-release"}},
		PostReleaseHooks: []pipeline.HookStep{{Run: "echo post-release"}},
	}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	names := stepNames(captured)
	assert.NotContains(t, names, "Run post_bump hooks")
	assert.NotContains(t, names, "Run pre_tag hooks")
	assert.NotContains(t, names, "Run post_tag hooks")
	for _, s := range captured {
		for _, sub := range s.subs {
			assert.NotContains(t, sub, "[dry-run] would run", "NoHooks must suppress dry-run hook lines too")
		}
	}
}

func TestRun_DryRun_HookTemplateError_Aborts(t *testing.T) {
	mr := exectest.NewMockRunner()
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{
		Platforms:   []port.Platform{platform},
		PreTagHooks: []pipeline.HookStep{{Run: "echo {{ .Bad"}},
	}

	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true)
	require.Error(t, p.Run())
}

// TestRun_DryRun_PostBumpHookWithStage_ShowsWouldStageLine proves a Stage-bearing HookStep's
// "would stage:" line actually surfaces through the full reporter dry-run path
// (dryRunHookStep → dryRunHookLinesOrNil → dryRunHookLines), landing in the step's subs
// alongside the "would run:" line in its result — not just at dryRunHookLines' own unit level.
func TestRun_DryRun_PostBumpHookWithStage_ShowsWouldStageLine(t *testing.T) {
	mr := exectest.NewMockRunner()
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{
		Platforms: []port.Platform{platform},
		PostBumpHooks: []pipeline.HookStep{
			{Run: "echo {{ .Version }}", Stage: []string{"dist/{{ .Version }}.tgz", "CHANGELOG.md"}},
		},
	}

	var captured []capturedStep
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls, "dry-run must not execute anything for real")

	byName := map[string]capturedStep{}
	for _, s := range captured {
		byName[s.name] = s
	}
	step := byName["Run post_bump hooks"]
	assert.Equal(t, "[dry-run] would run: echo 1.2.3", step.result)
	assert.Contains(t, step.subs, "[dry-run] would stage: dist/1.2.3.tgz")
	assert.Contains(t, step.subs, "[dry-run] would stage: CHANGELOG.md")
}

func TestRun_DryRun_Plain_ShowsHookLines(t *testing.T) {
	mr := exectest.NewMockRunner()
	platform := &testutil.MockPlatform{PlatformName: "github"}
	cfg := &pipeline.Config{
		Platforms:     []port.Platform{platform},
		PostBumpHooks: []pipeline.HookStep{{Run: "echo {{ .Version }}"}},
	}

	out := &bytes.Buffer{}
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, true)
	require.NoError(t, p.Run())

	assert.Contains(t, out.String(), "[dry-run] would run: echo 1.2.3")
}

func TestChangelogRun_DryRun_AllFourHookSteps(t *testing.T) {
	mr := exectest.NewMockRunner()
	changelog := &testutil.MockGenerator{}
	cfg := &pipeline.ChangelogConfig{
		Changelog:         changelog,
		ChangelogFile:     "CHANGELOG.md",
		Tag:               true,
		PostBumpHooks:     []pipeline.HookStep{{Run: "echo {{ .Version }}"}},
		PreChangelogHooks: []pipeline.HookStep{{Run: "make lint"}},
		PreTagHooks:       []pipeline.HookStep{{Run: "go build ./..."}},
		PostTagHooks:      []pipeline.HookStep{{Run: "npm publish"}},
	}

	var captured []capturedStep
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls)
	names := stepNames(captured)
	assert.Contains(t, names, "Run post_bump hooks")
	assert.Contains(t, names, "Run pre_changelog hooks")
	assert.Contains(t, names, "Run pre_tag hooks")
	assert.Contains(t, names, "Run post_tag hooks")
}

// TestChangelogRun_DryRun_PostBumpRendersEvenWhenDisabledAndNoTag proves post_bump's dry-run
// rendering fires even in the one case where dryRunOutput itself is never reached (Run()
// returns immediately after the DisableChangelog+!Tag branch, before the dry-run check) — the
// dry-run counterpart to TestChangelogRun_PostBumpHook_FiresEvenWhenDisabledAndNoTag.
func TestChangelogRun_DryRun_PostBumpRendersEvenWhenDisabledAndNoTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &pipeline.ChangelogConfig{
		DisableChangelog: true,
		Tag:              false,
		PostBumpHooks:    []pipeline.HookStep{{Run: "echo {{ .Version }}"}},
	}

	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, true)
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls, "dry-run must not execute anything for real")
	assert.Contains(t, out.String(), "[dry-run] would run: echo 1.2.3")
}

// TestChangelogRun_DryRun_PostBumpRendersEvenWhenDisabledAndNoTag_Reporter is the reporter-path
// counterpart: the step must appear even though dryRunOutput is never reached.
func TestChangelogRun_DryRun_PostBumpRendersEvenWhenDisabledAndNoTag_Reporter(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &pipeline.ChangelogConfig{
		DisableChangelog: true,
		Tag:              false,
		PostBumpHooks:    []pipeline.HookStep{{Run: "echo {{ .Version }}"}},
	}

	var captured []capturedStep
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls)
	names := stepNames(captured)
	assert.Contains(t, names, "Run post_bump hooks")
}

// TestChangelogRun_DryRun_PreChangelogHookWithStage_ShowsWouldStageLine is the plain-writer
// counterpart to TestRun_DryRun_PostBumpHookWithStage_ShowsWouldStageLine: proves the
// "would stage:" line surfaces through printDryRunHookLinesPlain → dryRunHookLinesOrNil →
// dryRunHookLines too, right after its "would run:" line, for the ChangelogPipeline.
func TestChangelogRun_DryRun_PreChangelogHookWithStage_ShowsWouldStageLine(t *testing.T) {
	mr := exectest.NewMockRunner()
	changelog := &testutil.MockGenerator{}
	cfg := &pipeline.ChangelogConfig{
		Changelog: changelog,
		PreChangelogHooks: []pipeline.HookStep{
			{Run: "make lint", Stage: []string{"dist/*.tgz"}},
		},
	}

	out := &bytes.Buffer{}
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, out, true)
	require.NoError(t, p.Run())

	assert.Empty(t, mr.Calls, "dry-run must not execute anything for real")
	lines := out.String()
	runIdx := strings.Index(lines, "[dry-run] would run: make lint")
	stageIdx := strings.Index(lines, "[dry-run] would stage: dist/*.tgz")
	require.NotEqual(t, -1, runIdx, "would run: line must be present")
	require.NotEqual(t, -1, stageIdx, "would stage: line must be present")
	assert.Less(t, runIdx, stageIdx, "would stage: must follow its would run: line")
}

func TestChangelogRun_DryRun_NoHooksSuppressesAllHookSteps(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &pipeline.ChangelogConfig{
		NoHooks:       true,
		Tag:           true,
		PostBumpHooks: []pipeline.HookStep{{Run: "echo post-bump"}},
		PreTagHooks:   []pipeline.HookStep{{Run: "echo pre-tag"}},
		PostTagHooks:  []pipeline.HookStep{{Run: "echo post-tag"}},
	}

	var captured []capturedStep
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &bytes.Buffer{}, true).
		WithReporter(capturingStepFn(&captured))
	require.NoError(t, p.Run())

	names := stepNames(captured)
	assert.NotContains(t, names, "Run post_bump hooks")
	assert.NotContains(t, names, "Run pre_tag hooks")
	assert.NotContains(t, names, "Run post_tag hooks")
}
