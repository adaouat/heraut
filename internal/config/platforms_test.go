package config_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestEffectiveTargets(t *testing.T) {
	base := []config.Target{{Forge: "Primary", Draft: true}}
	envOverride := []config.Target{{Forge: "Mirror"}}

	tests := []struct {
		name string
		cfg  *config.Config
		env  string
		want []config.Target
	}{
		{name: "nil config", cfg: nil, env: "", want: nil},
		{
			name: "top-level only",
			cfg:  &config.Config{Release: &config.Release{Targets: base}},
			env:  "", want: base,
		},
		{
			name: "env with no override keeps top-level",
			cfg: &config.Config{
				Release:      &config.Release{Targets: base},
				Environments: map[string]config.Environment{"staging": {}},
			},
			env: "staging", want: base,
		},
		{
			name: "env override replaces (does not merge)",
			cfg: &config.Config{
				Release: &config.Release{Targets: base},
				Environments: map[string]config.Environment{
					"staging": {Release: &config.EnvRelease{Targets: envOverride}},
				},
			},
			env: "staging", want: envOverride,
		},
		{
			name: "empty env override keeps top-level",
			cfg: &config.Config{
				Release: &config.Release{Targets: base},
				Environments: map[string]config.Environment{
					"staging": {Release: &config.EnvRelease{Targets: []config.Target{}}},
				},
			},
			env: "staging", want: base,
		},
		{
			name: "unknown env keeps top-level",
			cfg:  &config.Config{Release: &config.Release{Targets: base}},
			env:  "nope", want: base,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, config.EffectiveTargets(tc.cfg, tc.env))
		})
	}
}
