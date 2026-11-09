package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

// Changelog sections and the notes' previous tag follow the calendar the format describes, not the
// dotted numbers: under MM.YYYY.PATCH, December 2025 comes before January 2026.
func TestTagOrderFor_CalverFollowsTheFormat(t *testing.T) {
	prefix := "v"
	cfg := &config.Config{Version: "1", Versioning: config.Versioning{Strategy: "calver", Format: "MM.YYYY.PATCH", TagPrefix: &prefix}}
	assert.Empty(t, config.Validate(cfg), "the format is accepted")

	got := tagOrderFor(cfg, "")([]string{"v12.2025.0", "v01.2026.1", "v01.2026.0", "v11.2025.3", "latest"})

	assert.Equal(t, []string{"v01.2026.1", "v01.2026.0", "v12.2025.0", "v11.2025.3", "latest"}, got)
}
