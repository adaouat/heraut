package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestWithEnvDerivations_MergesTemplates(t *testing.T) {
	driver := &config.ContentDriver{
		Rendering: &config.Rendering{Templates: map[string]string{"commit": "driver-commit"}},
	}
	cfg := &config.Config{
		Rendering: &config.Rendering{Templates: map[string]string{"commit": "global-commit", "group": "global-group"}},
		Changelog: driver,
	}
	got := withEnvDerivations(driver, cfg, "")
	assert.Equal(t, "driver-commit", got.EffectiveTemplates["commit"], "driver overrides global")
	assert.Equal(t, "global-group", got.EffectiveTemplates["group"], "unset key falls through")
	assert.Nil(t, driver.EffectiveTemplates, "the original driver is never mutated")
}

func TestWithEnvDerivations_MergesTrailers(t *testing.T) {
	driver := &config.ContentDriver{
		Rendering: &config.Rendering{Commit: &config.RenderingCommit{
			Trailers: []config.FooterRule{{Token: "Co-authored-by", Renderer: "driver"}},
		}},
	}
	cfg := &config.Config{
		Rendering: &config.Rendering{Commit: &config.RenderingCommit{Trailers: []config.FooterRule{
			{Token: "co-authored-by", Renderer: "global"},
			{Token: "Refs", Hide: true},
		}}},
		Changelog: driver,
	}
	got := withEnvDerivations(driver, cfg, "")
	require.Equal(t, "driver", got.EffectiveTrailerRules["co-authored-by"].Renderer, "driver overrides global, keyed lowercase")
	require.True(t, got.EffectiveTrailerRules["refs"].Hide, "unset token falls through from global")
	assert.Nil(t, driver.EffectiveTrailerRules, "the original driver is never mutated")
}

// TestWithEnvDerivations_MergesTrailers_NilCommit covers ADR-0060: a Rendering block that sets
// other fields (or nothing at all) but no commit.trailers must not panic effectiveTrailers.
func TestWithEnvDerivations_MergesTrailers_NilCommit(t *testing.T) {
	driver := &config.ContentDriver{Rendering: &config.Rendering{}}
	cfg := &config.Config{Rendering: &config.Rendering{}, Changelog: driver}

	got := withEnvDerivations(driver, cfg, "")

	require.Equal(t, "_Co-Authored-By: {{ .Value }}_", got.EffectiveTrailerRules["co-authored-by"].Renderer,
		"the built-in default still applies when Rendering is set but Commit is nil")
}

func TestWithEnvDerivations_AppliesBuiltInCoAuthoredByDefault(t *testing.T) {
	driver := &config.ContentDriver{}
	cfg := &config.Config{Changelog: driver}

	got := withEnvDerivations(driver, cfg, "")

	require.Equal(t, "_Co-Authored-By: {{ .Value }}_", got.EffectiveTrailerRules["co-authored-by"].Renderer,
		"Co-Authored-By renders as a credit line by default, with no rendering.trailers configured (ADR-0058)")
}

func TestWithEnvDerivations_UserRuleOverridesBuiltInCoAuthoredByDefault(t *testing.T) {
	driver := &config.ContentDriver{}
	cfg := &config.Config{
		Rendering: &config.Rendering{Commit: &config.RenderingCommit{
			Trailers: []config.FooterRule{{Token: "co-authored-by", Renderer: "custom"}},
		}},
		Changelog: driver,
	}

	got := withEnvDerivations(driver, cfg, "")

	assert.Equal(t, "custom", got.EffectiveTrailerRules["co-authored-by"].Renderer, "user rule replaces the built-in default")
}

func TestWithEnvDerivations_UserRuleHidesBuiltInCoAuthoredByDefault(t *testing.T) {
	driver := &config.ContentDriver{}
	cfg := &config.Config{
		Rendering: &config.Rendering{Commit: &config.RenderingCommit{
			Trailers: []config.FooterRule{{Token: "Co-Authored-By", Hide: true}},
		}},
		Changelog: driver,
	}

	got := withEnvDerivations(driver, cfg, "")

	assert.True(t, got.EffectiveTrailerRules["co-authored-by"].Hide, "the existing hide escape hatch suppresses the built-in default too")
}
