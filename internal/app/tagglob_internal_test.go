package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

// TestWithEnvDerivations_SetsTagGlobForPerEnv verifies the app layer computes the per-env git tag
// glob (tagfmt.GlobPattern) onto the ContentDriver so the native generator can scope its tag walk.
func TestWithEnvDerivations_SetsTagGlobForPerEnv(t *testing.T) {
	driver := &config.ContentDriver{Generator: "native"}
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver-per-env", TagFormat: "{version}_{env}"},
		Changelog:  driver,
	}

	got := withEnvDerivations(driver, cfg, "prod")
	assert.Equal(t, "*_prod", got.TagGlob)
	assert.Empty(t, driver.TagGlob, "the original driver is never mutated")
}

// TestWithEnvDerivations_NoTagGlobForNonPerEnv verifies a non-per-env tag format leaves TagGlob
// empty, so native keeps listing all tags (unchanged behaviour for semver/calver).
func TestWithEnvDerivations_NoTagGlobForNonPerEnv(t *testing.T) {
	driver := &config.ContentDriver{Generator: "native"}
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagFormat: "v{version}"},
		Changelog:  driver,
	}

	got := withEnvDerivations(driver, cfg, "")
	assert.Empty(t, got.TagGlob)
}
