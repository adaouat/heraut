package cmd_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- structural ----

func TestCliffCmd_Structure(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var cliffSubs map[string]bool
	for _, c := range root.Commands() {
		if c.Use == "cliff" {
			cliffSubs = map[string]bool{}
			for _, sub := range c.Commands() {
				cliffSubs[sub.Use] = true
			}
		}
	}
	require.NotNil(t, cliffSubs, "cliff command missing from root")
	assert.True(t, cliffSubs["changelog"], "cliff changelog missing")
	assert.True(t, cliffSubs["release-notes"], "cliff release-notes missing")
}

// ---- cliff changelog ----

func TestCliffChangelog_NoChangelogConfigured_PrintsDefault(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
`)
	out, err := executeRoot("cliff", "changelog", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "[changelog]")
}

// ---- cliff release-notes ----

func TestCliffReleaseNotes_NotConfigured_PrintsDefault(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
`)
	out, err := executeRoot("cliff", "release-notes", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "[changelog]")
}
