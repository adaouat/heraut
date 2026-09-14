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
		Rendering: &config.Rendering{Trailers: []config.FooterRule{{Token: "Co-authored-by", Renderer: "driver"}}},
	}
	cfg := &config.Config{
		Rendering: &config.Rendering{Trailers: []config.FooterRule{
			{Token: "co-authored-by", Renderer: "global"},
			{Token: "Refs", Hide: true},
		}},
		Changelog: driver,
	}
	got := withEnvDerivations(driver, cfg, "")
	require.Equal(t, "driver", got.EffectiveTrailerRules["co-authored-by"].Renderer, "driver overrides global, keyed lowercase")
	require.True(t, got.EffectiveTrailerRules["refs"].Hide, "unset token falls through from global")
	assert.Nil(t, driver.EffectiveTrailerRules, "the original driver is never mutated")
}
