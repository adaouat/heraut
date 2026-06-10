package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

func TestWithEnvDerivations_CarriesRemoteMetadata(t *testing.T) {
	driver := &config.ContentDriver{Generator: "git-cliff"}
	cfg := &config.Config{RemoteMetadata: "disabled", Changelog: driver}

	got := withEnvDerivations(driver, cfg, "")
	assert.Equal(t, "disabled", got.RemoteMetadata)
	assert.Empty(t, driver.RemoteMetadata) // the original driver is never mutated
}

func TestWithEnvDerivations_EmptyPolicyLeavesDriverUnchanged(t *testing.T) {
	driver := &config.ContentDriver{Generator: "git-cliff"}
	cfg := &config.Config{Changelog: driver}

	got := withEnvDerivations(driver, cfg, "")
	assert.Empty(t, got.RemoteMetadata)
}
