package pipeline_test

import (
	"bytes"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/testutil"
)

func changelogHooks(cfg *pipeline.ChangelogConfig) *pipeline.ChangelogConfig {
	cfg.PostBumpHooks = []pipeline.HookStep{{Run: "bump {{ .Tag }}"}}
	cfg.PreChangelogHooks = []pipeline.HookStep{{Run: "pre-changelog"}}
	cfg.PreTagHooks = []pipeline.HookStep{{Run: "pre-tag"}}
	cfg.PostTagHooks = []pipeline.HookStep{{Run: "post-tag"}}
	return cfg
}

// A dry run of `heraut changelog` reports the same steps, in the same order, as a real run.
func TestChangelogRun_DryRunAndRealRunListTheSameSteps(t *testing.T) {
	gen := func() *testutil.MockGenerator { return &testutil.MockGenerator{} }
	tests := []struct {
		name string
		cfg  func() *pipeline.ChangelogConfig
	}{
		{"generate only", func() *pipeline.ChangelogConfig {
			return &pipeline.ChangelogConfig{Changelog: gen(), ChangelogFile: "CHANGELOG.md"}
		}},
		{"commit, with every hook", func() *pipeline.ChangelogConfig {
			return changelogHooks(&pipeline.ChangelogConfig{Changelog: gen(), ChangelogFile: "CHANGELOG.md", Commit: true})
		}},
		{"commit and tag, with every hook", func() *pipeline.ChangelogConfig {
			return changelogHooks(&pipeline.ChangelogConfig{Changelog: gen(), ChangelogFile: "CHANGELOG.md", Tag: true})
		}},
		{"tag without pushing", func() *pipeline.ChangelogConfig {
			return changelogHooks(&pipeline.ChangelogConfig{Changelog: gen(), ChangelogFile: "CHANGELOG.md", Tag: true, NoPush: true})
		}},
		{"changelog disabled, tag only", func() *pipeline.ChangelogConfig {
			return changelogHooks(&pipeline.ChangelogConfig{Changelog: gen(), ChangelogFile: "CHANGELOG.md", DisableChangelog: true, Tag: true})
		}},
		{"changelog disabled, nothing else", func() *pipeline.ChangelogConfig {
			return changelogHooks(&pipeline.ChangelogConfig{Changelog: gen(), ChangelogFile: "CHANGELOG.md", DisableChangelog: true})
		}},
		{"--no-hooks", func() *pipeline.ChangelogConfig {
			cfg := changelogHooks(&pipeline.ChangelogConfig{Changelog: gen(), ChangelogFile: "CHANGELOG.md", Tag: true})
			cfg.NoHooks = true
			return cfg
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := resolvedResult("v1.2.3")

			var dry []capturedStep
			require.NoError(t, pipeline.NewChangelog(exectest.NewMockRunner(), &fakeResolver{result: res}, tc.cfg(), &bytes.Buffer{}, true).
				WithReporter(capturingStepFn(&dry)).Run())

			mr := exectest.NewMockRunner()
			for range 40 {
				mr.QueueResponse("CHANGELOG.md\n", "", nil)
			}
			var real []capturedStep
			require.NoError(t, pipeline.NewChangelog(mr, &fakeResolver{result: res}, tc.cfg(), &bytes.Buffer{}, false).
				WithReporter(capturingStepFn(&real)).Run())

			assert.Equal(t, stepNames(real), stepNames(dry))
		})
	}
}
