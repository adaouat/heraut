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

func strPtr(s string) *string { return &s }

func TestResolve_InitialVersion(t *testing.T) {
	mr := exectest.NewMockRunner()
	// git tag -l returns empty (no tags)
	mr.QueueResponse("", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:       "semver",
			InitialVersion: "0.1.0",
			TagPrefix:      strPtr("v"),
		},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "0.1.0", result.Version)
	assert.Equal(t, "v0.1.0", result.Tag)
	assert.Equal(t, "", result.CurrentTag)
	assert.Equal(t, versioning.BumpNone, result.Bump)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"tag", "-l", "v*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestResolve_InitialVersionDefault(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver"},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "0.1.0", result.Version)
	assert.Equal(t, "v0.1.0", result.Tag)
}

func TestResolve_NoPrefixInitial(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	empty := ""
	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:       "semver",
			TagPrefix:      &empty,
			InitialVersion: "1.0.0",
		},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, "1.0.0", result.Tag)

	assert.Equal(t, []string{"tag", "-l", "*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestResolve_BumpMinor(t *testing.T) {
	mr := exectest.NewMockRunner()
	// git tag -l returns current tags
	mr.QueueResponse("v1.2.3\nv1.2.2\nv1.0.0\n", "", nil)
	// git log returns commits (null-byte separated full messages)
	mr.QueueResponse("feat: add new feature\x00fix: a small fix\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver",
			TagPrefix: strPtr("v"),
		},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "1.3.0", result.Version)
	assert.Equal(t, "v1.3.0", result.Tag)
	assert.Equal(t, "v1.2.3", result.CurrentTag)
	assert.Equal(t, versioning.BumpMinor, result.Bump)

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"log", "v1.2.3..HEAD", "--format=%B%x00"}, mr.Calls[1].Args)
}

func TestResolve_BumpMajor_Breaking(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v2.0.0\n", "", nil)
	mr.QueueResponse("feat!: breaking change\x00feat: add feature\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "3.0.0", result.Version)
	assert.Equal(t, versioning.BumpMajor, result.Bump)
}

func TestResolve_BumpPatch_FixOnly(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)
	mr.QueueResponse("fix: correct typo\x00chore: update deps\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "1.0.1", result.Version)
	assert.Equal(t, versioning.BumpPatch, result.Bump)
}

func TestResolve_BumpPatch_FallbackChoreOnly(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.5.0\n", "", nil)
	mr.QueueResponse("chore: update deps\x00docs: improve readme\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "1.5.1", result.Version)
	assert.Equal(t, versioning.BumpPatch, result.Bump)
}

func TestResolve_NoCommitsSinceTag_Error(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)
	mr.QueueResponse("", "", nil) // no commits (empty output)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no commits")
}

func TestResolve_ManualMode_NoOverride_Error(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver",
			Bump:     "manual",
		},
	}

	r := semver.New(mr, cfg)
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manual")
	// no git calls should have been made
	assert.Len(t, mr.Calls, 0)
}

func TestResolve_ManualMode_WithOverride(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver",
			Bump:      "manual",
			TagPrefix: strPtr("v"),
		},
	}

	r := semver.New(mr, cfg)
	r.SetVersionOverride("2.5.0")
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2.5.0", result.Version)
	assert.Equal(t, "v2.5.0", result.Tag)
	assert.Len(t, mr.Calls, 0)
}

// TestResolve_ManualMode_WithPrefixedOverride verifies that passing the full tag
// (e.g. "v2.5.0") to SetVersionOverride produces the same result as passing the
// bare version ("2.5.0") — the prefix is stripped before building the tag so
// piping `heraut version next` into `--version` doesn't double the prefix.
func TestResolve_ManualMode_WithPrefixedOverride(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver",
			Bump:      "manual",
			TagPrefix: strPtr("v"),
		},
	}

	r := semver.New(mr, cfg)
	r.SetVersionOverride("v2.5.0") // full tag, as returned by heraut version next
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2.5.0", result.Version) // prefix stripped from Version
	assert.Equal(t, "v2.5.0", result.Tag)    // prefix applied exactly once
	assert.Len(t, mr.Calls, 0)
}

// TestResolve_AutoMode_WithPrefixedOverride verifies the same stripping for bump:auto.
func TestResolve_AutoMode_WithPrefixedOverride(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver",
			Bump:      "auto",
			TagPrefix: strPtr("v"),
		},
	}

	r := semver.New(mr, cfg)
	r.SetVersionOverride("v1.0.0")
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, "v1.0.0", result.Tag)
	assert.Len(t, mr.Calls, 0)
}

// TestResolve_AutoMode_WithOverride verifies that SetVersionOverride takes effect even
// when bump is "auto" — the override must short-circuit the auto resolution path so
// callers can pin a specific version via --version without switching to bump: manual.
func TestResolve_AutoMode_WithOverride(t *testing.T) {
	mr := exectest.NewMockRunner()
	// No git calls should be made when an override is set.

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver",
			Bump:      "auto",
			TagPrefix: strPtr("v"),
		},
	}

	r := semver.New(mr, cfg)
	r.SetVersionOverride("1.0.0")
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, "v1.0.0", result.Tag)
	assert.Equal(t, versioning.BumpNone, result.Bump)
	// Override must short-circuit auto — no git calls.
	assert.Len(t, mr.Calls, 0)
}

func TestResolve_BreakingChange_Footer(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)
	// Full commit body with BREAKING CHANGE footer, null-byte separated
	mr.QueueResponse("fix: something\n\nBREAKING CHANGE: removed API\n\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2.0.0", result.Version)
	assert.Equal(t, versioning.BumpMajor, result.Bump)
}

func TestResolve_Minor_Before_Patch_Priority(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v0.5.0\n", "", nil)
	// mix of fix and feat — feat wins → minor
	mr.QueueResponse("fix: a bug\x00feat: new thing\x00fix: another bug\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "0.6.0", result.Version)
	assert.Equal(t, versioning.BumpMinor, result.Bump)
}

// Edge case: git's default version:refname sort orders "v1.3.0-rc.1" above
// "v1.2.3" (without versionsort.suffix configured), so the pre-release tag
// lands first. resolveAuto must skip it and use the first tag that parses as
// bare MAJOR.MINOR.PATCH — mirroring the CalVer resolver's skip-unparsable
// behavior — rather than failing BumpVersion on "3.0-rc.1".
func TestResolve_SkipsPreReleaseTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.3.0-rc.1\nv1.2.3\n", "", nil)
	mr.QueueResponse("fix: a small fix\x00", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "1.2.4", result.Version)
	assert.Equal(t, "v1.2.4", result.Tag)
	assert.Equal(t, "v1.2.3", result.CurrentTag)
	assert.Equal(t, versioning.BumpPatch, result.Bump)

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"log", "v1.2.3..HEAD", "--format=%B%x00"}, mr.Calls[1].Args)
}

// When every tag is a pre-release (none parse as bare MAJOR.MINOR.PATCH),
// resolveAuto falls back to the initial version — same as having no tags at
// all — instead of erroring on the unparsable pre-release version.
func TestResolve_AllTagsPreRelease_UsesInitialVersion(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0-rc.1\nv1.0.0-rc.0\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "0.1.0", result.Version)
	assert.Equal(t, "v0.1.0", result.Tag)
	assert.Equal(t, versioning.BumpNone, result.Bump)
	assert.Empty(t, result.CurrentTag)

	require.Len(t, mr.Calls, 1)
}

// Edge case: v1.9.0 → v1.10.0 (not v1.100.0)
func TestBumpVersion_MinorDoesNotCorruptPatch(t *testing.T) {
	got, err := semver.BumpVersion("1.9.0", versioning.BumpMinor)
	require.NoError(t, err)
	assert.Equal(t, "1.10.0", got)
}

func TestBumpVersion_PatchIncrement(t *testing.T) {
	got, err := semver.BumpVersion("1.9.9", versioning.BumpPatch)
	require.NoError(t, err)
	assert.Equal(t, "1.9.10", got)
}

func TestBumpVersion_MajorReset(t *testing.T) {
	got, err := semver.BumpVersion("1.5.3", versioning.BumpMajor)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", got)
}

func TestDetermineBump(t *testing.T) {
	tests := []struct {
		name    string
		commits []string
		want    versioning.BumpType
	}{
		{"feat → minor", []string{"feat: add x"}, versioning.BumpMinor},
		{"fix → patch", []string{"fix: y"}, versioning.BumpPatch},
		{"feat! → major", []string{"feat!: breaking"}, versioning.BumpMajor},
		{"fix! → major", []string{"fix!: also breaking"}, versioning.BumpMajor},
		{"feat(scope)! → major", []string{"feat(api)!: remove endpoint"}, versioning.BumpMajor},
		{"chore only → patch fallback", []string{"chore: bump deps"}, versioning.BumpPatch},
		{"major beats minor beats patch", []string{"fix: y", "feat: x", "feat!: z"}, versioning.BumpMajor},
		{"BREAKING CHANGE footer", []string{"fix: y\n\nBREAKING CHANGE: boom"}, versioning.BumpMajor},
		{"BREAKING-CHANGE hyphenated footer", []string{"fix: y\n\nBREAKING-CHANGE: boom"}, versioning.BumpMajor},
		{"bang in description, not type prefix → not breaking", []string{"fix: handle the foo!: token"}, versioning.BumpPatch},
		{"BREAKING CHANGE mentioned mid-sentence, not a footer → not breaking", []string{"fix: y\n\nThis is not a BREAKING CHANGE: just a mention."}, versioning.BumpPatch},
		{"BREAKING-CHANGE mentioned mid-sentence, not a footer → not breaking", []string{"fix: y\n\nAlso recognize the hyphenated BREAKING-CHANGE: footer as a synonym."}, versioning.BumpPatch},
		{"BREAKING CHANGE starts a wrapped body line, not its paragraph → not breaking", []string{"fix: y\n\nDiscussing isBreaking's\nBREAKING CHANGE: footer check here."}, versioning.BumpPatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := semver.DetermineBump(tc.commits)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBumpAuto_NoTags(t *testing.T) {
	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:       "semver",
			InitialVersion: "0.1.0",
		},
	}
	r := semver.New(nil, cfg)
	got, err := r.BumpAuto(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "0.1.0", got)
}

func TestBumpAuto_WithCommits(t *testing.T) {
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver"},
	}
	r := semver.New(nil, cfg)
	got, err := r.BumpAuto([]string{"1.2.3"}, []string{"feat: add thing"})
	require.NoError(t, err)
	assert.Equal(t, "1.3.0", got)
}

func TestBumpAuto_NoCommitsSinceTag(t *testing.T) {
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver"},
	}
	r := semver.New(nil, cfg)
	_, err := r.BumpAuto([]string{"1.2.3"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no commits")
}

func TestBumpFromDate_Unsupported(t *testing.T) {
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver"},
	}
	r := semver.New(nil, cfg)
	_, err := r.BumpFromDate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestBumpVersion_InvalidFormat(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr string
	}{
		{"too few parts", "1.2", "invalid semver"},
		{"invalid major", "abc.2.3", "invalid major"},
		{"invalid minor", "1.abc.3", "invalid minor"},
		{"invalid patch", "1.2.abc", "invalid patch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := semver.BumpVersion(tc.version, versioning.BumpPatch)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestResolve_GitTagListError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("not a git repo"))

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing git tags")
}

func TestResolve_GitLogError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.2.3\n", "", nil)                  // git tag -l succeeds
	mr.QueueResponse("", "", errors.New("git log failed")) // git log fails

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}

	r := semver.New(mr, cfg)
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading git log")
}
