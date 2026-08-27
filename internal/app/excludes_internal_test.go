package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

// TestWithEnvDerivations_MergesExcludes covers T224: a driver's own rendering.excludes must
// layer additively onto the global rendering.excludes, mirroring how rendering.templates already
// merges per-driver — previously the per-driver value was never read at all.
func TestWithEnvDerivations_MergesExcludes(t *testing.T) {
	driver := &config.ContentDriver{
		Rendering: &config.Rendering{Excludes: []config.Exclude{{Type: "chore"}}},
	}
	cfg := &config.Config{
		Rendering: &config.Rendering{Excludes: []config.Exclude{{Regex: "^wip:"}}},
		Changelog: driver,
	}
	got := withEnvDerivations(driver, cfg, "")
	assert.Equal(t, []config.Exclude{{Regex: "^wip:"}, {Type: "chore"}}, got.Excludes,
		"global and per-driver excludes must both apply, additively")
	assert.Nil(t, driver.Excludes, "the original driver is never mutated")
}

func TestWithEnvDerivations_ExcludesGlobalOnly(t *testing.T) {
	driver := &config.ContentDriver{}
	cfg := &config.Config{
		Rendering: &config.Rendering{Excludes: []config.Exclude{{Regex: "^wip:"}}},
		Changelog: driver,
	}
	got := withEnvDerivations(driver, cfg, "")
	assert.Equal(t, []config.Exclude{{Regex: "^wip:"}}, got.Excludes)
}

func TestWithEnvDerivations_ExcludesDriverOnly(t *testing.T) {
	driver := &config.ContentDriver{
		Rendering: &config.Rendering{Excludes: []config.Exclude{{Type: "chore"}}},
	}
	cfg := &config.Config{Changelog: driver}
	got := withEnvDerivations(driver, cfg, "")
	assert.Equal(t, []config.Exclude{{Type: "chore"}}, got.Excludes)
}
