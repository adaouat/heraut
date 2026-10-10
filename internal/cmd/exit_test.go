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

const envSelectionConfig = `
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
    branch: main
`

// TestExitCode_EnvSelection_Config pins T359: a missing or unknown --env and the per-env branch
// guard are configuration problems (Spec 01 code 2), not runtime failures (code 3).
func TestExitCode_EnvSelection_Config(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"version next without --env", []string{"version", "next"}},
		{"version next with an unknown --env", []string{"version", "next", "--env", "nope"}},
		{"version current without --env", []string{"version", "current"}},
		{"version current with an unknown --env", []string{"version", "current", "--env", "nope"}},
		{"changelog with an unknown --env", []string{"changelog", "--dry-run", "--env", "nope"}},
		{"release with an unknown --env", []string{"release", "--dry-run", "--env", "nope"}},
		{"changelog from the wrong branch", []string{"changelog", "--env", "prod"}},
		{"release from the wrong branch", []string{"release", "--env", "prod"}},
		{"version next from the wrong branch", []string{"version", "next", "--env", "prod"}},
		{"version current from the wrong branch", []string{"version", "current", "--env", "prod"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := writeConfig(t, envSelectionConfig)
			exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "rev-parse --abbrev-ref HEAD") echo "develop" ;;
  "tag -l dev/* --sort=-version:refname") echo "dev/1.0.0" ;;
  *) exit 1 ;;
esac
`)
			_, err := executeRoot(append(tc.args, "--config", cfgPath)...)
			require.Error(t, err)
			assert.Equal(t, exitcode.Config, cmd.ExitCode(err), "%v", err)
		})
	}
}

// TestExitCode_BranchGuardGitFailure_Runtime keeps the genuine git failure on the runtime code:
// only the mismatch itself is a configuration problem.
func TestExitCode_BranchGuardGitFailure_Runtime(t *testing.T) {
	cfgPath := writeConfig(t, envSelectionConfig)
	exectest.FakeBin(t, "git", "#!/bin/sh\nexit 1\n")
	_, err := executeRoot("version", "next", "--env", "prod", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err), "%v", err)
}
