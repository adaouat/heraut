package pipeline_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning"
)

func planFor(t *testing.T, cfg *pipeline.Config, res versioning.Result) (pipeline.Plan, *exectest.MockRunner, error) {
	t.Helper()
	mr := exectest.NewMockRunner()
	p := pipeline.New(mr, &fakeResolver{result: res}, cfg, &bytes.Buffer{}, true)
	plan, err := p.Plan()
	return plan, mr, err
}

func TestPlan_FullRelease_ListsEveryActionInOrder(t *testing.T) {
	cfg := &pipeline.Config{
		Changelog:     &testutil.MockGenerator{},
		ChangelogFile: "CHANGELOG.md",
		Notes:         &testutil.MockGenerator{},
		Platforms: []port.Platform{
			&testutil.MockPlatform{PlatformName: "github", HasAssetsVal: true},
			&testutil.MockPlatform{PlatformName: "gitlab"},
		},
		PostBumpHooks:    []pipeline.HookStep{{Run: "echo bump {{ .Version }}", Stage: []string{"VERSION"}}},
		PreTagHooks:      []pipeline.HookStep{{Run: "make check"}},
		PostReleaseHooks: []pipeline.HookStep{{Run: "notify {{ .Platform }} {{ .Tag }}"}},
		SemVerStrategy:   true,
	}
	res := versioning.Result{Version: "1.4.0-rc.1", Tag: "v1.4.0-rc.1", CurrentTag: "v1.3.0", Warnings: []string{"held"}}

	plan, mr, err := planFor(t, cfg, res)

	require.NoError(t, err)
	assert.Empty(t, mr.Calls, "planning runs nothing")
	assert.Equal(t, "v1.4.0-rc.1", plan.Tag)
	assert.Equal(t, "1.4.0-rc.1", plan.Version)
	assert.Equal(t, "v1.3.0", plan.CurrentTag)
	assert.True(t, plan.PreRelease)
	assert.Equal(t, []string{"held"}, plan.Warnings)
	assert.Equal(t, []pipeline.PlanStep{
		{Action: "hook", Point: "post_bump", Commands: []string{"echo bump 1.4.0-rc.1"}, Stage: []string{"VERSION"}},
		{Action: "changelog", Target: "CHANGELOG.md"},
		{Action: "commit_changelog", Target: "CHANGELOG.md"},
		{Action: "hook", Point: "pre_tag", Commands: []string{"make check"}},
		{Action: "tag", Target: "v1.4.0-rc.1"},
		{Action: "push_tag", Target: "v1.4.0-rc.1"},
		{Action: "publish", Target: "github", Notes: true, Assets: true, PreRelease: true},
		{Action: "hook", Point: "post_release", Target: "github", Commands: []string{"notify github v1.4.0-rc.1"}},
		{Action: "publish", Target: "gitlab", Notes: true, PreRelease: true},
		{Action: "hook", Point: "post_release", Target: "gitlab", Commands: []string{"notify gitlab v1.4.0-rc.1"}},
	}, plan.Steps)
}

func TestPlan_SinglePlatformGeneratesNotesOnce(t *testing.T) {
	cfg := &pipeline.Config{
		Notes:     &testutil.MockGenerator{},
		Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}

	plan, _, err := planFor(t, cfg, versioning.Result{Version: "1.0.0", Tag: "v1.0.0"})

	require.NoError(t, err)
	assert.Equal(t, []pipeline.PlanStep{
		{Action: "tag", Target: "v1.0.0"},
		{Action: "push_tag", Target: "v1.0.0"},
		{Action: "notes"},
		{Action: "publish", Target: "github", Notes: true},
	}, plan.Steps)
	assert.False(t, plan.PreRelease)
}

func TestPlan_DisabledChangelogNoHooksAndCalVer(t *testing.T) {
	cfg := &pipeline.Config{
		Changelog:        &testutil.MockGenerator{},
		DisableChangelog: true,
		NoHooks:          true,
		PreTagHooks:      []pipeline.HookStep{{Run: "make check"}},
		Platforms:        []port.Platform{&testutil.MockPlatform{PlatformName: "github"}},
	}

	plan, _, err := planFor(t, cfg, versioning.Result{Version: "2026.10.2-0", Tag: "2026.10.2-0"})

	require.NoError(t, err)
	assert.False(t, plan.PreRelease, "a pre-release shape means nothing outside a SemVer strategy")
	assert.Equal(t, []pipeline.PlanStep{
		{Action: "tag", Target: "2026.10.2-0"},
		{Action: "push_tag", Target: "2026.10.2-0"},
		{Action: "publish", Target: "github"},
	}, plan.Steps)
}

func TestPlan_ResolveAndHookTemplateErrorsAreReturned(t *testing.T) {
	_, _, err := planFor(t, &pipeline.Config{}, versioning.Result{})
	require.NoError(t, err)

	mr := exectest.NewMockRunner()
	p := pipeline.New(mr, &fakeResolver{err: errors.New("no tags")}, &pipeline.Config{}, &bytes.Buffer{}, true)
	_, err = p.Plan()
	require.ErrorContains(t, err, "no tags")

	_, _, err = planFor(t, &pipeline.Config{PreTagHooks: []pipeline.HookStep{{Run: "{{ .Typo }}"}}}, versioning.Result{Tag: "v1"})
	require.ErrorContains(t, err, "Typo")
}
