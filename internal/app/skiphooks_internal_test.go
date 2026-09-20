package app

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookPoints(t *testing.T) {
	assert.Equal(t,
		[]string{"post_bump", "pre_changelog", "pre_tag", "post_tag", "pre_release", "post_release"},
		ReleaseHookPoints())
	assert.Equal(t,
		[]string{"post_bump", "pre_changelog", "pre_tag", "post_tag"},
		ChangelogHookPoints(), "changelog never publishes, so pre_release/post_release never apply")
}

func TestHookPoints_ReturnCopies(t *testing.T) {
	ReleaseHookPoints()[0] = "mutated"
	assert.Equal(t, "post_bump", ReleaseHookPoints()[0], "callers must not be able to mutate the package's list")
}

func TestValidateSkipHooks(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		allowed []string
		want    []string
		wantErr string
	}{
		{"nil values", nil, ReleaseHookPoints(), nil, ""},
		{"empty values", []string{}, ReleaseHookPoints(), nil, ""},
		{"single valid point", []string{"post_release"}, ReleaseHookPoints(), []string{"post_release"}, ""},
		{"several valid points keep order", []string{"pre_tag", "post_bump"}, ReleaseHookPoints(), []string{"pre_tag", "post_bump"}, ""},
		{"trims whitespace, drops blanks, dedups", []string{" pre_tag ", "", "pre_tag", "post_bump"}, ReleaseHookPoints(), []string{"pre_tag", "post_bump"}, ""},
		{"unknown point", []string{"pre_publish"}, ReleaseHookPoints(), nil, `unknown hook point "pre_publish"`},
		{"unknown point lists the valid ones", []string{"typo"}, ReleaseHookPoints(), nil, "post_bump, pre_changelog, pre_tag, post_tag, pre_release, post_release"},
		{"one bad value among good ones fails the whole list", []string{"pre_tag", "typo"}, ReleaseHookPoints(), nil, `unknown hook point "typo"`},
		{"case matters", []string{"POST_BUMP"}, ReleaseHookPoints(), nil, `unknown hook point "POST_BUMP"`},
		{"release-only point rejected for changelog", []string{"post_release"}, ChangelogHookPoints(), nil, `hook point "post_release" does not apply to this command`},
		{"pre_release rejected for changelog", []string{"pre_release"}, ChangelogHookPoints(), nil, `hook point "pre_release" does not apply to this command`},
		{"changelog rejection lists only its own points", []string{"post_release"}, ChangelogHookPoints(), nil, "post_bump, pre_changelog, pre_tag, post_tag"},
		{"shared point accepted for changelog", []string{"pre_tag"}, ChangelogHookPoints(), []string{"pre_tag"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateSkipHooks(tc.values, tc.allowed)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestValidateSkipHooks_ChangelogRejectionOmitsReleaseOnlyPointsFromHint(t *testing.T) {
	_, err := ValidateSkipHooks([]string{"post_release"}, ChangelogHookPoints())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "pre_release", "the hint must list only what this command accepts")
}

func allReleaseHooks() *pipeline.Config {
	return &pipeline.Config{
		PostBumpHooks:     []pipeline.HookStep{{Run: "echo post_bump", Stage: []string{"a.txt"}}},
		PreChangelogHooks: []pipeline.HookStep{{Run: "echo pre_changelog"}},
		PreTagHooks:       []pipeline.HookStep{{Run: "echo pre_tag"}},
		PostTagHooks:      []pipeline.HookStep{{Run: "echo post_tag"}},
		PreReleaseHooks:   []pipeline.HookStep{{Run: "echo pre_release"}},
		PostReleaseHooks:  []pipeline.HookStep{{Run: "echo post_release"}},
	}
}

func TestApplySkipHooks_Release(t *testing.T) {
	// hooksByPoint reads a point's steps back out of the config so each row can assert on
	// exactly the point it skipped, and prove every other point is untouched.
	hooksByPoint := func(c *pipeline.Config) map[string][]pipeline.HookStep {
		return map[string][]pipeline.HookStep{
			"post_bump":     c.PostBumpHooks,
			"pre_changelog": c.PreChangelogHooks,
			"pre_tag":       c.PreTagHooks,
			"post_tag":      c.PostTagHooks,
			"pre_release":   c.PreReleaseHooks,
			"post_release":  c.PostReleaseHooks,
		}
	}
	original := hooksByPoint(allReleaseHooks())

	tests := []struct {
		name string
		skip []string
	}{
		{"nothing skipped", nil},
		{"post_bump", []string{"post_bump"}},
		{"pre_changelog", []string{"pre_changelog"}},
		{"pre_tag", []string{"pre_tag"}},
		{"post_tag", []string{"post_tag"}},
		{"pre_release", []string{"pre_release"}},
		{"post_release", []string{"post_release"}},
		{"two points", []string{"pre_tag", "post_release"}},
		{"every point", ReleaseHookPoints()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := allReleaseHooks()
			applySkipHooks(cfg, tc.skip)

			skipped := map[string]bool{}
			for _, p := range tc.skip {
				skipped[p] = true
			}
			for point, steps := range hooksByPoint(cfg) {
				if skipped[point] {
					assert.Empty(t, steps, "%s should be skipped", point)
				} else {
					assert.Equal(t, original[point], steps, "%s must be untouched", point)
				}
			}
		})
	}
}

func TestApplySkipHooks_Changelog(t *testing.T) {
	newCfg := func() *pipeline.ChangelogConfig {
		return &pipeline.ChangelogConfig{
			PostBumpHooks:     []pipeline.HookStep{{Run: "echo post_bump"}},
			PreChangelogHooks: []pipeline.HookStep{{Run: "echo pre_changelog"}},
			PreTagHooks:       []pipeline.HookStep{{Run: "echo pre_tag"}},
			PostTagHooks:      []pipeline.HookStep{{Run: "echo post_tag"}},
		}
	}

	cfg := newCfg()
	applyChangelogSkipHooks(cfg, []string{"pre_changelog", "post_tag"})
	assert.Equal(t, newCfg().PostBumpHooks, cfg.PostBumpHooks)
	assert.Empty(t, cfg.PreChangelogHooks)
	assert.Equal(t, newCfg().PreTagHooks, cfg.PreTagHooks)
	assert.Empty(t, cfg.PostTagHooks)

	cfg = newCfg()
	applyChangelogSkipHooks(cfg, nil)
	assert.Equal(t, newCfg(), cfg, "no skip list is a no-op")
}

// TestApplySkipHooks_ShrinksStepTotal proves a skipped point disappears from the numbered-step
// count exactly like a point that was never configured — the [N/total] counter must not promise
// a step that will never run.
func TestApplySkipHooks_ShrinksStepTotal(t *testing.T) {
	cfg := allReleaseHooks()
	cfg.Changelog = nil
	before := releaseStepTotal(cfg)

	applySkipHooks(cfg, []string{"pre_tag", "post_tag"})
	assert.Equal(t, before-2, releaseStepTotal(cfg))

	cCfg := &pipeline.ChangelogConfig{
		Tag:           true,
		PostBumpHooks: []pipeline.HookStep{{Run: "echo post_bump"}},
		PreTagHooks:   []pipeline.HookStep{{Run: "echo pre_tag"}},
		PostTagHooks:  []pipeline.HookStep{{Run: "echo post_tag"}},
	}
	cBefore := changelogStepTotal(cCfg)
	applyChangelogSkipHooks(cCfg, []string{"pre_tag"})
	assert.Equal(t, cBefore-1, changelogStepTotal(cCfg))
}

func TestBuildChangelogPipelineConfig_SkipHooksEmptiesSkippedPoints(t *testing.T) {
	testutil.ClearCIEnv(t)
	runner := exectest.NewMockRunner()
	readRunner := exectest.NewMockRunner()
	readRunner.QueueResponse("", "", assertNoOriginErr)

	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Hooks: &config.Hooks{
			PostBump: []config.HookStep{{Run: "echo post-bump"}},
			PreTag:   []config.HookStep{{Run: "go build ./..."}},
			PostTag:  []config.HookStep{{Run: "npm publish"}},
		},
	}

	cCfg, err := buildChangelogPipelineConfig(runner, readRunner, cfg, PipelineOpts{SkipHooks: []string{"pre_tag"}})
	require.NoError(t, err)
	assert.Equal(t, []pipeline.HookStep{{Run: "echo post-bump"}}, cCfg.PostBumpHooks)
	assert.Empty(t, cCfg.PreTagHooks)
	assert.Equal(t, []pipeline.HookStep{{Run: "npm publish"}}, cCfg.PostTagHooks)
	assert.False(t, cCfg.NoHooks, "--skip-hook is not --no-hooks")
}
