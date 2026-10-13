package semver_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixedResolver(t *testing.T, cfg *config.Config, core, label string, all, merged string) (*semver.Resolver, *exectest.MockRunner) {
	t.Helper()
	mr := exectest.NewMockRunner()
	mr.QueueResponse(all, "", nil)
	mr.QueueResponse(merged, "", nil)
	r := semver.New(mr, cfg)
	r.SetFixedVersion(core)
	r.SetPreRelease(label)
	return r, mr
}

func semverCfg() *config.Config {
	return &config.Config{Versioning: config.Versioning{Strategy: "semver"}}
}

func TestResolveFixed(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		all      string
		merged   string
		log      string // stdout of `git log <previous>..HEAD`, queued third when set
		wantNone bool   // expect the no-commits error
		wantTag  string
		wantVer  string
		wantCur  string
		wantErr  error
		wantSub  string
		wantWarn string
	}{
		{name: "first release", all: "v7.7.0\n", merged: "v7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.7.0"},
		{name: "no tags at all", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: ""},
		{name: "rc never bumps anything but the counter", label: "rc", all: "v7.7.0\n", merged: "v7.7.0\n", wantTag: "v7.8.0-rc.1", wantVer: "7.8.0-rc.1", wantCur: "v7.7.0"},
		{name: "second rc", label: "rc", all: "v7.8.0-rc.1\nv7.7.0\n", merged: "v7.8.0-rc.1\nv7.7.0\n", log: "fix: y\x00", wantTag: "v7.8.0-rc.2", wantVer: "7.8.0-rc.2", wantCur: "v7.8.0-rc.1"},
		{name: "final after rcs", all: "v7.8.0-rc.2\nv7.7.0\n", merged: "v7.8.0-rc.2\nv7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.7.0"},
		{name: "a repeat rc with no new commit is refused", label: "rc", all: "v7.8.0-rc.1\nv7.7.0\n", merged: "v7.8.0-rc.1\nv7.7.0\n", log: "", wantNone: true},
		{name: "a higher label promotes without new commits", label: "rc", all: "v7.8.0-beta.1\nv7.7.0\n", merged: "v7.8.0-beta.1\nv7.7.0\n", wantTag: "v7.8.0-rc.1", wantVer: "7.8.0-rc.1", wantCur: "v7.8.0-beta.1"},
		{name: "already released", all: "v7.8.0\nv7.7.0\n", merged: "v7.8.0\nv7.7.0\n", wantErr: semver.ErrTagExists, wantSub: "--pre-release"},
		{name: "build-metadata tag is the release", all: "v7.8.0+3\nv7.7.0\n", merged: "v7.8.0+3\nv7.7.0\n", wantErr: semver.ErrTagExists, wantSub: "--set-version 7.8.0 --set-build-id"},
		{name: "released elsewhere still counts", all: "v7.8.0\nv7.7.0\n", merged: "v7.7.0\n", wantErr: semver.ErrTagExists},
		{name: "rc after the final", label: "rc", all: "v7.8.0\nv7.8.0-rc.1\n", merged: "v7.8.0\nv7.8.0-rc.1\n", wantErr: semver.ErrPreReleaseRegression, wantSub: "already released"},
		{name: "lower label than an existing one", label: "beta", all: "v7.8.0-rc.1\n", merged: "v7.8.0-rc.1\n", wantErr: semver.ErrPreReleaseRegression},
		{name: "higher tag outside the branch history is silent", all: "v7.9.0\nv7.7.0\n", merged: "v7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.7.0"},
		{name: "higher release in the branch history warns", all: "v7.9.0\nv7.7.0\n", merged: "v7.9.0\nv7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.7.0",
			wantWarn: "branch version 7.8.0 is lower than v7.9.0, already in this branch's history"},
		{name: "a merged pre-release does not warn", all: "v7.9.0-rc.1\nv7.7.0\n", merged: "v7.9.0-rc.1\nv7.7.0\n", wantTag: "v7.8.0", wantVer: "7.8.0", wantCur: "v7.7.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, mr := fixedResolver(t, semverCfg(), "7.8.0", tc.label, tc.all, tc.merged)
			if tc.log != "" || tc.wantNone {
				mr.QueueResponse(tc.log, "", nil)
			}

			res, err := r.Resolve()

			if tc.wantNone {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "no commits since v7.8.0-rc.1")
				return
			}

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				if tc.wantSub != "" {
					assert.Contains(t, err.Error(), tc.wantSub)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, res.Tag)
			assert.Equal(t, tc.wantVer, res.Version)
			assert.Equal(t, tc.wantCur, res.CurrentTag)
			assert.Equal(t, versioning.BumpNone, res.Bump)
			if tc.wantWarn == "" {
				assert.Empty(t, r.Warnings())
			} else {
				assert.Equal(t, []string{tc.wantWarn}, r.Warnings())
			}
			wantCalls := 2
			if tc.log != "" {
				wantCalls = 3 // only a repeated label reads the commits since its previous tag
			}
			require.Len(t, mr.Calls, wantCalls, "commits are read only to re-cut the same label")
			assert.Equal(t, tagsCall("v"), mr.Calls[0].Args)
			assert.Equal(t, mergedCall("v"), mr.Calls[1].Args)
		})
	}
}

func TestResolveFixed_CustomPrefix(t *testing.T) {
	prefix := "rel-"
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: &prefix}}
	r, _ := fixedResolver(t, cfg, "7.8.0", "", "rel-7.7.0\n", "rel-7.7.0\n")

	res, err := r.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "rel-7.8.0", res.Tag)
	assert.Equal(t, "7.8.0", res.Version)
}

func TestResolveFixed_SetVersionStillWins(t *testing.T) {
	mr := exectest.NewMockRunner()
	r := semver.New(mr, semverCfg())
	r.SetFixedVersion("7.8.0")
	r.SetVersionOverride("9.9.9")

	res, err := r.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "v9.9.9", res.Tag)
	assert.Empty(t, mr.Calls)
}

func TestResolveFixed_EmptyRestoresNormalResolution(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // no tags: initial version
	r := semver.New(mr, semverCfg())
	r.SetFixedVersion("7.8.0")
	r.SetFixedVersion("")

	res, err := r.Resolve()

	require.NoError(t, err)
	assert.Equal(t, "v0.1.0", res.Tag)
}
