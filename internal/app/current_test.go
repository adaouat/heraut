package app_test

import (
	"errors"
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func TestCurrentTag_Semver(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("v1.2.3\nv1.2.2\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}
	got, err := app.CurrentTag(mr, cfg, "")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", got)

	// Verify glob arg
	assert.Equal(t, []string{"tag", "-l", "v*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestCurrentTag_Calver(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("2026.05.1\n", "", nil)

	empty := ""
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "calver", TagPrefix: &empty},
	}
	got, err := app.CurrentTag(mr, cfg, "")
	require.NoError(t, err)
	assert.Equal(t, "2026.05.1", got)
}

func TestCurrentTag_SemverPerEnv(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("prod/1.2.3\nprod/1.2.2\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"prod": {TagFormat: "prod/${version}"},
		},
	}
	got, err := app.CurrentTag(mr, cfg, "prod")
	require.NoError(t, err)
	assert.Equal(t, "prod/1.2.3", got)

	// Verify glob contains prod pattern
	assert.Contains(t, mr.Calls[0].Args[2], "prod")
}

func TestCurrentTag_PerEnvCommonTagFormat(t *testing.T) {
	// Bug T54: with a top-level tag_format and no per-env override, the glob
	// must still resolve. Previously currentTagGlob read envCfg.TagFormat directly,
	// which was empty here, producing "tag format must contain {version}".
	mr := testutil.NewMockRunner()
	mr.QueueResponse("uat/7.4.1-158404\nuat/7.4.0-155391\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}-{build}",
		},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto"},
		},
	}
	got, err := app.CurrentTag(mr, cfg, "uat")
	require.NoError(t, err)
	assert.Equal(t, "uat/7.4.1-158404", got)
	assert.Equal(t, "uat/*-*", mr.Calls[0].Args[2])
}

func TestCurrentTag_PerEnvMissingEnvArg(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver-per-env"},
	}
	_, err := app.CurrentTag(mr, cfg, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--env")
}

func TestCurrentTag_NoTags(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}
	_, err := app.CurrentTag(mr, cfg, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tags")
}

func TestCurrentTag_GitError(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", errors.New("not a git repo"))

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")},
	}
	_, err := app.CurrentTag(mr, cfg, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing git tags")
}

func TestCurrentTag_UnknownStrategy(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "unknown"},
	}
	_, err := app.CurrentTag(mr, cfg, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}
