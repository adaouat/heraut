package pipeline_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
)

// allHooks configures a command at each of the six hook points.
func allHooks(cfg *pipeline.Config) *pipeline.Config {
	cfg.PostBumpHooks = []pipeline.HookStep{{Run: "bump {{ .Tag }}"}}
	cfg.PreChangelogHooks = []pipeline.HookStep{{Run: "pre-changelog"}}
	cfg.PreTagHooks = []pipeline.HookStep{{Run: "pre-tag"}}
	cfg.PostTagHooks = []pipeline.HookStep{{Run: "post-tag"}}
	cfg.PreReleaseHooks = []pipeline.HookStep{{Run: "pre-release {{ .Platform }}"}}
	cfg.PostReleaseHooks = []pipeline.HookStep{{Run: "post-release {{ .Platform }}"}}
	return cfg
}

func twoPlatforms() []port.Platform {
	return []port.Platform{
		&testutil.MockPlatform{PlatformName: "github", HasAssetsVal: true},
		&testutil.MockPlatform{PlatformName: "gitlab"},
	}
}

// planStepNames maps the --json plan to the names the reporter sees. pre_release and post_release
// hooks run inside their platform's publish step, so they have no step of their own.
func planStepNames(plan pipeline.Plan) []string {
	names := []string{"Resolve version"}
	for _, s := range plan.Steps {
		switch s.Action {
		case "hook":
			if s.Point == "pre_release" || s.Point == "post_release" {
				continue
			}
			names = append(names, fmt.Sprintf("Run %s hooks", s.Point))
		case "changelog":
			names = append(names, "Generate changelog")
		case "commit_changelog":
			names = append(names, "Commit changelog")
		case "tag":
			names = append(names, "Create tag "+s.Target)
		case "push_tag":
			names = append(names, "Push tag")
		case "notes":
			names = append(names, "Generate release notes")
		case "publish":
			names = append(names, "Publish to "+s.Target)
		}
	}
	return names
}

// The --json plan, the dry-run and a real run each know the release's steps; they must agree on
// which steps there are and in what order.
func TestRelease_PlanDryRunAndRealRunListTheSameSteps(t *testing.T) {
	tests := []struct {
		name string
		cfg  func() *pipeline.Config
	}{
		{"minimal", func() *pipeline.Config {
			return &pipeline.Config{Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}}}
		}},
		{"everything, two platforms", func() *pipeline.Config {
			return allHooks(&pipeline.Config{
				Changelog: &testutil.MockGenerator{}, ChangelogFile: "CHANGELOG.md",
				Notes: &testutil.MockGenerator{}, Platforms: twoPlatforms(),
			})
		}},
		{"single platform generates notes once", func() *pipeline.Config {
			return &pipeline.Config{
				Notes:     &testutil.MockGenerator{},
				Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
			}
		}},
		{"changelog disabled for the env", func() *pipeline.Config {
			return allHooks(&pipeline.Config{
				Changelog: &testutil.MockGenerator{}, ChangelogFile: "CHANGELOG.md", DisableChangelog: true,
				Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
			})
		}},
		{"notes disabled for the env", func() *pipeline.Config {
			return &pipeline.Config{
				Notes: &testutil.MockGenerator{}, DisableNotes: true,
				Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
			}
		}},
		{"--no-hooks", func() *pipeline.Config {
			cfg := allHooks(&pipeline.Config{
				Changelog: &testutil.MockGenerator{}, ChangelogFile: "CHANGELOG.md", Platforms: twoPlatforms(),
			})
			cfg.NoHooks = true
			return cfg
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := resolvedResult("v1.2.3")

			plan, err := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: res}, tc.cfg(), &bytes.Buffer{}, true).Plan()
			require.NoError(t, err)
			want := planStepNames(plan)

			var dry []capturedStep
			require.NoError(t, pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: res}, tc.cfg(), &bytes.Buffer{}, true).
				WithReporter(capturingStepFn(&dry)).Run())
			assert.Equal(t, want, stepNames(dry), "dry-run steps")

			mr := exectest.NewMockRunner()
			for range 40 {
				mr.QueueResponse("CHANGELOG.md\n", "", nil) // every git/sh call; the staged-files probe needs output
			}
			var real []capturedStep
			require.NoError(t, pipeline.New(mr, &fakeResolver{result: res}, tc.cfg(), &bytes.Buffer{}, false).
				WithReporter(capturingStepFn(&real)).Run())
			assert.Equal(t, want, stepNames(real), "executed steps")
		})
	}
}
