package app_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
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
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(semverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_Calver(t *testing.T) {
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(calverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewResolver_SemverPerEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
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
	mr := exectest.NewMockRunner()
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
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "unknown-strategy"},
	}
	_, err := app.NewResolver(cfg, "", false, "", "", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-strategy")
}

func TestNewResolver_VersionOverride_Semver(t *testing.T) {
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(semverCfg(), "", false, "v2.0.0", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", result.Tag)
	assert.Equal(t, "2.0.0", result.Version)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_VersionOverride_Calver(t *testing.T) {
	mr := exectest.NewMockRunner()
	r, err := app.NewResolver(calverCfg(), "", false, "2026.05.3", "", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "2026.05.3", result.Tag)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_VersionOverride_SemverPerEnv(t *testing.T) {
	mr := exectest.NewMockRunner()
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

func TestNewResolver_BuildID_RendersTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}-{build}",
		},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto"},
		},
	}
	r, err := app.NewResolver(cfg, "uat", false, "7.4.1", "158404", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "uat/7.4.1-158404", result.Tag)
	assert.Equal(t, "7.4.1", result.Version)
	assert.Empty(t, mr.Calls, "static resolver must not call git")
}

func TestNewResolver_BuildID_UsesEnvTagFormatOverride(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}",
		},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto", TagFormat: "{env}/{version}+{build}"},
		},
	}
	r, err := app.NewResolver(cfg, "uat", false, "7.4.1", "99", mr)
	require.NoError(t, err)

	result, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "uat/7.4.1+99", result.Tag)
}

func TestNewResolver_BuildID_RequiresVersion(t *testing.T) {
	mr := exectest.NewMockRunner()
	_, err := app.NewResolver(semverCfg(), "", false, "", "158404", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--build requires --version")
}

func TestNewResolver_BuildID_NoBuildToken(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy:  "semver-per-env",
			TagFormat: "{env}/{version}",
		},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto"},
		},
	}
	_, err := app.NewResolver(cfg, "uat", false, "7.4.1", "158404", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{build}")
}

func TestNewResolver_BuildID_NoTagFormat(t *testing.T) {
	mr := exectest.NewMockRunner()
	_, err := app.NewResolver(semverCfg(), "", false, "7.4.1", "158404", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag_format")
}

func TestValidateBuildID(t *testing.T) {
	require.NoError(t, app.ValidateBuildID("158404"))
	require.Error(t, app.ValidateBuildID("bad/value"))
	require.Error(t, app.ValidateBuildID(""))
}

func TestValidateVersionOverride(t *testing.T) {
	require.NoError(t, app.ValidateVersionOverride("v1.2.3"))
	require.NoError(t, app.ValidateVersionOverride("2024.03.15.2"))
	require.Error(t, app.ValidateVersionOverride(""))
	require.Error(t, app.ValidateVersionOverride("1.2.3 "))
}
