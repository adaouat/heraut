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

func TestEffectiveReleaseNotes(t *testing.T) {
	base := &config.ContentDriver{Output: "NOTES.md"}
	envNotes := &config.ContentDriver{Template: "custom.tmpl"}

	tests := []struct {
		name string
		cfg  *config.Config
		env  string
		want *config.ContentDriver
	}{
		{name: "nil config", cfg: nil, env: "", want: nil},
		{name: "no release block", cfg: &config.Config{}, env: "", want: nil},
		{
			name: "top-level only",
			cfg:  &config.Config{Release: &config.Release{Notes: base}},
			env:  "", want: base,
		},
		{
			name: "env with no notes override inherits top-level",
			cfg: &config.Config{
				Release:      &config.Release{Notes: base},
				Environments: map[string]config.Environment{"staging": {}},
			},
			env: "staging", want: base,
		},
		{
			name: "env override merges onto top-level",
			cfg: &config.Config{
				Release: &config.Release{Notes: base},
				Environments: map[string]config.Environment{
					"staging": {Release: &config.EnvRelease{Notes: envNotes}},
				},
			},
			env:  "staging",
			want: &config.ContentDriver{Output: "NOTES.md", Template: "custom.tmpl"},
		},
		{
			name: "env-only notes with no top-level notes",
			cfg: &config.Config{
				Environments: map[string]config.Environment{
					"staging": {Release: &config.EnvRelease{Notes: envNotes}},
				},
			},
			env: "staging", want: envNotes,
		},
		{
			name: "unknown env keeps top-level",
			cfg:  &config.Config{Release: &config.Release{Notes: base}},
			env:  "nope", want: base,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, config.EffectiveReleaseNotes(tc.cfg, tc.env))
		})
	}
}
