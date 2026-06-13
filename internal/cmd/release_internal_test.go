package cmd

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestHasEffectivePlatforms(t *testing.T) {
	githubPlatform := config.Platform{Name: "github", Type: "github", Repository: "org/repo"}
	gitlabPlatform := config.Platform{Name: "gitlab", Type: "gitlab", Project: "grp/repo"}

	tests := []struct {
		name string
		cfg  *config.Config
		env  string
		want bool
	}{
		{
			name: "root platforms, no env",
			cfg:  &config.Config{Release: &config.Release{Platforms: []config.Platform{githubPlatform}}},
			want: true,
		},
		{
			name: "no release block anywhere",
			cfg:  &config.Config{},
			want: false,
		},
		{
			name: "env not present in environments falls back to root",
			cfg: &config.Config{
				Release: &config.Release{Platforms: []config.Platform{githubPlatform}},
			},
			env:  "uat",
			want: true,
		},
		{
			name: "env has no release override, inherits root",
			cfg: &config.Config{
				Release: &config.Release{Platforms: []config.Platform{githubPlatform}},
				Environments: map[string]config.Environment{
					"uat": {Bump: "auto"},
				},
			},
			env:  "uat",
			want: true,
		},
		{
			name: "env release overrides notes only, inherits root platforms",
			cfg: &config.Config{
				Release: &config.Release{Platforms: []config.Platform{githubPlatform}},
				Environments: map[string]config.Environment{
					"uat": {Release: &config.EnvRelease{Notes: &config.ContentDriver{Generator: "git-cliff"}}},
				},
			},
			env:  "uat",
			want: true,
		},
		{
			name: "env release sets an empty platforms list, inherits root",
			cfg: &config.Config{
				Release: &config.Release{Platforms: []config.Platform{githubPlatform}},
				Environments: map[string]config.Environment{
					"uat": {Release: &config.EnvRelease{Platforms: []config.Platform{}}},
				},
			},
			env:  "uat",
			want: true,
		},
		{
			name: "env platforms override root platforms",
			cfg: &config.Config{
				Release: &config.Release{Platforms: []config.Platform{githubPlatform}},
				Environments: map[string]config.Environment{
					"uat": {Release: &config.EnvRelease{Platforms: []config.Platform{gitlabPlatform}}},
				},
			},
			env:  "uat",
			want: true,
		},
		{
			name: "no root platforms but env provides its own",
			cfg: &config.Config{
				Environments: map[string]config.Environment{
					"uat": {Release: &config.EnvRelease{Platforms: []config.Platform{gitlabPlatform}}},
				},
			},
			env:  "uat",
			want: true,
		},
		{
			name: "no root platforms and env overrides notes only",
			cfg: &config.Config{
				Environments: map[string]config.Environment{
					"uat": {Release: &config.EnvRelease{Notes: &config.ContentDriver{Generator: "git-cliff"}}},
				},
			},
			env:  "uat",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hasEffectivePlatforms(tc.cfg, tc.env))
		})
	}
}
