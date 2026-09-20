package app_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stayAtV0SemverCfg() *config.Config {
	return &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: "semver",
			Bump:     &config.BumpConfig{StayAtV0: true},
		},
	}
}

func stayAtV0PerEnvCfg() *config.Config {
	return &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
			Bump:     &config.BumpConfig{StayAtV0: true},
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", Source: "dev", TagFormat: "prod/{version}"},
		},
	}
}

func TestNewResolver_Semver_StayAtV0_HoldsBackAndWarns(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(stayAtV0SemverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v0.69.0", res.Tag)
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "1.0.0 → 0.69.0")
	assert.Contains(t, res.Warnings[0], "feat!: break the api")
}

func TestNewResolver_Semver_WithAllowMajor_ReleasesMajorWithoutWarning(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(stayAtV0SemverCfg(), "", false, "", "", mr, app.WithAllowMajor(true))
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", res.Tag)
	assert.Empty(t, res.Warnings)
}

func TestNewResolver_Semver_WithoutStayAtV0_NoWarning(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(semverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", res.Tag)
	assert.Empty(t, res.Warnings)
}

func TestNewResolver_SetVersion_AllowMajorIsNoOp(t *testing.T) {
	r, err := app.NewResolver(stayAtV0SemverCfg(), "", false, "v1.0.0", "", exectest.NewMockRunner(), app.WithAllowMajor(true))
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", res.Tag)
	assert.Empty(t, res.Warnings)
}

func TestNewResolver_SemverPerEnv_AutoEnvHoldsBackAndWarns(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(stayAtV0PerEnvCfg(), "dev", false, "", "", mr)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "dev/0.69.0", res.Tag)
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "1.0.0 → 0.69.0")
}

func TestNewResolver_SemverPerEnv_AllowMajor(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(stayAtV0PerEnvCfg(), "dev", false, "", "", mr, app.WithAllowMajor(true))
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "dev/1.0.0", res.Tag)
	assert.Empty(t, res.Warnings)
}

func TestNewResolver_SemverPerEnv_PromoteEnvHasNoWarnings(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/0.68.0\n", "", nil) // git tag -l dev/*        → source env tags
	mr.QueueResponse("", "", nil)             // git tag -l prod/0.68.0  → candidate does not exist yet
	mr.QueueResponse("", "", nil)             // git tag -l prod/*       → no prod tags yet

	r, err := app.NewResolver(stayAtV0PerEnvCfg(), "prod", false, "", "", mr)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "prod/0.68.0", res.Tag)
	assert.Empty(t, res.Warnings)
}
