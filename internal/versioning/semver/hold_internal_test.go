package semver

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBumpAfterHold_AppendsAcrossCalls(t *testing.T) {
	cfg := &config.Config{
		Versioning: config.Versioning{Bump: &config.BumpConfig{StayAtV0: true}},
	}
	r := New(nil, cfg)

	r.bumpAfterHold("0.68.0", []string{"feat!: first"})
	r.bumpAfterHold("0.68.0", []string{"feat!: second"})

	require.Len(t, r.Warnings(), 2, "a second call within one resolution must not drop the first warning")
	assert.Contains(t, r.Warnings()[0], "feat!: first")
	assert.Contains(t, r.Warnings()[1], "feat!: second")
}

// RULE-052: rendering.excludes hide commits from the changelog but never from the bump, so a
// release driven only by hidden commits has an empty changelog. The resolver says so.
func TestBumpAfterHold_WarnsWhenEveryReleasableCommitIsHiddenFromTheChangelog(t *testing.T) {
	withChangelog := func() *config.Config {
		return &config.Config{Changelog: &config.ContentDriver{Output: "CHANGELOG.md"}}
	}
	tests := []struct {
		name     string
		cfg      func() *config.Config
		commits  []string
		wantWarn bool
	}{
		{"only dependency noise", withChangelog, []string{"chore(deps): bump x", "chore(deps-dev): bump y"}, true},
		{"noise plus a release commit", withChangelog, []string{"chore(deps): bump x", "chore(release): 1.2.3"}, true},
		{"one real commit among the noise", withChangelog, []string{"chore(deps): bump x", "fix: handle nil"}, false},
		{"a feat", withChangelog, []string{"feat: add login"}, false},
		{"hidden by a user rendering.excludes entry", func() *config.Config {
			c := withChangelog()
			c.Rendering = &config.Rendering{Excludes: []config.Exclude{{Type: "docs"}}}
			return c
		}, []string{"docs: tweak"}, true},
		{"hidden by the changelog driver's own excludes", func() *config.Config {
			c := withChangelog()
			c.Changelog.Rendering = &config.Rendering{Excludes: []config.Exclude{{Regex: `^fix: typo`}}}
			return c
		}, []string{"fix: typo in readme"}, true},
		{"no changelog configured: nothing to be empty", func() *config.Config { return &config.Config{} }, []string{"chore(deps): bump x"}, false},
		{"a non-releasable hidden commit beside a visible one", withChangelog, []string{"chore(deps): bump x", "feat: add login"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := New(nil, tc.cfg())
			bump := r.bumpAfterHold("1.2.0", tc.commits)

			assert.NotEqual(t, 0, int(bump), "the bump itself is unchanged")
			if !tc.wantWarn {
				assert.Empty(t, r.Warnings())
				return
			}
			require.Len(t, r.Warnings(), 1)
			assert.Contains(t, r.Warnings()[0], "empty changelog")
			assert.Contains(t, r.Warnings()[0], "rendering.excludes")
			assert.Len(t, r.WouldBeVersions(), 1, "stays parallel to Warnings")
			assert.Empty(t, r.WouldBeVersions()[0])
		})
	}
}

func TestBumpAfterHold_NoEmptyChangelogWarningForAPreRelease(t *testing.T) {
	r := New(nil, &config.Config{Changelog: &config.ContentDriver{Output: "CHANGELOG.md"}})
	r.preReleaseLabel = "rc"
	r.bumpAfterHold("1.2.0", []string{"chore(deps): bump x"})
	assert.Empty(t, r.Warnings(), "a pre-release never writes a changelog section (ADR-0064)")
}
