package semver_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tagsCall(prefix string) []string {
	return []string{"tag", "-l", prefix + "*", "--sort=-version:refname"}
}

func mergedCall(prefix string) []string {
	return []string{"tag", "-l", prefix + "*", "--merged", "HEAD", "--sort=-version:refname"}
}

func logCall(tag string) []string {
	return []string{"log", tag + "..HEAD", "--format=%B%x00"}
}

func TestResolve_PreRelease(t *testing.T) {
	const nul = "\x00"
	tests := []struct {
		name       string
		prefix     string
		cfg        *config.Config
		responses  []string // stdout of each queued git call, in order
		calls      [][]string
		label      string
		allowMajor bool
		wantTag    string
		wantCur    string
		wantBump   versioning.BumpType
		wantErr    error
		wantErrSub string
		wantWarn   []string
	}{
		{
			name:      "first beta",
			prefix:    "v",
			responses: []string{"v1.3.0\n", "feat: x" + nul, "v1.3.0\n"},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v")},
			label:     "beta", wantTag: "v1.4.0-beta.1", wantCur: "v1.3.0", wantBump: versioning.BumpMinor,
		},
		{
			name:      "beta counter",
			prefix:    "v",
			responses: []string{"v1.4.0-beta.1\nv1.3.0\n", "feat: x" + nul + "fix: y" + nul, "v1.4.0-beta.1\nv1.3.0\n", "fix: y" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v"), logCall("v1.4.0-beta.1")},
			label:     "beta", wantTag: "v1.4.0-beta.2", wantCur: "v1.4.0-beta.1", wantBump: versioning.BumpMinor,
		},
		{
			name:      "switch to rc",
			prefix:    "v",
			responses: []string{"v1.4.0-beta.2\nv1.3.0\n", "feat: x" + nul + "fix: y" + nul, "v1.4.0-beta.2\nv1.3.0\n", "fix: y" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v"), logCall("v1.4.0-beta.2")},
			label:     "rc", wantTag: "v1.4.0-rc.1", wantCur: "v1.4.0-beta.2", wantBump: versioning.BumpMinor,
		},
		{
			name:      "label regression",
			prefix:    "v",
			responses: []string{"v1.4.0-rc.1\nv1.4.0-beta.2\nv1.3.0\n", "feat: x" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0")},
			label:     "beta", wantErr: semver.ErrPreReleaseRegression,
			wantErrSub: "v1.4.0-beta.3 would sort below existing v1.4.0-rc.1 — ship 1.4.0 or use a label that sorts higher",
		},
		{
			name:      "no new commits",
			prefix:    "v",
			responses: []string{"v1.4.0-rc.1\nv1.3.0\n", "feat: x" + nul, "v1.4.0-rc.1\nv1.3.0\n", ""},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v"), logCall("v1.4.0-rc.1")},
			label:     "rc", wantErrSub: "no commits since v1.4.0-rc.1 — create at least one commit before running heraut release",
		},
		{
			name:      "no commits since final",
			prefix:    "v",
			responses: []string{"v1.3.0\n", ""},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0")},
			label:     "rc", wantErrSub: "no commits since v1.3.0",
		},
		{
			name:      "patch series",
			prefix:    "v",
			responses: []string{"v1.3.0\n", "fix: a" + nul, "v1.3.0\n"},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v")},
			label:     "rc", wantTag: "v1.3.1-rc.1", wantCur: "v1.3.0", wantBump: versioning.BumpPatch,
		},
		{
			name:      "minor escalation warns",
			prefix:    "v",
			responses: []string{"v1.3.1-rc.1\nv1.3.0\n", "fix: a" + nul + "feat: b" + nul, "v1.3.1-rc.1\nv1.3.0\n", "feat: b" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v"), logCall("v1.3.1-rc.1")},
			label:     "rc", wantTag: "v1.4.0-rc.1", wantCur: "v1.3.1-rc.1", wantBump: versioning.BumpMinor,
			wantWarn: []string{"pre-release core escalated 1.3.1 → 1.4.0\n  - feat: b"},
		},
		{
			name:      "major escalation blocked",
			prefix:    "v",
			responses: []string{"v1.4.0-rc.1\nv1.3.0\n", "feat: b" + nul + "feat!: c" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0")},
			label:     "rc", wantErr: semver.ErrMajorEscalation,
			wantErrSub: "pre-release series v1.4.0-rc.1 would escalate to a new major 2.0.0: breaking change(s) since v1.3.0 — pass --allow-major to open the 2.0.0 series, or ship 1.4.0 first\n  - feat!: c",
		},
		{
			name:      "major escalation allowed",
			prefix:    "v",
			responses: []string{"v1.4.0-rc.1\nv1.3.0\n", "feat: b" + nul + "feat!: c" + nul, "v1.4.0-rc.1\nv1.3.0\n", "feat!: c" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v"), logCall("v1.4.0-rc.1")},
			label:     "rc", allowMajor: true, wantTag: "v2.0.0-rc.1", wantCur: "v1.4.0-rc.1", wantBump: versioning.BumpMajor,
			wantWarn: []string{"pre-release core escalated 1.4.0 → 2.0.0\n  - feat!: c"},
		},
		{
			name:      "no final yet",
			prefix:    "v",
			responses: []string{"", ""},
			calls:     [][]string{tagsCall("v"), mergedCall("v")},
			label:     "alpha", wantTag: "v0.1.0-alpha.1", wantCur: "", wantBump: versioning.BumpNone,
		},
		{
			name:      "merged subset: previous tag follows HEAD, counter follows all tags",
			prefix:    "v",
			responses: []string{"v1.4.0-beta.2\nv1.4.0-beta.1\nv1.3.0\n", "feat: x" + nul, "v1.4.0-beta.1\nv1.3.0\n", "fix: y" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v"), logCall("v1.4.0-beta.1")},
			label:     "beta", wantTag: "v1.4.0-beta.3", wantCur: "v1.4.0-beta.1", wantBump: versioning.BumpMinor,
		},
		{
			name:      "merged subset: commit requirement uses the merged previous tag",
			prefix:    "v",
			responses: []string{"v1.4.0-beta.2\nv1.4.0-beta.1\nv1.3.0\n", "feat: x" + nul, "v1.4.0-beta.1\nv1.3.0\n", ""},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v"), logCall("v1.4.0-beta.1")},
			label:     "beta", wantErrSub: "no commits since v1.4.0-beta.1 — create at least one commit",
		},
		{
			name:      "merged subset: only the final is reachable, no extra log call",
			prefix:    "v",
			responses: []string{"v1.4.0-beta.2\nv1.3.0\n", "feat: x" + nul, "v1.3.0\n"},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v")},
			label:     "beta", wantTag: "v1.4.0-beta.3", wantCur: "v1.3.0", wantBump: versioning.BumpMinor,
		},
		{
			name:      "no final: existing pre-release is the previous tag",
			prefix:    "v",
			responses: []string{"v0.1.0-alpha.1\n", "v0.1.0-alpha.1\n", "feat: x" + nul},
			calls:     [][]string{tagsCall("v"), mergedCall("v"), logCall("v0.1.0-alpha.1")},
			label:     "alpha", wantTag: "v0.1.0-alpha.2", wantCur: "v0.1.0-alpha.1", wantBump: versioning.BumpNone,
		},
		{
			name:      "no final: no commits since the existing pre-release",
			prefix:    "v",
			responses: []string{"v0.1.0-alpha.1\n", "v0.1.0-alpha.1\n", ""},
			calls:     [][]string{tagsCall("v"), mergedCall("v"), logCall("v0.1.0-alpha.1")},
			label:     "alpha", wantErrSub: "no commits since v0.1.0-alpha.1 — create at least one commit",
		},
		{
			name:      "no final: lower label regresses below existing pre-release",
			prefix:    "v",
			responses: []string{"v0.1.0-beta.1\n"},
			calls:     [][]string{tagsCall("v")},
			label:     "alpha", wantErr: semver.ErrPreReleaseRegression, wantErrSub: "v0.1.0-alpha.1 would sort below existing v0.1.0-beta.1",
		},
		{
			name:      "no final: escalation past an older open series",
			prefix:    "v",
			responses: []string{"v0.0.5-alpha.1\n", "v0.0.5-alpha.1\n", "feat: x" + nul},
			calls:     [][]string{tagsCall("v"), mergedCall("v"), logCall("v0.0.5-alpha.1")},
			label:     "alpha", wantTag: "v0.1.0-alpha.1", wantCur: "v0.0.5-alpha.1", wantBump: versioning.BumpNone,
			wantWarn: []string{"pre-release core escalated 0.0.5 → 0.1.0"},
		},
		{
			name:      "custom prefix",
			prefix:    "rel-",
			responses: []string{"rel-1.3.0\nv9.9.9\n", "feat: x" + nul, "rel-1.3.0\n"},
			calls:     [][]string{tagsCall("rel-"), logCall("rel-1.3.0"), mergedCall("rel-")},
			label:     "beta", wantTag: "rel-1.4.0-beta.1", wantCur: "rel-1.3.0", wantBump: versioning.BumpMinor,
		},
		{
			name:      "core below an open higher series does not escalate",
			prefix:    "v",
			responses: []string{"v1.5.0-rc.1\nv1.3.0\n", "fix: a" + nul, "v1.5.0-rc.1\nv1.3.0\n"},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v")},
			label:     "rc", wantTag: "v1.3.1-rc.1", wantCur: "v1.3.0", wantBump: versioning.BumpPatch,
		},
		{
			name:      "bare label tag is not a counter",
			prefix:    "v",
			responses: []string{"v1.4.0-rc\nv1.3.0\n", "feat: x" + nul, "v1.4.0-rc\nv1.3.0\n", "feat: x" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0"), mergedCall("v"), logCall("v1.4.0-rc")},
			label:     "rc", wantTag: "v1.4.0-rc.1", wantCur: "v1.4.0-rc", wantBump: versioning.BumpMinor,
		},
		{
			name:      "longer pre-release of the label blocks the counter",
			prefix:    "v",
			responses: []string{"v1.4.0-rc.1.1\nv1.3.0\n", "feat: x" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0")},
			label:     "rc", wantErr: semver.ErrPreReleaseRegression, wantErrSub: "would sort below existing v1.4.0-rc.1.1",
		},
		{
			name:      "uppercase label sorts below lowercase",
			prefix:    "v",
			responses: []string{"v1.4.0-rc.1\nv1.3.0\n", "feat: x" + nul},
			calls:     [][]string{tagsCall("v"), logCall("v1.3.0")},
			label:     "RC", wantErr: semver.ErrPreReleaseRegression, wantErrSub: "v1.4.0-RC.1 would sort below existing v1.4.0-rc.1",
		},
		{
			name:      "core is computed from the newest final",
			prefix:    "v",
			responses: []string{"v1.4.0\nv1.3.0\n", "fix: a" + nul, "v1.4.0\nv1.3.0\n"},
			calls:     [][]string{tagsCall("v"), logCall("v1.4.0"), mergedCall("v")},
			label:     "rc", wantTag: "v1.4.1-rc.1", wantCur: "v1.4.0", wantBump: versioning.BumpPatch,
		},
		{
			name:      "stay_at_v0 holds a breaking change at 0.x",
			prefix:    "v",
			cfg:       stayAtV0Cfg(),
			responses: []string{"v0.5.0\n", "feat!: x" + nul, "v0.5.0\n"},
			calls:     [][]string{tagsCall("v"), logCall("v0.5.0"), mergedCall("v")},
			label:     "rc", wantTag: "v0.6.0-rc.1", wantCur: "v0.5.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.6.0 (pass --allow-major on this run to get 1.0.0 instead)\n  - feat!: x"},
		},
		{
			name:      "stay_at_v0 lifted by allow-major",
			prefix:    "v",
			cfg:       stayAtV0Cfg(),
			responses: []string{"v0.5.0\n", "feat!: x" + nul, "v0.5.0\n"},
			calls:     [][]string{tagsCall("v"), logCall("v0.5.0"), mergedCall("v")},
			label:     "rc", allowMajor: true, wantTag: "v1.0.0-rc.1", wantCur: "v0.5.0", wantBump: versioning.BumpMajor,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			for _, out := range tc.responses {
				mr.QueueResponse(out, "", nil)
			}
			cfg := tc.cfg
			if cfg == nil {
				cfg = &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr(tc.prefix)}}
			}
			r := semver.New(mr, cfg)
			r.SetPreRelease(tc.label)
			r.SetAllowMajor(tc.allowMajor)

			res, err := r.Resolve()

			require.Len(t, mr.Calls, len(tc.calls))
			for i, want := range tc.calls {
				assert.Equal(t, "git", mr.Calls[i].Name)
				assert.Equal(t, want, mr.Calls[i].Args, "call %d", i)
			}
			if tc.wantErr != nil || tc.wantErrSub != "" {
				require.Error(t, err)
				if tc.wantErr != nil {
					assert.True(t, errors.Is(err, tc.wantErr), "got %v", err)
				}
				assert.Contains(t, err.Error(), tc.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, res.Tag)
			assert.Equal(t, strings.TrimPrefix(tc.wantTag, tc.prefix), res.Version)
			assert.Equal(t, tc.wantCur, res.CurrentTag)
			assert.Equal(t, tc.wantBump, res.Bump)
			assert.Equal(t, tc.wantWarn, r.Warnings())
			assert.Len(t, r.WouldBeVersions(), len(r.Warnings()))
		})
	}
}

func TestResolve_PreRelease_EscalationWarningHasNoWouldBeVersion(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.3.1-rc.1\nv1.3.0\n", "", nil)
	mr.QueueResponse("fix: a\x00feat: b\x00", "", nil)
	mr.QueueResponse("v1.3.1-rc.1\nv1.3.0\n", "", nil)
	mr.QueueResponse("feat: b\x00", "", nil)
	r := semver.New(mr, &config.Config{Versioning: config.Versioning{Strategy: "semver"}})
	r.SetPreRelease("rc")
	_, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, []string{""}, r.WouldBeVersions())
}

func TestResolve_PreRelease_EscalationWarningCapsListedCommits(t *testing.T) {
	var commits []string
	for i := 0; i < 7; i++ {
		commits = append(commits, "feat!: c"+string(rune('a'+i)))
	}
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0-rc.1\nv1.3.0\n", "", nil)
	mr.QueueResponse(strings.Join(commits, "\x00")+"\x00", "", nil)
	mr.QueueResponse("v1.4.0-rc.1\nv1.3.0\n", "", nil)
	mr.QueueResponse("feat!: ca\x00", "", nil)
	r := semver.New(mr, &config.Config{Versioning: config.Versioning{Strategy: "semver"}})
	r.SetPreRelease("rc")
	r.SetAllowMajor(true)
	_, err := r.Resolve()
	require.NoError(t, err)
	require.Len(t, r.Warnings(), 1)
	assert.Equal(t, 5, strings.Count(r.Warnings()[0], "\n  - "))
	assert.Contains(t, r.Warnings()[0], "\n  … and 2 more")
}

func TestResolve_PreRelease_WarningsResetBetweenCalls(t *testing.T) {
	mr := exectest.NewMockRunner()
	for i := 0; i < 2; i++ {
		mr.QueueResponse("v1.3.1-rc.1\nv1.3.0\n", "", nil)
		mr.QueueResponse("fix: a\x00feat: b\x00", "", nil)
		mr.QueueResponse("v1.3.1-rc.1\nv1.3.0\n", "", nil)
		mr.QueueResponse("feat: b\x00", "", nil)
	}
	r := semver.New(mr, &config.Config{Versioning: config.Versioning{Strategy: "semver"}})
	r.SetPreRelease("rc")
	for i := 0; i < 2; i++ {
		_, err := r.Resolve()
		require.NoError(t, err)
		assert.Len(t, r.Warnings(), 1)
	}
}

func TestResolve_PreRelease_OverrideAndManualBypass(t *testing.T) {
	mr := exectest.NewMockRunner()
	r := semver.New(mr, &config.Config{Versioning: config.Versioning{Strategy: "semver"}})
	r.SetPreRelease("rc")
	r.SetVersionOverride("1.2.3-beta.1")
	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3-beta.1", res.Tag)
	assert.Empty(t, mr.Calls)
}

func TestResolve_EmptyPreReleaseLeavesAutoPathUnchanged(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.3.0\n", "", nil)
	mr.QueueResponse("feat: x\x00", "", nil)
	r := semver.New(mr, &config.Config{Versioning: config.Versioning{Strategy: "semver"}})
	r.SetPreRelease("rc")
	r.SetPreRelease("")
	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v1.4.0", res.Tag)
	require.Len(t, mr.Calls, 2)
	assert.Equal(t, tagsCall("v"), mr.Calls[0].Args)
	assert.Equal(t, logCall("v1.3.0"), mr.Calls[1].Args)
}

func TestValidatePreReleaseLabel(t *testing.T) {
	for _, ok := range []string{"rc", "beta", "next", "rc-x", "0a", "RC"} {
		assert.NoError(t, semver.ValidatePreReleaseLabel(ok), ok)
	}
	for _, bad := range []string{"", "1", "007", "rc.1", "rc_x", "rç", "rc 1"} {
		assert.Error(t, semver.ValidatePreReleaseLabel(bad), bad)
	}
}
