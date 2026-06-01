package config_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestConfig_EffectiveTagFormat(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		env  string
		want string
	}{
		{
			name: "env override wins over top-level",
			cfg: &config.Config{
				Versioning: config.Versioning{TagFormat: "{env}/{version}"},
				Environments: map[string]config.Environment{
					"prod": {TagFormat: "release/{version}"},
				},
			},
			env:  "prod",
			want: "release/{version}",
		},
		{
			name: "falls back to top-level when env override empty",
			cfg: &config.Config{
				Versioning: config.Versioning{TagFormat: "{env}/{version}-{build}"},
				Environments: map[string]config.Environment{
					"uat": {Bump: "auto"},
				},
			},
			env:  "uat",
			want: "{env}/{version}-{build}",
		},
		{
			name: "falls back to top-level when env not in map",
			cfg: &config.Config{
				Versioning: config.Versioning{TagFormat: "{env}/{version}"},
			},
			env:  "missing",
			want: "{env}/{version}",
		},
		{
			name: "empty env returns top-level",
			cfg: &config.Config{
				Versioning: config.Versioning{TagFormat: "v{version}"},
			},
			env:  "",
			want: "v{version}",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cfg.EffectiveTagFormat(tc.env))
		})
	}
}
