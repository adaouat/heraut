package cmd_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/testutil"
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
	testutil.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l dev/* --sort=-version:refname") echo "" ;;
  *) exit 1 ;;
esac
`)
	_, err := executeRoot("version", "next", "--config", cfgPath, "--env", "prod")
	require.Error(t, err)
	assert.Equal(t, exitcode.Promotion, cmd.ExitCode(err))
}

func TestExitCode_CurrentNoTags_Runtime(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  prefix: "v"
`)
	testutil.FakeBin(t, "git", `#!/bin/sh
echo ""
`)
	_, err := executeRoot("version", "current", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}
