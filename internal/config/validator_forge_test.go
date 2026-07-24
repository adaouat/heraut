package config_test

import (
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func cfgWithForges(forges []config.Forge, enrichForge string) *config.Config {
	return &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver"},
		Changelog:  &config.ContentDriver{Generator: "native", Output: "CHANGELOG.md"},
		Commits:    &config.Commits{EnrichmentForge: enrichForge, EnrichmentPolicy: "optional"},
		Forges:     forges,
	}
}

// forgeErr reports whether some validation error's Path or Message contains want.
func forgeErr(errs config.ValidationErrors, want string) bool {
	for _, e := range errs {
		if strings.Contains(e.Path, want) || strings.Contains(e.Message, want) {
			return true
		}
	}
	return false
}

func TestValidate_Forges(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string // substring in some error Path/Message; "" = no forge error
	}{
		{"single forge, no enrichment_forge ok",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab"}}, ""), ""},
		{"unknown platform",
			cfgWithForges([]config.Forge{{Name: "A", Type: "bitbucket"}}, ""), "platform"},
		{"duplicate name",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab"}, {Name: "A", Type: "github"}}, "A"), "duplicate"},
		{"multi forge requires enrichment_forge",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab"}, {Name: "B", Type: "github"}}, ""), "enrichment_forge"},
		{"enrichment_forge names unknown forge",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab"}}, "Z"), "unknown"},
		{"bad api_mode",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab", APIMode: "grpc"}}, ""), "api_mode"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := config.Validate(tc.cfg)
			if tc.want == "" {
				// Robust to unrelated config incompleteness: assert only that no
				// forge/enrichment error was produced (not that the whole config is valid).
				assert.False(t, forgeErr(errs, "forges"), "unexpected forge error: %v", errs)
				assert.False(t, forgeErr(errs, "enrichment_forge"), "unexpected error: %v", errs)
				return
			}
			assert.True(t, forgeErr(errs, tc.want), "expected an error mentioning %q, got %v", tc.want, errs)
		})
	}
}
