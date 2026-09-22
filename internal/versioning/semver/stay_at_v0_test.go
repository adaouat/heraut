package semver_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stayAtV0Cfg(overrides ...config.BumpRule) *config.Config {
	return &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver",
			TagPrefix: strPtr("v"),
			Bump:      &config.BumpConfig{StayAtV0: true, Overrides: overrides},
		},
	}
}

// resolveStay resolves against a repo whose latest tag is tag and whose commits since it are commits.
func resolveStay(t *testing.T, cfg *config.Config, tag string, allowMajor bool, commits ...string) (versioning.Result, *semver.Resolver) {
	t.Helper()
	mr := exectest.NewMockRunner()
	mr.QueueResponse(tag+"\n", "", nil)
	mr.QueueResponse(strings.Join(commits, "\x00")+"\x00", "", nil)
	r := semver.New(mr, cfg)
	r.SetAllowMajor(allowMajor)
	res, err := r.Resolve()
	require.NoError(t, err)
	return res, r
}

func TestResolve_StayAtV0(t *testing.T) {
	noStay := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}
	tests := []struct {
		name        string
		cfg         *config.Config
		tag         string
		allowMajor  bool
		commits     []string
		wantVersion string
		wantBump    versioning.BumpType
		wantWarn    []string // substrings expected in the single warning; nil means no warning
		notWarn     []string
	}{
		{
			name: "breaking at 0.x is held back to minor", cfg: stayAtV0Cfg(), tag: "v0.68.0",
			commits:     []string{"feat!: break the api", "fix: small"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"1.0.0 → 0.69.0", "--allow-major", "feat!: break the api"},
			notWarn:  []string{"fix: small"},
		},
		{
			name: "0.0.x holds back to 0.1.0", cfg: stayAtV0Cfg(), tag: "v0.0.5",
			commits:     []string{"feat!: x"},
			wantVersion: "0.1.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"1.0.0 → 0.1.0"},
		},
		{
			name: "BREAKING CHANGE footer counts and lists the subject only", cfg: stayAtV0Cfg(), tag: "v0.68.0",
			commits:     []string{"feat: new thing\n\nBREAKING CHANGE: the old thing is gone"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"feat: new thing"},
			notWarn:  []string{"the old thing is gone"},
		},
		{
			name: "self-retiring: major >= 1 is untouched", cfg: stayAtV0Cfg(), tag: "v1.4.0",
			commits:     []string{"feat!: x"},
			wantVersion: "2.0.0", wantBump: versioning.BumpMajor,
		},
		{
			name: "minor at 0.x is untouched", cfg: stayAtV0Cfg(), tag: "v0.68.0",
			commits:     []string{"feat: x"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
		},
		{
			name: "patch at 0.x is untouched", cfg: stayAtV0Cfg(), tag: "v0.68.0",
			commits:     []string{"fix: x"},
			wantVersion: "0.68.1", wantBump: versioning.BumpPatch,
		},
		{
			name: "allow-major lifts the hold", cfg: stayAtV0Cfg(), tag: "v0.68.0", allowMajor: true,
			commits:     []string{"feat!: x"},
			wantVersion: "1.0.0", wantBump: versioning.BumpMajor,
		},
		{
			name: "off by default", cfg: noStay, tag: "v0.68.0",
			commits:     []string{"feat!: x"},
			wantVersion: "1.0.0", wantBump: versioning.BumpMajor,
		},
		{
			name: "an override yielding major is held back too",
			cfg:  stayAtV0Cfg(config.BumpRule{Type: "fix", Bump: "major"}), tag: "v0.68.0",
			commits:     []string{"fix: bug"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"fix: bug"},
		},
		{
			name: "exactly five major commits list all five and no 'and N more' line", cfg: stayAtV0Cfg(), tag: "v0.1.0",
			commits:     []string{"feat!: break 1", "feat!: break 2", "feat!: break 3", "feat!: break 4", "feat!: break 5"},
			wantVersion: "0.2.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"  - feat!: break 1", "  - feat!: break 5"},
			notWarn:  []string{"… and"},
		},
		{
			name: "an override that already demotes breaking triggers nothing",
			cfg:  stayAtV0Cfg(config.BumpRule{Breaking: boolPtr(true), Bump: "minor"}), tag: "v0.68.0",
			commits:     []string{"feat!: x"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, r := resolveStay(t, tc.cfg, tc.tag, tc.allowMajor, tc.commits...)
			assert.Equal(t, tc.wantVersion, res.Version)
			assert.Equal(t, tc.wantBump, res.Bump)

			got := r.Warnings()
			joined := strings.Join(got, "\n")
			for _, s := range tc.notWarn {
				assert.NotContains(t, joined, s)
			}
			if tc.wantWarn == nil {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1)
			for _, s := range tc.wantWarn {
				assert.Contains(t, got[0], s)
			}
		})
	}
}

func TestResolve_StayAtV0_WarningFormat(t *testing.T) {
	_, r := resolveStay(t, stayAtV0Cfg(), "v0.68.0", false, "feat!: break the api")
	assert.Equal(t, []string{
		"major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (pass --allow-major on this run to get 1.0.0 instead)\n" +
			"  - feat!: break the api",
	}, r.Warnings())
	assert.Equal(t, []string{"1.0.0"}, r.WouldBeVersions())
}

func TestResolve_StayAtV0_WarningCapsListedCommits(t *testing.T) {
	var commits []string
	for i := 1; i <= 7; i++ {
		commits = append(commits, fmt.Sprintf("feat!: break %d", i))
	}
	_, r := resolveStay(t, stayAtV0Cfg(), "v0.1.0", false, commits...)

	require.Len(t, r.Warnings(), 1)
	w := r.Warnings()[0]
	for i := 1; i <= 5; i++ {
		assert.Contains(t, w, fmt.Sprintf("  - feat!: break %d", i))
	}
	assert.NotContains(t, w, "feat!: break 6")
	assert.NotContains(t, w, "feat!: break 7")
	assert.True(t, strings.HasSuffix(w, "\n  … and 2 more"), "got: %q", w)
}

func TestResolve_StayAtV0_ManualModeAndSetVersionUntouched(t *testing.T) {
	cfg := stayAtV0Cfg()
	cfg.Versioning.Bump.Mode = "manual"
	r := semver.New(exectest.NewMockRunner(), cfg)
	r.SetVersionOverride("1.0.0")

	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", res.Version)
	assert.Empty(t, r.Warnings())
}

func TestResolve_StayAtV0_WarningsResetBetweenCalls(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break\x00", "", nil)
	r := semver.New(mr, stayAtV0Cfg())

	_, err := r.Resolve()
	require.NoError(t, err)
	require.Len(t, r.Warnings(), 1)

	r.SetVersionOverride("0.70.0") // the second resolution holds nothing back
	_, err = r.Resolve()
	require.NoError(t, err)
	assert.Empty(t, r.Warnings(), "a later Resolve with nothing held back must not repeat the old warning")
}

func TestResolve_StayAtV0_WouldBeVersionsStaysParallelToWarningsAndResets(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break\x00", "", nil)
	r := semver.New(mr, stayAtV0Cfg())

	_, err := r.Resolve()
	require.NoError(t, err)
	require.Len(t, r.Warnings(), 1)
	require.Len(t, r.WouldBeVersions(), 1)
	assert.Equal(t, "1.0.0", r.WouldBeVersions()[0])

	r.SetVersionOverride("0.70.0") // the second resolution holds nothing back
	_, err = r.Resolve()
	require.NoError(t, err)
	assert.Empty(t, r.Warnings())
	assert.Empty(t, r.WouldBeVersions(), "a later Resolve with nothing held back must not repeat the old wouldBe version")
}

func TestBumpAuto_StayAtV0(t *testing.T) {
	r := semver.New(nil, stayAtV0Cfg())
	got, err := r.BumpAuto([]string{"0.68.0"}, []string{"feat!: x"})
	require.NoError(t, err)
	assert.Equal(t, "0.69.0", got)
	require.Len(t, r.Warnings(), 1)
	assert.Contains(t, r.Warnings()[0], "1.0.0 → 0.69.0")
	assert.Equal(t, []string{"1.0.0"}, r.WouldBeVersions())

	r.SetAllowMajor(true)
	got, err = r.BumpAuto([]string{"0.68.0"}, []string{"feat!: x"})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", got)
	assert.Empty(t, r.Warnings(), "allow-major clears the previous run's warning")
	assert.Empty(t, r.WouldBeVersions(), "allow-major clears the previous run's wouldBe version")
}

func TestBumpAuto_StayAtV0_WarningsResetBetweenCalls(t *testing.T) {
	r := semver.New(nil, stayAtV0Cfg())
	_, err := r.BumpAuto([]string{"0.68.0"}, []string{"feat!: x"})
	require.NoError(t, err)
	require.Len(t, r.Warnings(), 1)

	_, err = r.BumpAuto([]string{"0.69.0"}, []string{"feat: y"})
	require.NoError(t, err)
	assert.Empty(t, r.Warnings(), "a later call with nothing held back must not repeat the old warning")
}

func TestWarnings_ReturnsACopy(t *testing.T) {
	r := semver.New(nil, stayAtV0Cfg())
	_, err := r.BumpAuto([]string{"0.68.0"}, []string{"feat!: x"})
	require.NoError(t, err)

	r.Warnings()[0] = "mutated"
	assert.Contains(t, r.Warnings()[0], "held back", "callers must not be able to mutate the recorded warnings")
}

func TestResolve_StayAtV0_NoTagsYet_ReturnsInitialVersionWithoutWarning(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git tag -l v*  → no tags yet
	r := semver.New(mr, stayAtV0Cfg())

	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "0.1.0", res.Version)
	assert.Equal(t, "v0.1.0", res.Tag)
	assert.Equal(t, versioning.BumpNone, res.Bump)
	assert.Empty(t, r.Warnings())
	require.Len(t, mr.Calls, 1, "no tags means no commit walk")
}

func TestBumpAuto_StayAtV0_NoTagsYet_ReturnsInitialVersionWithoutWarning(t *testing.T) {
	r := semver.New(nil, stayAtV0Cfg())

	got, err := r.BumpAuto(nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "0.1.0", got)
	assert.Empty(t, r.Warnings())
}
