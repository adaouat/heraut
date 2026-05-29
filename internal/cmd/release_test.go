package cmd_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelease_Structural(t *testing.T) {
	c := cmd.NewReleaseCmd()
	require.NotNil(t, c)
	assert.Equal(t, "release", c.Use)
	assert.NotEmpty(t, c.Short)
	assert.NotNil(t, c.Flags().Lookup("version"), "flag 'version' not registered")
}

func TestRelease_ConfigNotFound(t *testing.T) {
	_, err := executeRoot("release", "--config", "/nonexistent/path/.heraut.yml")
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
}

func TestRelease_InvalidConfig(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: badstrategy
`)
	_, err := executeRoot("release", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
}

func TestRelease_DryRun_OutputsVersion(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	testutil.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "v1.0.0" ;;
  "log v1.0.0..HEAD --format=%B"*) printf "feat: new feature\x00" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("release", "--config", cfgPath, "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "v1.1.0")
	assert.Contains(t, out, "[dry-run]")
}

func TestRelease_PreflightFail_GitIdentityMissing(t *testing.T) {
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
	_, err := executeRoot("release", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}
