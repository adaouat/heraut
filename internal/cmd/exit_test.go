package cmd_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExitCode_Success_Zero(t *testing.T) {
	assert.Equal(t, exitcode.OK, cmd.ExitCode(nil))
}

func TestExitCode_InvalidStrategy_Config(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semvr
`)
	_, err := executeRoot("check", "config", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
}

func TestExitCode_MissingVersion_Config(t *testing.T) {
	cfgPath := writeConfig(t, `
versioning:
  strategy: semver
`)
	_, err := executeRoot("check", "config", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
}

func TestExitCode_PromotionGuard_E003(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
  prod:
    bump: promote
    source: dev
    tag_format: "prod/{version}"
`)
	// dev (the promotion source) has no tags → E003.
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l dev/* --sort=-version:refname") echo "" ;;
  *) exit 1 ;;
esac
`)
	_, err := executeRoot("version", "next", "--config", cfgPath, "--env", "prod")
	require.Error(t, err)
	assert.Equal(t, exitcode.Promotion, cmd.ExitCode(err))
}

func TestExitCode_NoCommits_Runtime(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "v1.0.0" ;;
  "log v1.0.0..HEAD --format=%B"*) echo "" ;;
  *) exit 1 ;;
esac
`)
	_, err := executeRoot("version", "next", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}

func TestExitCode_CurrentNoTags_Runtime(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
echo ""
`)
	_, err := executeRoot("version", "current", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}

func TestExitCode_VersionNext_InvalidConfig_Config(t *testing.T) {
	cfgPath := writeConfig(t, cyclicPerEnvConfig())
	exectest.FakeBin(t, "git", `#!/bin/sh
echo ""
`)
	_, err := executeRoot("version", "next", "--config", cfgPath, "--env", "prod")
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
}

func TestExitCode_CheckAll_ConfigOnlyFailure_Config(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: invalid-strategy
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "John Doe" ;;
  "config user.email") echo "john@example.com" ;;
  *) exit 0 ;;
esac
`)
	_, err := executeRoot("check", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
}
