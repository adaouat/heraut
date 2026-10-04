package cmd_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const preReleaseConfig = `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`

func TestPreReleaseFlag_Registered(t *testing.T) {
	root := cmd.NewRootCmd("v0.0.0-test")
	for _, path := range [][]string{{"release"}, {"version", "next"}} {
		c, _, err := root.Find(path)
		require.NoError(t, err)
		f := c.Flags().Lookup("pre-release")
		require.NotNil(t, f, "%v has --pre-release", path)
		assert.Equal(t, "", f.DefValue)
	}

	changelog, _, err := root.Find([]string{"changelog"})
	require.NoError(t, err)
	assert.Nil(t, changelog.Flags().Lookup("pre-release"), "changelog never mints pre-releases")
}

func TestVersionNext_PreRelease_PrintsTagAndEscalationWarning(t *testing.T) {
	cfgPath := writeConfig(t, preReleaseConfig)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") printf "v1.3.1-rc.1\nv1.3.0\n" ;;
  "log v1.3.0..HEAD --format=%B%x00") printf "fix: a\x00feat: b\x00" ;;
  "tag -l v* --merged HEAD --sort=-version:refname") printf "v1.3.1-rc.1\nv1.3.0\n" ;;
  "log v1.3.1-rc.1..HEAD --format=%B%x00") printf "feat: b\x00" ;;
  *) exit 1 ;;
esac
`)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath, "--pre-release", "rc")
	require.NoError(t, err)

	assert.Equal(t, "v1.4.0-rc.1\n", stdout)
	assert.Contains(t, stderr, "pre-release core escalated 1.3.1 → 1.4.0")
}

func TestVersionNext_PreRelease_RegressionIsRuntimeError(t *testing.T) {
	cfgPath := writeConfig(t, preReleaseConfig)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") printf "v1.4.0-rc.1\nv1.3.0\n" ;;
  "log v1.3.0..HEAD --format=%B%x00") printf "feat: x\x00" ;;
  *) exit 1 ;;
esac
`)

	_, _, err := executeRootSeparateStreams("version", "next", "--config", cfgPath, "--pre-release", "beta")
	require.Error(t, err)
	assert.ErrorContains(t, err, "would sort below existing")
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}

func TestVersionNext_PreRelease_UsageErrorsAreConfigExit(t *testing.T) {
	cfgPath := writeConfig(t, preReleaseConfig)
	tests := []struct {
		name string
		args []string
	}{
		{"with set-version", []string{"--pre-release", "rc", "--set-version", "1.4.0-rc.1"}},
		{"invalid label", []string{"--pre-release", "RC!"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"version", "next", "--config", cfgPath}, tc.args...)
			_, _, err := executeRootSeparateStreams(args...)
			require.Error(t, err)
			assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
		})
	}
}
