package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

// TestWithEnvDerivations_SetsTagGlobForPerEnv verifies the app layer computes the per-env git tag
// glob (tagfmt.GlobPattern) onto the ContentDriver so the native generator can scope its tag walk.
func TestWithEnvDerivations_SetsTagGlobForPerEnv(t *testing.T) {
	driver := &config.ContentDriver{}
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
	driver := &config.ContentDriver{}
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver", TagFormat: "v{version}"},
		Changelog:  driver,
	}

	got := withEnvDerivations(driver, cfg, "")
	assert.Empty(t, got.TagGlob)
}

// TestWithEnvDerivations_DerivesTagPatternForPerEnv verifies the app layer derives a regex
// TagPattern from the effective tag_format for a per-env strategy, mirroring the TagGlob
// derivation above but for the regex form some generators/tests scope by.
func TestWithEnvDerivations_DerivesTagPatternForPerEnv(t *testing.T) {
	driver := &config.ContentDriver{}
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "calver-per-env", TagFormat: "{version}_{env}"},
		Changelog:  driver,
	}

	got := withEnvDerivations(driver, cfg, "prod")
	assert.Equal(t, "^.+_prod$", got.TagPattern)
	assert.Empty(t, driver.TagPattern, "the original driver is never mutated")
}

// TestWithEnvDerivations_ExplicitTagPatternWins verifies a user-set TagPattern is never
// overridden by the per-env auto-derivation.
func TestWithEnvDerivations_ExplicitTagPatternWins(t *testing.T) {
	driver := &config.ContentDriver{TagPattern: "custom-pattern"}
	cfg := &config.Config{
		Versioning: config.Versioning{Strategy: "semver-per-env", TagFormat: "{version}_{env}"},
		Changelog:  driver,
	}

	got := withEnvDerivations(driver, cfg, "prod")
	assert.Equal(t, "custom-pattern", got.TagPattern, "explicit user tag_pattern must win over derivation")
}
