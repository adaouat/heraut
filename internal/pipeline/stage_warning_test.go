package pipeline_test

import (
	"bytes"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
)

// ADR-0061 addendum: a run that collects hook `stage` patterns but makes no changelog commit says
// which files it left uncommitted; it never fails.
const uncommittedStageWarning = "not committed"

func TestRun_StageWithoutChangelogCommit_WarnsWhichFilesWereLeft(t *testing.T) {
	stageHook := []pipeline.HookStep{{Run: "echo bumping", Stage: []string{"VERSION", "dist/{{ .Version }}.json"}}}
	tests := []struct {
		name string
		cfg  func() *pipeline.Config
	}{
		{"no changelog configured", func() *pipeline.Config { return &pipeline.Config{PostBumpHooks: stageHook} }},
		{"changelog disabled for this run", func() *pipeline.Config {
			return &pipeline.Config{Changelog: &testutil.MockGenerator{}, DisableChangelog: true, PostBumpHooks: stageHook}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse("", "", nil) // sh -c (post_bump)
			mr.QueueResponse("", "", nil) // git tag
			mr.QueueResponse("", "", nil) // git push <tag>

			cfg := tc.cfg()
			cfg.Platforms = []port.Platform{&testutil.MockPlatform{PlatformName: "github"}}
			var out bytes.Buffer
			p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, false)
			require.NoError(t, p.Run())

			assert.Contains(t, out.String(), uncommittedStageWarning)
			assert.Contains(t, out.String(), "VERSION")
			assert.Contains(t, out.String(), "dist/1.2.3.json", "the patterns are shown rendered")
		})
	}
}

func TestRun_NoWarningWhenTheChangelogCommitCarriesTheStagedFiles(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // sh -c (post_bump)
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push
	mr.QueueResponse("", "", nil)               // git tag
	mr.QueueResponse("", "", nil)               // git push <tag>

	cfg := &pipeline.Config{
		Changelog:     &testutil.MockGenerator{},
		PostBumpHooks: []pipeline.HookStep{{Run: "echo bumping", Stage: []string{"extra.txt"}}},
		Platforms:     []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}
	var out bytes.Buffer
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, false)
	require.NoError(t, p.Run())

	assert.NotContains(t, out.String(), uncommittedStageWarning)
}

func TestRun_NoWarningWithoutStagePatterns(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // sh -c (post_bump)
	mr.QueueResponse("", "", nil) // git tag
	mr.QueueResponse("", "", nil) // git push <tag>

	cfg := &pipeline.Config{
		PostBumpHooks: []pipeline.HookStep{{Run: "echo bumping"}},
		Platforms:     []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}
	var out bytes.Buffer
	p := pipeline.New(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, false)
	require.NoError(t, p.Run())

	assert.NotContains(t, out.String(), uncommittedStageWarning)
}

func TestChangelogRun_StageWithoutCommit_WarnsWhichFilesWereLeft(t *testing.T) {
	tests := []struct {
		name string
		cfg  *pipeline.ChangelogConfig
		runs int // sh calls
	}{
		{"changelog written, no --commit or --tag", &pipeline.ChangelogConfig{
			Changelog:     &testutil.MockGenerator{},
			PostBumpHooks: []pipeline.HookStep{{Run: "echo bumping", Stage: []string{"VERSION"}}},
		}, 1},
		{"changelog disabled", &pipeline.ChangelogConfig{
			DisableChangelog: true,
			PostBumpHooks:    []pipeline.HookStep{{Run: "echo bumping", Stage: []string{"VERSION"}}},
		}, 1},
		{"pre_changelog stage, no commit", &pipeline.ChangelogConfig{
			Changelog:         &testutil.MockGenerator{},
			PreChangelogHooks: []pipeline.HookStep{{Run: "make lint", Stage: []string{"lint.txt"}}},
		}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			for range tc.runs {
				mr.QueueResponse("", "", nil)
			}
			var out bytes.Buffer
			p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, tc.cfg, &out, false)
			require.NoError(t, p.Run())

			assert.Contains(t, out.String(), uncommittedStageWarning)
			if len(tc.cfg.PostBumpHooks) > 0 {
				assert.Contains(t, out.String(), "VERSION")
			} else {
				assert.Contains(t, out.String(), "lint.txt")
			}
		})
	}
}

func TestChangelogRun_NoWarningWhenTheCommitHappens(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // sh -c (post_bump)
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push

	cfg := &pipeline.ChangelogConfig{
		Changelog:     &testutil.MockGenerator{},
		Commit:        true,
		PostBumpHooks: []pipeline.HookStep{{Run: "echo bumping", Stage: []string{"extra.txt"}}},
	}
	var out bytes.Buffer
	p := pipeline.NewChangelog(mr, &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, false)
	require.NoError(t, p.Run())

	assert.NotContains(t, out.String(), uncommittedStageWarning)
}
