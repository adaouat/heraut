package cmd_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChangelogCmd(t *testing.T) {
	c := cmd.NewChangelogCmd()
	require.NotNil(t, c)
	assert.Equal(t, "changelog", c.Use)
	assert.NotEmpty(t, c.Short)

	for _, name := range []string{"commit", "tag", "version"} {
		assert.NotNil(t, c.Flags().Lookup(name), "flag %q not registered", name)
	}
}

// TestRootCmd_HasChangelogSubcommand verifies `heraut changelog` is wired into the root.
func TestRootCmd_HasChangelogSubcommand(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var found bool
	for _, sub := range root.Commands() {
		if sub.Use == "changelog" {
			found = true
			break
		}
	}
	assert.True(t, found, "changelog subcommand not registered on root")
}
