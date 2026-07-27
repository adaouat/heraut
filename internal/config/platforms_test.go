package config_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestEffectivePlatforms(t *testing.T) {
	root := []config.Platform{{Name: "root", Type: "github"}}
	envPlatforms := []config.Platform{{Name: "env", Type: "gitlab"}}

	t.Run("nil config returns nil", func(t *testing.T) {
		assert.Nil(t, config.EffectivePlatforms(nil, ""))
	})

	t.Run("nil release returns nil", func(t *testing.T) {
		cfg := &config.Config{}
		assert.Nil(t, config.EffectivePlatforms(cfg, ""))
	})

	t.Run("no env returns root platforms", func(t *testing.T) {
		cfg := &config.Config{Release: &config.Release{Platforms: root}}
		assert.Equal(t, root, config.EffectivePlatforms(cfg, ""))
	})

	t.Run("unknown env inherits root", func(t *testing.T) {
		cfg := &config.Config{Release: &config.Release{Platforms: root}}
		assert.Equal(t, root, config.EffectivePlatforms(cfg, "staging"))
	})

	t.Run("env without release override inherits root", func(t *testing.T) {
		cfg := &config.Config{
			Release:      &config.Release{Platforms: root},
			Environments: map[string]config.Environment{"staging": {Bump: "auto"}},
		}
		assert.Equal(t, root, config.EffectivePlatforms(cfg, "staging"))
	})

	t.Run("env release with empty platforms inherits root", func(t *testing.T) {
		cfg := &config.Config{
			Release: &config.Release{Platforms: root},
			Environments: map[string]config.Environment{
				"staging": {Bump: "auto", Release: &config.EnvRelease{}},
			},
		}
		assert.Equal(t, root, config.EffectivePlatforms(cfg, "staging"))
	})

	t.Run("env release with platforms replaces root", func(t *testing.T) {
		cfg := &config.Config{
			Release: &config.Release{Platforms: root},
			Environments: map[string]config.Environment{
				"prod": {Bump: "auto", Release: &config.EnvRelease{Platforms: envPlatforms}},
			},
		}
		assert.Equal(t, envPlatforms, config.EffectivePlatforms(cfg, "prod"))
	})

	t.Run("env platforms apply even with nil root release", func(t *testing.T) {
		cfg := &config.Config{
			Environments: map[string]config.Environment{
				"prod": {Bump: "auto", Release: &config.EnvRelease{Platforms: envPlatforms}},
			},
		}
		assert.Equal(t, envPlatforms, config.EffectivePlatforms(cfg, "prod"))
	})
}

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
