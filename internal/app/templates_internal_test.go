package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

func TestWithEnvDerivations_MergesTemplates(t *testing.T) {
	driver := &config.ContentDriver{
		Generator: "native",
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
