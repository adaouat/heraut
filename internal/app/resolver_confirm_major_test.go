package app_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
)

func confirmMajorCfg() *config.Config {
	cfg := semverCfg()
	cfg.Versioning.Bump = &config.BumpConfig{ConfirmMajor: true}
	return cfg
}

func confirmMajorPerEnvCfg() *config.Config {
	cfg := stayAtV0PerEnvCfg()
	cfg.Versioning.Bump = &config.BumpConfig{ConfirmMajor: true}
	return cfg
}

// ADR-0078: the result carries the commits that forced a major still to be confirmed, for every
// strategy that bumps from commits.
func TestNewResolver_ConfirmMajor_MarksTheResult(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		env     string
		tags    string
		opts    []app.ResolverOption
		wantTag string
		want    []string
	}{
		{"semver", confirmMajorCfg(), "", "v1.4.0\n", nil, "v2.0.0", []string{"feat!: break the api"}},
		{"semver with --allow-major", confirmMajorCfg(), "", "v1.4.0\n", []app.ResolverOption{app.WithAllowMajor(true)}, "v2.0.0", nil},
		{"semver-per-env auto environment", confirmMajorPerEnvCfg(), "dev", "dev/1.4.0\n", nil, "dev/2.0.0", []string{"feat!: break the api"}},
		{"semver-per-env with --allow-major", confirmMajorPerEnvCfg(), "dev", "dev/1.4.0\n", []app.ResolverOption{app.WithAllowMajor(true)}, "dev/2.0.0", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.tags, "", nil)
			mr.QueueResponse("feat!: break the api\x00", "", nil)

			r, err := app.NewResolver(tc.cfg, tc.env, false, "", "", mr, tc.opts...)
			require.NoError(t, err)
			res, err := r.Resolve()
			require.NoError(t, err)

			assert.Equal(t, tc.wantTag, res.Tag)
			assert.Equal(t, tc.want, res.UnconfirmedMajor)
		})
	}
}

// A promote environment copies a version approved where it came from: nothing to confirm.
func TestNewResolver_ConfirmMajor_PromoteEnvironmentIsNotMarked(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/2.0.0\n", "", nil)  // source env tags
	mr.QueueResponse("", "", nil)             // candidate prod/2.0.0 does not exist yet
	mr.QueueResponse("prod/1.4.0\n", "", nil) // prod tags

	r, err := app.NewResolver(confirmMajorPerEnvCfg(), "prod", false, "", "", mr)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "prod/2.0.0", res.Tag)
	assert.Empty(t, res.UnconfirmedMajor)
}

// --set-version is a version someone typed: nothing to confirm.
func TestNewResolver_ConfirmMajor_SetVersionIsNotMarked(t *testing.T) {
	r, err := app.NewResolver(confirmMajorCfg(), "", false, "v2.0.0", "", exectest.NewMockRunner(), app.WithoutCollisionGuard())
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v2.0.0", res.Tag)
	assert.Empty(t, res.UnconfirmedMajor)
}
