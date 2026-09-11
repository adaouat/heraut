package config_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestConfig_HooksAccessors_NilSafe(t *testing.T) {
	cfg := &config.Config{}

	assert.Nil(t, cfg.PostBumpHooks())
	assert.Nil(t, cfg.PreChangelogHooks())
	assert.Nil(t, cfg.PreTagHooks())
	assert.Nil(t, cfg.PostTagHooks())
	assert.Nil(t, cfg.PreReleaseHooks())
	assert.Nil(t, cfg.PostReleaseHooks())
}

func TestConfig_HooksAccessors_ReturnsConfiguredList(t *testing.T) {
	cfg := &config.Config{
		Hooks: &config.Hooks{
			PostBump:     []string{"echo post_bump"},
			PreChangelog: []string{"echo pre_changelog"},
			PreTag:       []string{"echo pre_tag"},
			PostTag:      []string{"echo post_tag"},
			PreRelease:   []string{"echo pre_release"},
			PostRelease:  []string{"echo post_release"},
		},
	}

	assert.Equal(t, []string{"echo post_bump"}, cfg.PostBumpHooks())
	assert.Equal(t, []string{"echo pre_changelog"}, cfg.PreChangelogHooks())
	assert.Equal(t, []string{"echo pre_tag"}, cfg.PreTagHooks())
	assert.Equal(t, []string{"echo post_tag"}, cfg.PostTagHooks())
	assert.Equal(t, []string{"echo pre_release"}, cfg.PreReleaseHooks())
	assert.Equal(t, []string{"echo post_release"}, cfg.PostReleaseHooks())
}
