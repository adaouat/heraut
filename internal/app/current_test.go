package app_test

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func TestCurrentTag_Semver(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.2.3\nv1.2.2\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}
	got, err := app.CurrentTag(mr, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", got)

	// Verify glob arg
	assert.Equal(t, []string{"tag", "-l", "v*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestCurrentTag_Calver(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.05.1\n", "", nil)

	empty := ""
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "calver", TagPrefix: &empty},
	}
	got, err := app.CurrentTag(mr, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "2026.05.1", got)
}

func TestCurrentTag_SemverPerEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("prod/1.2.3\nprod/1.2.2\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"prod": {TagFormat: "prod/{version}"},
		},
	}
	got, err := app.CurrentTag(mr, cfg, "prod", false)
	require.NoError(t, err)
	assert.Equal(t, "prod/1.2.3", got)

	// Verify glob contains prod pattern
	assert.Contains(t, mr.Calls[0].Args[2], "prod")
}

func TestCurrentTag_PerEnvCommonTagFormat(t *testing.T) {
	// Bug T54: with a top-level tag_format and no per-env override, the glob
	// must still resolve. Previously currentTagGlob read envCfg.TagFormat directly,
	// which was empty here, producing "tag format must contain {version}".
	mr := exectest.NewMockRunner()
	mr.QueueResponse("uat/7.4.1+158404\nuat/7.4.0+155391\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}+{build}",
		},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto"},
		},
	}
	got, err := app.CurrentTag(mr, cfg, "uat", false)
	require.NoError(t, err)
	assert.Equal(t, "uat/7.4.1+158404", got)
	assert.Equal(t, "uat/*+*", mr.Calls[0].Args[2])
}

func TestCurrentTag_PerEnvMissingEnvArg(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver-per-env"},
	}
	_, err := app.CurrentTag(mr, cfg, "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--env")
}

func TestCurrentTag_NoTags(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}
	_, err := app.CurrentTag(mr, cfg, "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tags")
}

func TestCurrentTag_GitError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", errors.New("not a git repo"))

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}
	_, err := app.CurrentTag(mr, cfg, "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing git tags")
}

func TestCurrentTag_UnknownStrategy(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "unknown"},
	}
	_, err := app.CurrentTag(mr, cfg, "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestCurrentVersion_SemverStripsPrefix(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.2.3\nv1.2.2\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}
	got, err := app.CurrentVersion(mr, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", got)
}

func TestCurrentVersion_SemverDefaultPrefix(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v9.0.0\n", "", nil)

	// No TagPrefix set → semver defaults to "v".
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver"},
	}
	got, err := app.CurrentVersion(mr, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "9.0.0", got)
}

func TestCurrentVersion_CalverStripsPrefix(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("rel-2026.05.1\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "calver", TagPrefix: strPtr("rel-")},
	}
	got, err := app.CurrentVersion(mr, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "2026.05.1", got)
}

func TestCurrentVersion_PerEnvBuildFormat(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("main/7.4.1+158404\nmain/7.4.0+155398\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}+{build}",
		},
		Environments: map[string]config.Environment{
			"main": {Bump: "auto"},
		},
	}
	got, err := app.CurrentVersion(mr, cfg, "main", false)
	require.NoError(t, err)
	assert.Equal(t, "7.4.1", got)
}

func TestCurrentVersion_PropagatesCurrentTagError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // no tags

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}
	_, err := app.CurrentVersion(mr, cfg, "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tags")
}

func TestCurrentVersion_UnknownStrategy(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("x\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "unknown"},
	}
	_, err := app.CurrentVersion(mr, cfg, "", false)
	require.Error(t, err)
}

func TestCurrentTag_Semver_SkipsPreReleaseByDefault(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0-rc.1\nv1.3.0\n", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

	got, err := app.CurrentTag(mr, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "v1.3.0", got)
}

func TestCurrentTag_Semver_IncludePreRelease(t *testing.T) {
	tests := []struct {
		name, tags, want string
	}{
		{"pre-release above older final", "v1.4.0-rc.1\nv1.3.0\n", "v1.4.0-rc.1"},
		{"final above its own pre-release", "v1.4.0-rc.1\nv1.4.0\n", "v1.4.0"},
		{"numeric identifiers by value", "v1.4.0-rc.2\nv1.4.0-rc.10\n", "v1.4.0-rc.10"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.tags, "", nil)
			cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

			got, err := app.CurrentTag(mr, cfg, "", true)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCurrentTag_SemverPerEnv_IncludePreRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("prod/1.4.0-rc.2\nprod/1.4.0-rc.10\nprod/1.3.0\n", "", nil)
	cfg := &config.Config{
		Versioning:   config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{"prod": {TagFormat: "prod/{version}"}},
	}

	got, err := app.CurrentTag(mr, cfg, "prod", true)
	require.NoError(t, err)
	assert.Equal(t, "prod/1.4.0-rc.10", got)
}

func TestCurrentTag_Semver_OnlyPreReleases_HintsAtFlag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0-rc.1\n", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

	_, err := app.CurrentTag(mr, cfg, "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--include-pre-release")
}

func TestCurrentTag_Calver_IgnoresIncludePreRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.05.1\n", "", nil)
	empty := ""
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", TagPrefix: &empty}}

	got, err := app.CurrentTag(mr, cfg, "", true)
	require.NoError(t, err)
	assert.Equal(t, "2026.05.1", got)
}

// FIX-2: --bare must strip build metadata for plain semver too, matching per-env's behaviour and
// the flag help's promise ("strip prefix/env/build").
func TestCurrentVersion_Semver_StripsBuildMetadata(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0+158404\n", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

	got, err := app.CurrentVersion(mr, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "1.4.0", got)
}

func TestCurrentVersion_Semver_IncludePreRelease_StripsBuildMetadata(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0-rc.1+5\n", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

	got, err := app.CurrentVersion(mr, cfg, "", true)
	require.NoError(t, err)
	assert.Equal(t, "1.4.0-rc.1", got)
}

func TestCurrentVersion_Semver_CustomPrefix_StripsBuildMetadata(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("rel-1.4.0+158404\n", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("rel-")}}

	got, err := app.CurrentVersion(mr, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "1.4.0", got)
}

func TestCurrentVersion_Semver_IncludePreRelease_Bare(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0-rc.2\nv1.3.0\n", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

	got, err := app.CurrentVersion(mr, cfg, "", true)
	require.NoError(t, err)
	assert.Equal(t, "1.4.0-rc.2", got)
}
