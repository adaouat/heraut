package cmd_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/testutil"
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

func TestChangelog_ConfigNotFound(t *testing.T) {
	_, err := executeRoot("changelog", "--config", "/nonexistent/path/.heraut.yml")
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
}

func TestChangelog_InvalidConfig(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: badstrategy
`)
	_, err := executeRoot("changelog", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
}

func TestChangelog_DryRun_OutputsVersion(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
changelog:
  generator: git-cliff
  output: CHANGELOG.md
`)
	testutil.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "v1.0.0" ;;
  "log v1.0.0..HEAD --format=%B"*) printf "feat: new feature\x00" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("changelog", "--config", cfgPath, "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "v1.1.0")
	assert.Contains(t, out, "[dry-run]")
}

func TestChangelog_PreflightFail_GitIdentityMissing(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	// git --version succeeds; everything else (config user.name, config user.email) exits 1.
	testutil.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.49.0" ;;
  *) exit 1 ;;
esac
`)
	_, err := executeRoot("changelog", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}
