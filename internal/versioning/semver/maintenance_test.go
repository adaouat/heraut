package semver_test

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func u64(n uint64) *uint64 { return &n }

func mustVersion(t *testing.T, s string) semver.Version {
	t.Helper()
	v, err := semver.Parse(s)
	require.NoError(t, err)
	return v
}

func TestRangeFrom(t *testing.T) {
	tests := []struct {
		name   string
		in     config.BranchRange
		branch string
		lo, hi string
		label  string
	}{
		{"N.N.x", config.BranchRange{Major: 1, Minor: u64(3)}, "release/1.3", "1.3.0", "1.4.0", "1.3.x"},
		{"N.x", config.BranchRange{Major: 1}, "release/1.x", "1.0.0", "2.0.0", "1.x"},
		{"0.x", config.BranchRange{Major: 0}, "release/0.x", "0.0.0", "1.0.0", "0.x"},
		{"0.0.x", config.BranchRange{Major: 0, Minor: u64(0)}, "lts", "0.0.0", "0.1.0", "0.0.x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rg := semver.RangeFrom(tc.in, tc.branch)
			assert.Equal(t, tc.lo, rg.Lo.String())
			assert.Equal(t, tc.hi, rg.Hi.String())
			assert.Equal(t, tc.label, rg.Label)
			assert.Equal(t, tc.branch, rg.Branch)
		})
	}
}

func TestRange_Contains(t *testing.T) {
	line13 := semver.RangeFrom(config.BranchRange{Major: 1, Minor: u64(3)}, "release/1.3")
	line1 := semver.RangeFrom(config.BranchRange{Major: 1}, "release/1.x")
	tests := []struct {
		name string
		rg   semver.Range
		v    string
		want bool
	}{
		{"lower bound inclusive", line13, "1.3.0", true},
		{"patch inside", line13, "1.3.9", true},
		{"upper bound exclusive", line13, "1.4.0", false},
		{"below", line13, "1.2.9", false},
		{"pre-release of Lo compares on core", line13, "1.3.0-rc.1", true},
		{"pre-release of Hi compares on core", line13, "1.4.0-rc.1", false},
		{"build metadata ignored", line13, "1.3.2+5", true},
		{"minor inside N.x", line1, "1.9.0", true},
		{"next major outside N.x", line1, "2.0.0", false},
		{"previous major outside N.x", line1, "0.9.0", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.rg.Contains(mustVersion(t, tc.v)))
		})
	}
}

func TestResolve_Maintenance(t *testing.T) {
	line13 := config.BranchRange{Major: 1, Minor: u64(3)}
	line1 := config.BranchRange{Major: 1}
	line0 := config.BranchRange{Major: 0}

	merged := func(prefix string) []string {
		return []string{"tag", "-l", prefix + "*", "--merged", "HEAD", "--sort=-version:refname"}
	}
	logSince := func(tag string) []string { return []string{"log", tag + "..HEAD", "--format=%B%x00"} }
	// ADR-0065: the collision probe also matches <tag>+*, since a build-metadata tag of the same
	// version is that version's release (ADR-0064).
	probe := func(tag string) []string { return []string{"tag", "-l", tag, tag + "+*"} }

	tests := []struct {
		name      string
		prefix    *string
		stayAtV0  bool
		rg        config.BranchRange
		branch    string
		responses []string // FIFO stdout per git call
		wantCalls [][]string
		wantTag   string
		wantCur   string
		wantBump  versioning.BumpType
		wantErr   error
		errText   []string
		wantWarn  bool
	}{
		{
			name: "patch on line", rg: line13, branch: "release/1.3",
			responses: []string{"v1.3.1\nv1.3.0\n", "fix: x\x00", ""},
			wantCalls: [][]string{merged("v"), logSince("v1.3.1"), probe("v1.3.2")},
			wantTag:   "v1.3.2", wantCur: "v1.3.1", wantBump: versioning.BumpPatch,
		},
		{
			name: "feat out of range", rg: line13, branch: "release/1.3",
			responses: []string{"v1.3.1\nv1.3.0\n", "feat: y\x00"},
			wantCalls: [][]string{merged("v"), logSince("v1.3.1")},
			wantErr:   semver.ErrOutOfRange,
			errText:   []string{"feat: y would release 1.4.0, outside release/1.3 (>=1.3.0 <1.4.0)", "land it on a branch whose range allows it, or on main"},
		},
		{
			name: "feat on 1.x", rg: line1, branch: "release/1.x",
			responses: []string{"v1.4.0\nv1.3.1\n", "feat: z\x00", ""},
			wantCalls: [][]string{merged("v"), logSince("v1.4.0"), probe("v1.5.0")},
			wantTag:   "v1.5.0", wantCur: "v1.4.0", wantBump: versioning.BumpMinor,
		},
		{
			name: "breaking out of 1.x", rg: line1, branch: "release/1.x",
			responses: []string{"v1.4.0\n", "feat!: b\x00"},
			wantCalls: [][]string{merged("v"), logSince("v1.4.0")},
			wantErr:   semver.ErrOutOfRange,
			errText:   []string{"feat!: b would release 2.0.0, outside release/1.x (>=1.0.0 <2.0.0)"},
		},
		{
			name: "tag exists", rg: line13, branch: "release/1.3",
			responses: []string{"v1.3.1\n", "fix: x\x00", "v1.3.2\n"},
			wantCalls: [][]string{merged("v"), logSince("v1.3.1"), probe("v1.3.2")},
			wantErr:   semver.ErrTagExists,
			errText:   []string{"v1.3.2 (cut on another branch)", "pick the next free version with --set-version"},
		},
		{
			name: "build-metadata release of the next version exists", rg: line13, branch: "release/1.3",
			responses: []string{"v1.3.1\n", "fix: x\x00", "v1.3.2+7\n"},
			wantCalls: [][]string{merged("v"), logSince("v1.3.1"), probe("v1.3.2")},
			wantErr:   semver.ErrTagExists,
			errText:   []string{"v1.3.2+7 (cut on another branch)"},
		},
		{
			name: "no in-range base", rg: line13, branch: "release/1.3",
			responses: []string{"v1.2.5\n"},
			wantCalls: [][]string{merged("v")},
			wantErr:   semver.ErrNoInRangeRelease,
			errText:   []string{"no release in range 1.3.x in the history of release/1.3", "tag the branch's starting point or pass --set-version"},
		},
		{
			name: "no commits", rg: line13, branch: "release/1.3",
			responses: []string{"v1.3.1\n", ""},
			wantCalls: [][]string{merged("v"), logSince("v1.3.1")},
			errText:   []string{"no commits since v1.3.1"},
		},
		{
			name: "rel- prefix + build metadata base", prefix: strPtr("rel-"), rg: line13, branch: "release/1.3",
			responses: []string{"rel-1.3.1+5\nrel-1.3.0\n", "fix: x\x00", ""},
			wantCalls: [][]string{merged("rel-"), logSince("rel-1.3.1+5"), probe("rel-1.3.2")},
			wantTag:   "rel-1.3.2", wantCur: "rel-1.3.1+5", wantBump: versioning.BumpPatch,
		},
		{
			name: "pre-release of next core ignored as base", rg: line13, branch: "release/1.3",
			responses: []string{"v1.3.2-rc.1\nv1.3.1\n", "fix: x\x00", ""},
			wantCalls: [][]string{merged("v"), logSince("v1.3.1"), probe("v1.3.2")},
			wantTag:   "v1.3.2", wantCur: "v1.3.1", wantBump: versioning.BumpPatch,
		},
		{
			// v2.0.0 exists on main but is not merged into HEAD, so --merged never lists it.
			name: "higher main tags not reachable", rg: line13, branch: "release/1.3",
			responses: []string{"v1.3.1\nv1.3.0\n", "fix: x\x00", ""},
			wantCalls: [][]string{merged("v"), logSince("v1.3.1"), probe("v1.3.2")},
			wantTag:   "v1.3.2", wantCur: "v1.3.1", wantBump: versioning.BumpPatch,
		},
		{
			name: "stay_at_v0 on 0.x line", stayAtV0: true, rg: line0, branch: "release/0.x",
			responses: []string{"v0.3.1\n", "feat!: b\x00", ""},
			wantCalls: [][]string{merged("v"), logSince("v0.3.1"), probe("v0.4.0")},
			wantTag:   "v0.4.0", wantCur: "v0.3.1", wantBump: versioning.BumpMinor, wantWarn: true,
		},
		{
			name: "out-of-range ordering among merged tags", rg: line13, branch: "release/1.3",
			responses: []string{"v1.4.0\nv1.3.1\nv1.3.0\n", "fix: x\x00", ""},
			wantCalls: [][]string{merged("v"), logSince("v1.3.1"), probe("v1.3.2")},
			wantTag:   "v1.3.2", wantCur: "v1.3.1", wantBump: versioning.BumpPatch,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			for _, out := range tc.responses {
				mr.QueueResponse(out, "", nil)
			}
			cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: tc.prefix}}
			if tc.stayAtV0 {
				cfg.Versioning.Bump = &config.BumpConfig{StayAtV0: true}
			}
			r := semver.New(mr, cfg)
			rg := semver.RangeFrom(tc.rg, tc.branch)
			r.SetMaintenanceRange(&rg)

			res, err := r.Resolve()

			gotCalls := make([][]string, len(mr.Calls))
			for i, c := range mr.Calls {
				assert.Equal(t, "git", c.Name)
				gotCalls[i] = c.Args
			}
			assert.Equal(t, tc.wantCalls, gotCalls)

			if tc.wantErr != nil || len(tc.errText) > 0 {
				require.Error(t, err)
				if tc.wantErr != nil {
					assert.True(t, errors.Is(err, tc.wantErr), "want %v, got %v", tc.wantErr, err)
				}
				for _, s := range tc.errText {
					assert.Contains(t, err.Error(), s)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, res.Tag)
			assert.Equal(t, tc.wantCur, res.CurrentTag)
			assert.Equal(t, tc.wantBump, res.Bump)
			if tc.wantWarn {
				assert.Len(t, r.Warnings(), 1)
			} else {
				assert.Empty(t, r.Warnings())
			}
		})
	}
}

// TestResolve_MaintenanceRangeNil_KeepsGlobalSequence guards the no-branches path: a nil range
// must leave resolveAuto's git call sequence exactly as it is without maintenance support.
func TestResolve_MaintenanceRangeNil_KeepsGlobalSequence(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v2.0.0\nv1.3.1\n", "", nil)
	mr.QueueResponse("fix: x\x00", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver"}}
	r := semver.New(mr, cfg)
	r.SetMaintenanceRange(nil)

	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v2.0.1", res.Tag)
	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "-l", "v*", "--sort=-version:refname"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"log", "v2.0.0..HEAD", "--format=%B%x00"}, mr.Calls[1].Args)
}

func TestResolve_PreReleaseMaintenance(t *testing.T) {
	const nul = "\x00"
	line13 := config.BranchRange{Major: 1, Minor: u64(3)}
	line1 := config.BranchRange{Major: 1}

	tests := []struct {
		name      string
		rg        config.BranchRange
		branch    string
		label     string
		responses []string // FIFO stdout per git call: global listing, merged listing, logs
		wantCalls [][]string
		wantTag   string
		wantCur   string
		wantBump  versioning.BumpType
		wantErr   error
		errText   []string
		wantWarn  []string
	}{
		{
			name: "rc on line", rg: line13, branch: "release/1.3", label: "rc",
			responses: []string{"v2.0.0\nv1.4.0\nv1.3.1\n", "v1.3.1\nv1.3.0\n", "fix: x" + nul},
			wantCalls: [][]string{tagsCall("v"), mergedCall("v"), logCall("v1.3.1")},
			wantTag:   "v1.3.2-rc.1", wantCur: "v1.3.1", wantBump: versioning.BumpPatch,
		},
		{
			name: "rc counter on line", rg: line13, branch: "release/1.3", label: "rc",
			responses: []string{"v2.0.0\nv1.3.2-rc.1\nv1.3.1\n", "v1.3.2-rc.1\nv1.3.1\n", "fix: x" + nul + "fix: y" + nul, "fix: y" + nul},
			wantCalls: [][]string{tagsCall("v"), mergedCall("v"), logCall("v1.3.1"), logCall("v1.3.2-rc.1")},
			wantTag:   "v1.3.2-rc.2", wantCur: "v1.3.2-rc.1", wantBump: versioning.BumpPatch,
		},
		{
			name: "feat rc out of range", rg: line13, branch: "release/1.3", label: "rc",
			responses: []string{"v2.0.0\nv1.3.1\n", "v1.3.1\n", "feat: y" + nul},
			wantCalls: [][]string{tagsCall("v"), mergedCall("v"), logCall("v1.3.1")},
			wantErr:   semver.ErrOutOfRange,
			errText:   []string{"feat: y would release 1.4.0, outside release/1.3 (>=1.3.0 <1.4.0)", "land it on a branch whose range allows it, or on main"},
		},
		{
			name: "main's open series doesn't escalate the line", rg: line13, branch: "release/1.3", label: "rc",
			responses: []string{"v2.1.0-rc.1\nv2.0.0\nv1.3.1\n", "v1.3.1\n", "fix: x" + nul},
			wantCalls: [][]string{tagsCall("v"), mergedCall("v"), logCall("v1.3.1")},
			wantTag:   "v1.3.2-rc.1", wantCur: "v1.3.1", wantBump: versioning.BumpPatch,
		},
		{
			name: "no in-range base", rg: line13, branch: "release/1.3", label: "rc",
			responses: []string{"v2.0.0\nv1.2.5\n", "v1.2.5\n"},
			wantCalls: [][]string{tagsCall("v"), mergedCall("v")},
			wantErr:   semver.ErrNoInRangeRelease,
			errText:   []string{"no release in range 1.3.x in the history of release/1.3"},
		},
		{
			// The line's own open series escalates; a higher series on main must not hide it.
			name: "line's open series escalation is still reported", rg: line1, branch: "release/1.x", label: "rc",
			responses: []string{"v2.1.0-rc.1\nv2.0.0\nv1.4.1-rc.1\nv1.4.0\n", "v1.4.1-rc.1\nv1.4.0\n", "fix: a" + nul + "feat: b" + nul, "feat: b" + nul},
			wantCalls: [][]string{tagsCall("v"), mergedCall("v"), logCall("v1.4.0"), logCall("v1.4.1-rc.1")},
			wantTag:   "v1.5.0-rc.1", wantCur: "v1.4.1-rc.1", wantBump: versioning.BumpMinor,
			wantWarn: []string{"pre-release core escalated 1.4.1 → 1.5.0\n  - feat: b"},
		},
		{
			name: "counter stays global", rg: line13, branch: "release/1.3", label: "rc",
			responses: []string{"v1.3.2-rc.1\nv1.3.1\n", "v1.3.1\n", "fix: x" + nul},
			wantCalls: [][]string{tagsCall("v"), mergedCall("v"), logCall("v1.3.1")},
			wantTag:   "v1.3.2-rc.2", wantCur: "v1.3.1", wantBump: versioning.BumpPatch,
		},
		{
			name: "monotonicity stays global", rg: line13, branch: "release/1.3", label: "beta",
			responses: []string{"v1.3.2-rc.1\nv1.3.1\n", "v1.3.1\n", "fix: x" + nul},
			wantCalls: [][]string{tagsCall("v"), mergedCall("v"), logCall("v1.3.1")},
			wantErr:   semver.ErrPreReleaseRegression,
			errText:   []string{"v1.3.2-beta.1 would sort below existing v1.3.2-rc.1"},
		},
		{
			name: "promotion on line needs no new commit", rg: line13, branch: "release/1.3", label: "rc",
			responses: []string{"v2.0.0\nv1.3.2-beta.2\nv1.3.1\n", "v1.3.2-beta.2\nv1.3.1\n", "fix: x" + nul},
			wantCalls: [][]string{tagsCall("v"), mergedCall("v"), logCall("v1.3.1")},
			wantTag:   "v1.3.2-rc.1", wantCur: "v1.3.2-beta.2", wantBump: versioning.BumpPatch,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			for _, out := range tc.responses {
				mr.QueueResponse(out, "", nil)
			}
			r := semver.New(mr, &config.Config{Versioning: config.Versioning{Strategy: "semver"}})
			r.SetPreRelease(tc.label)
			rg := semver.RangeFrom(tc.rg, tc.branch)
			r.SetMaintenanceRange(&rg)

			res, err := r.Resolve()

			gotCalls := make([][]string, len(mr.Calls))
			for i, c := range mr.Calls {
				assert.Equal(t, "git", c.Name)
				gotCalls[i] = c.Args
			}
			assert.Equal(t, tc.wantCalls, gotCalls)

			if tc.wantErr != nil || len(tc.errText) > 0 {
				require.Error(t, err)
				if tc.wantErr != nil {
					assert.True(t, errors.Is(err, tc.wantErr), "want %v, got %v", tc.wantErr, err)
				}
				for _, s := range tc.errText {
					assert.Contains(t, err.Error(), s)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, res.Tag)
			assert.Equal(t, tc.wantCur, res.CurrentTag)
			assert.Equal(t, tc.wantBump, res.Bump)
			assert.Equal(t, tc.wantWarn, r.Warnings())
		})
	}
}
