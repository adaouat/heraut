package app_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func semverCfg() *config.Config {
	return &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
	}
}

func calverCfg() *config.Config {
	return &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "calver", Format: "YYYY.MM.PATCH"},
	}
}

func TestNewResolver_Semver(t *testing.T) {
	mr := testutil.NewMockRunner()
	r, err := app.NewResolver(semverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_Calver(t *testing.T) {
	mr := testutil.NewMockRunner()
	r, err := app.NewResolver(calverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_SemverPerEnv(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
		},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", TagFormat: "prod/${version}"},
		},
	}
	r, err := app.NewResolver(cfg, "prod", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_CalverPerEnv(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: "calver-per-env",
			Format:   "YYYY.MM.PATCH",
		},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", TagFormat: "prod/${version}"},
		},
	}
	r, err := app.NewResolver(cfg, "prod", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_UnknownStrategy(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "unknown-strategy"},
	}
	_, err := app.NewResolver(cfg, "", false, "", "", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-strategy")
}

func TestNewResolver_VersionOverride_Semver(t *testing.T) {
	mr := testutil.NewMockRunner()
	r, err := app.NewResolver(semverCfg(), "", false, "v2.0.0", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", result.Tag)
	assert.Equal(t, "2.0.0", result.Version)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_VersionOverride_Calver(t *testing.T) {
	mr := testutil.NewMockRunner()
	r, err := app.NewResolver(calverCfg(), "", false, "2026.05.3", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "2026.05.3", result.Tag)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_VersionOverride_SemverPerEnv(t *testing.T) {
	mr := testutil.NewMockRunner()
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"prod": {Bump: "auto", TagFormat: "prod/${version}"},
		},
	}
	r, err := app.NewResolver(cfg, "prod", false, "v1.5.0", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v1.5.0", result.Tag)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}
