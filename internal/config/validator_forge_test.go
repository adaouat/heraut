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
		{"base_url missing scheme",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab", BaseURL: "gitlab.example.com"}}, ""), "base_url"},
		{"base_url malformed",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab", BaseURL: "not a url"}}, ""), "base_url"},
		{"base_url well-formed absolute URL ok",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab", BaseURL: "https://gitlab.example.com"}}, ""), ""},
		{"api_url missing scheme",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab", APIURL: "gitlab.example.com/api"}}, ""), "api_url"},
		{"api_url well-formed absolute URL ok",
			cfgWithForges([]config.Forge{{Name: "A", Type: "gitlab", APIURL: "https://gitlab.example.com/api"}}, ""), ""},
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

// TestValidate_EnvironmentReleaseTargets_Forge pins I4: environments.<env>.release.targets was
// never validated (only the top-level release.targets was), so a per-env target naming an
// unknown forge — or omitting forge with more than one forge configured — passed
// `heraut check config` and failed later inside BuildPipeline, pointing at the wrong path
// (release.targets[0] instead of environments.<env>.release.targets[0].forge).
func TestValidate_EnvironmentReleaseTargets_Forge(t *testing.T) {
	baseCfg := func(envRelease *config.EnvRelease, forges []config.Forge) *config.Config {
		return &config.Config{
			Version:    "1",
			Versioning: config.Versioning{Strategy: "semver-per-env"},
			Environments: map[string]config.Environment{
				"prod": {
					Bump:      "auto",
					TagFormat: "prod/{version}",
					Release:   envRelease,
				},
			},
			Forges: forges,
		}
	}

	t.Run("unknown forge name in a per-env target is rejected", func(t *testing.T) {
		cfg := baseCfg(
			&config.EnvRelease{Targets: []config.Target{{Forge: "does-not-exist"}}},
			[]config.Forge{{Name: "gh", Type: "github"}},
		)
		errs := config.Validate(cfg)
		e := findErr(errs, "environments.prod.release.targets[0].forge")
		if assert.NotNil(t, e, "expected an error on environments.prod.release.targets[0].forge, got %v", errs) {
			assert.Contains(t, e.Message, "unknown forge")
		}
	})

	t.Run("empty forge with more than one forge configured is ambiguous", func(t *testing.T) {
		cfg := baseCfg(
			&config.EnvRelease{Targets: []config.Target{{}}},
			[]config.Forge{{Name: "gh", Type: "github"}, {Name: "gl", Type: "gitlab"}},
		)
		errs := config.Validate(cfg)
		e := findErr(errs, "environments.prod.release.targets[0].forge")
		if assert.NotNil(t, e, "expected an error on environments.prod.release.targets[0].forge, got %v", errs) {
			assert.Contains(t, e.Message, "required when more than one forge is configured")
		}
	})

	t.Run("known forge name in a per-env target is accepted", func(t *testing.T) {
		cfg := baseCfg(
			&config.EnvRelease{Targets: []config.Target{{Forge: "gh"}}},
			[]config.Forge{{Name: "gh", Type: "github"}},
		)
		errs := config.Validate(cfg)
		assert.False(t, forgeErr(errs, "environments.prod.release.targets"), "unexpected error: %v", errs)
	})
}

// TestValidate_UnsatisfiableTargets pins T171: no two targets may resolve to the same forge.
// Two bare targets with zero or one forges[] entry collapse to the same auto-detected identity,
// and two targets naming the same forge explicitly collapse likewise — both let the first
// `release create` succeed and the second fail after the tag has already been pushed.
func TestValidate_UnsatisfiableTargets(t *testing.T) {
	tests := []struct {
		name    string
		forges  []config.Forge
		targets []config.Target
		want    string // substring expected in some error; "" = valid
	}{
		{
			name:    "zero-config single bare target is fine",
			forges:  nil,
			targets: []config.Target{{}},
			want:    "",
		},
		{
			name:    "zero-config with two bare targets is unsatisfiable",
			forges:  nil,
			targets: []config.Target{{}, {Draft: true}},
			want:    "release.targets",
		},
		{
			name:    "one forge with two bare targets is unsatisfiable",
			forges:  []config.Forge{{Name: "A", Type: "gitlab"}},
			targets: []config.Target{{}, {Draft: true}},
			want:    "release.targets",
		},
		{
			name:    "two forges, each target names one",
			forges:  []config.Forge{{Name: "A", Type: "gitlab"}, {Name: "B", Type: "github"}},
			targets: []config.Target{{Forge: "A"}, {Forge: "B"}},
			want:    "",
		},
		{
			name:    "two targets naming the SAME forge is unsatisfiable",
			forges:  []config.Forge{{Name: "A", Type: "gitlab"}, {Name: "B", Type: "github"}},
			targets: []config.Target{{Forge: "A"}, {Forge: "A", Draft: true}},
			want:    "release.targets",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Version:    "1",
				Versioning: config.Versioning{Strategy: "semver"},
				Forges:     tc.forges,
				Release:    &config.Release{Targets: tc.targets},
			}
			errs := config.Validate(cfg)
			if tc.want == "" {
				for _, e := range errs {
					assert.NotContains(t, e.Path, "release.targets", "unexpected target error: %v", e)
				}
				return
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e.Path, tc.want) || strings.Contains(e.Message, tc.want) {
					found = true
				}
			}
			assert.True(t, found, "expected an error mentioning %q, got %v", tc.want, errs)
		})
	}
}

// TestValidate_platformBaseURLMalformed pins the C1 regression: forges[].base_url with no
// scheme (the likeliest typo, e.g. "gitlab.example.com") must fail validation with a clear
// "not a valid URL"-shaped message rather than silently reaching platformConfigFromTarget,
// where it would produce a driver with no GITLAB_HOST/GH_HOST override and publish to the
// wrong host (see the deleted validatePlatformBaseURL / TestValidate_platformBaseURLMalformed
// from before the forges migration).
func TestValidate_platformBaseURLMalformed(t *testing.T) {
	cfg := cfgWithForges([]config.Forge{{Name: "gitlab-internal", Type: "gitlab", BaseURL: "gitlab.example.com"}}, "")
	errs := config.Validate(cfg)
	e := findErr(errs, "forges[0].base_url")
	if assert.NotNil(t, e, "expected a validation error on forges[0].base_url, got %v", errs) {
		assert.Contains(t, e.Message, "not a valid URL")
	}
}
