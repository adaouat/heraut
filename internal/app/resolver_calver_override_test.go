package app_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
)

func calverOverrideCfg(strategy string) (*config.Config, string) {
	cfg := &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: strategy, Format: "YYYY.MM.PATCH"},
	}
	if strategy == "calver-per-env" {
		cfg.Environments = map[string]config.Environment{"uat": {Bump: "auto", TagFormat: "{env}/{version}"}}
		return cfg, "uat"
	}
	return cfg, ""
}

// ADR-0074 addendum: a CalVer --set-version must be the canonical form of versioning.format,
// because it ends up in the tag and in the hook variables.
func TestNewResolver_CalverVersionOverride_MustBeCanonicalForTheFormat(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr string
		wantTag map[string]string // by strategy
	}{
		{"canonical", "2026.05.3", "", map[string]string{"calver": "2026.05.3", "calver-per-env": "uat/2026.05.3"}},
		{"patch zero", "2026.05.0", "", map[string]string{"calver": "2026.05.0", "calver-per-env": "uat/2026.05.0"}},
		{"leading v is stripped for a tag_format", "v2026.05.3", "", map[string]string{"calver-per-env": "uat/2026.05.3"}},
		{"unpadded month", "2026.5.3", "canonical", nil},
		{"padded patch", "2026.05.03", "canonical", nil},
		{"shell syntax", "x$(id)", "does not match", nil},
		{"command separator", "2026.05.3;id", "does not match", nil},
		{"missing patch", "2026.05", "does not match", nil},
		{"extra segment", "2026.05.3.1", "does not match", nil},
		{"semver-looking", "1.2.3", "does not match", nil},
	}
	for _, strategy := range []string{"calver", "calver-per-env"} {
		for _, tc := range tests {
			t.Run(strategy+"/"+tc.name, func(t *testing.T) {
				cfg, env := calverOverrideCfg(strategy)
				mr := exectest.NewMockRunner()
				r, err := app.NewResolver(cfg, env, false, tc.value, "", mr, app.WithoutCollisionGuard())
				if tc.wantErr == "" {
					want, ok := tc.wantTag[strategy]
					if !ok {
						t.Skip("not applicable to this strategy")
					}
					require.NoError(t, err)
					res, err := r.Resolve()
					require.NoError(t, err)
					assert.Equal(t, want, res.Tag)
					return
				}
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.ErrorContains(t, err, "YYYY.MM.PATCH", "the error names the configured format")
				assert.ErrorContains(t, err, "--set-version")
			})
		}
	}
}

func TestNewResolver_CalverVersionOverride_HonorsTagPrefix(t *testing.T) {
	cfg, _ := calverOverrideCfg("calver")
	prefix := "rel-"
	cfg.Versioning.TagPrefix = &prefix

	for _, value := range []string{"2026.05.3", "rel-2026.05.3"} {
		r, err := app.NewResolver(cfg, "", false, value, "", exectest.NewMockRunner(), app.WithoutCollisionGuard())
		require.NoError(t, err, value)
		res, err := r.Resolve()
		require.NoError(t, err)
		assert.Equal(t, "rel-2026.05.3", res.Tag)
	}

	_, err := app.NewResolver(cfg, "", false, "rel-x$(id)", "", exectest.NewMockRunner(), app.WithoutCollisionGuard())
	require.Error(t, err)
}

func TestNewResolver_CalverPerEnvVersionOverride_CheckedWithABuildID(t *testing.T) {
	cfg, env := calverOverrideCfg("calver-per-env")
	cfg.Environments["uat"] = config.Environment{Bump: "auto", TagFormat: "{env}/{version}+{build}"}

	_, err := app.NewResolver(cfg, env, false, "x$(id)", "build_1", exectest.NewMockRunner(), app.WithoutCollisionGuard())
	require.Error(t, err)
	assert.ErrorContains(t, err, "--set-version")
}
