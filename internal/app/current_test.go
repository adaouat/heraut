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
		Versioning: config.Versioning{Strategy: "semver", Prefix: strPtr("v")},
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
		Versioning: config.Versioning{Strategy: "calver", Prefix: &empty},
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
		Versioning: config.Versioning{Strategy: "semver", Prefix: strPtr("v")},
	}
	_, err := app.CurrentTag(mr, cfg, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tags")
}

func TestCurrentTag_GitError(t *testing.T) {
	mr := testutil.NewMockRunner()
	mr.QueueResponse("", "", errors.New("not a git repo"))

	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", Prefix: strPtr("v")},
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
