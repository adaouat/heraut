package semver_test

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

func confirmMajorCfg(stayAtV0 bool, overrides ...config.BumpRule) *config.Config {
	return &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver",
			TagPrefix: strPtr("v"),
			Bump:      &config.BumpConfig{ConfirmMajor: true, StayAtV0: stayAtV0, Overrides: overrides},
		},
	}
}

// ADR-0078: under versioning.bump.confirm_major an automatic major bump is marked for confirmation,
// with the commits that forced it; the resolver still returns the major, the commands decide.
func TestResolve_ConfirmMajor(t *testing.T) {
	plain := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}
	tests := []struct {
		name       string
		cfg        *config.Config
		tag        string
		allowMajor bool
		commits    []string
		wantTag    string
		wantMarked []string
		wantWarned bool
	}{
		{
			name: "a major above 1.0 is marked", cfg: confirmMajorCfg(false), tag: "v1.4.0",
			commits: []string{"feat!: break the api", "fix: small"},
			wantTag: "v2.0.0", wantMarked: []string{"feat!: break the api"},
		},
		{
			name: "a BREAKING CHANGE footer is marked by its subject", cfg: confirmMajorCfg(false), tag: "v1.4.0",
			commits: []string{"feat: new thing\n\nBREAKING CHANGE: the old thing is gone"},
			wantTag: "v2.0.0", wantMarked: []string{"feat: new thing"},
		},
		{
			name: "0.x to 1.0.0 is marked with the setting alone", cfg: confirmMajorCfg(false), tag: "v0.9.0",
			commits: []string{"feat!: x"},
			wantTag: "v1.0.0", wantMarked: []string{"feat!: x"},
		},
		{
			name: "at v0 the stay_at_v0 hold goes first", cfg: confirmMajorCfg(true), tag: "v0.9.0",
			commits: []string{"feat!: x"},
			wantTag: "v0.10.0", wantWarned: true,
		},
		{
			name: "above 1.0 with both settings the major is marked", cfg: confirmMajorCfg(true), tag: "v1.4.0",
			commits: []string{"feat!: x"},
			wantTag: "v2.0.0", wantMarked: []string{"feat!: x"},
		},
		{
			name: "--allow-major confirms it", cfg: confirmMajorCfg(false), tag: "v1.4.0", allowMajor: true,
			commits: []string{"feat!: x"},
			wantTag: "v2.0.0",
		},
		{
			name: "without the setting nothing is marked", cfg: plain, tag: "v1.4.0",
			commits: []string{"feat!: x"},
			wantTag: "v2.0.0",
		},
		{
			name: "a minor is not marked", cfg: confirmMajorCfg(false), tag: "v1.4.0",
			commits: []string{"feat: x"},
			wantTag: "v1.5.0",
		},
		{
			name: "an override yielding major is marked",
			cfg:  confirmMajorCfg(false, config.BumpRule{Type: "perf", Bump: "major"}), tag: "v1.4.0",
			commits: []string{"perf: rewrite the engine", "fix: small"},
			wantTag: "v2.0.0", wantMarked: []string{"perf: rewrite the engine"},
		},
		{
			name: "an override demoting breaking commits leaves nothing to confirm",
			cfg:  confirmMajorCfg(false, config.BumpRule{Breaking: boolPtr(true), Bump: "minor"}), tag: "v1.4.0",
			commits: []string{"feat!: x"},
			wantTag: "v1.5.0",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, r := resolveStay(t, tc.cfg, tc.tag, tc.allowMajor, tc.commits...)

			assert.Equal(t, tc.wantTag, res.Tag)
			assert.Equal(t, tc.wantMarked, r.UnconfirmedMajor())
			if tc.wantWarned {
				assert.Len(t, r.Warnings(), 1)
			} else {
				assert.Empty(t, r.Warnings())
			}
		})
	}
}

// Every subject is kept, not only the first five the hold's warning lists: the commands decide how
// many to show.
func TestResolve_ConfirmMajor_KeepsEveryForcingCommit(t *testing.T) {
	commits := []string{"feat!: a", "feat!: b", "feat!: c", "feat!: d", "feat!: e", "feat!: f"}
	_, r := resolveStay(t, confirmMajorCfg(false), "v1.4.0", false, commits...)
	assert.Equal(t, commits, r.UnconfirmedMajor())
}

// A second resolution starts clean.
func TestResolve_ConfirmMajor_ResetsBetweenCalls(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0\n", "", nil)
	mr.QueueResponse("feat!: x\x00", "", nil)
	mr.QueueResponse("v1.4.0\n", "", nil)
	mr.QueueResponse("fix: y\x00", "", nil)
	r := semver.New(mr, confirmMajorCfg(false))

	_, err := r.Resolve()
	require.NoError(t, err)
	require.NotEmpty(t, r.UnconfirmedMajor())
	_, err = r.Resolve()
	require.NoError(t, err)
	assert.Empty(t, r.UnconfirmedMajor())
}

// semver-per-env's bump: auto environments reach the same entry point through BumpAuto.
func TestBumpAuto_ConfirmMajor(t *testing.T) {
	r := semver.New(nil, confirmMajorCfg(false))

	v, err := r.BumpAuto([]string{"1.4.0"}, []string{"feat!: x"})
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", v)
	assert.Equal(t, []string{"feat!: x"}, r.UnconfirmedMajor())

	_, err = r.BumpAuto([]string{"2.0.0"}, []string{"fix: y"})
	require.NoError(t, err)
	assert.Empty(t, r.UnconfirmedMajor(), "each call starts clean")
}

// Opening a pre-release series on a new major is an automatic major too.
func TestResolve_ConfirmMajor_PreReleaseOnANewMajor(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.3.0\n", "", nil)     // tags
	mr.QueueResponse("feat!: x\x00", "", nil) // log since the last final
	mr.QueueResponse("v1.3.0\n", "", nil)     // tags merged into HEAD
	r := semver.New(mr, confirmMajorCfg(false))
	r.SetPreRelease("rc")

	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0-rc.1", res.Tag)
	assert.Equal(t, []string{"feat!: x"}, r.UnconfirmedMajor())
}

// On a version branch a major leaves the branch's range: that error stands.
func TestResolve_ConfirmMajor_VersionBranchKeepsItsOutOfRangeError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0\n", "", nil)
	mr.QueueResponse("feat!: b\x00", "", nil)
	r := semver.New(mr, confirmMajorCfg(false))
	rg := semver.RangeFrom(config.BranchRange{Major: 1}, "release/1.x")
	r.SetMaintenanceRange(&rg)

	_, err := r.Resolve()
	require.Error(t, err)
	assert.True(t, errors.Is(err, semver.ErrOutOfRange), "got %v", err)
}
