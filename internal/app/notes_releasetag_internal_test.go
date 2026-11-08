package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

// heraut notes accepts exactly the tags the strategy creates for the active environment: another
// environment's tag is refused, and a CalVer tag with a trailing literal is accepted.
func TestIsReleaseTag(t *testing.T) {
	perEnv := func(strategy, format string) *config.Config {
		return &config.Config{Versioning: config.Versioning{Strategy: strategy, Format: format, TagFormat: "{env}/{version}"},
			Environments: map[string]config.Environment{"dev": {Bump: "auto"}, "prod": {Bump: "auto"}}}
	}
	calver := func(format string) *config.Config {
		return &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: format}}
	}
	tests := []struct {
		name string
		cfg  *config.Config
		env  string
		tag  string
		want bool
	}{
		{"semver-per-env: its own tag", perEnv("semver-per-env", ""), "prod", "prod/0.1.0", true},
		{"semver-per-env: another environment's tag", perEnv("semver-per-env", ""), "prod", "dev/0.1.0", false},
		{"calver-per-env: another environment's tag", perEnv("calver-per-env", "YYYY.MM.PATCH"), "prod", "dev/2026.11.0", false},
		{"calver: a trailing literal", calver("YYYY.MM.PATCH-lts"), "", "2026.11.0-lts", true},
		{"calver: a four-part format", calver("YYYY.MM.DD.PATCH"), "", "2026.11.07.0", true},
		{"calver: not the format", calver("YYYY.MM.PATCH-lts"), "", "2026.11.0", false},
		{"calver: no version", calver("YYYY.MM.PATCH"), "", "latest", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isReleaseTag(tc.cfg, tc.env, tc.tag))
		})
	}
}
