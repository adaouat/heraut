package cmd_test

import (
	"bytes"
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- structural ----

func TestCheckCmd_Structure(t *testing.T) {
	root := cmd.NewRootCmd()
	var checkSubs, cliffSubs map[string]bool
	for _, c := range root.Commands() {
		if c.Use == "check" {
			checkSubs = map[string]bool{}
			for _, sub := range c.Commands() {
				checkSubs[sub.Use] = true
				if sub.Use == "cliff" {
					cliffSubs = map[string]bool{}
					for _, s := range sub.Commands() {
						cliffSubs[s.Use] = true
					}
				}
			}
		}
	}
	require.NotNil(t, checkSubs, "check command missing from root")
	assert.True(t, checkSubs["config"], "check config missing")
	assert.True(t, checkSubs["runtime"], "check runtime missing")
	assert.True(t, checkSubs["cliff"], "check cliff missing")
	require.NotNil(t, cliffSubs, "check cliff has no subcommands")
	assert.True(t, cliffSubs["changelog"], "check cliff changelog missing")
	assert.True(t, cliffSubs["release-notes"], "check cliff release-notes missing")
}

// ---- check config ----

func TestCheckConfig_Valid(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
`)
	out, err := executeRoot("check", "config", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "ok")
}

func TestCheckConfig_Invalid(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: badstrategy
`)
	_, err := executeRoot("check", "config", "--config", cfgPath)
	require.Error(t, err)
}

func TestCheckConfig_InvalidPrintsPath(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: badstrategy
`)
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "config", "--config", cfgPath})
	_, _ = root.ExecuteC()
	assert.Contains(t, buf.String(), "versioning.strategy")
}

// ---- check runtime ----

func TestCheckRuntime_AllGood(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
release:
  platforms:
    - platform: github
`)
	testutil.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "John Doe" ;;
  "config user.email") echo "john@example.com" ;;
  *) exit 0 ;;
esac
`)
	testutil.FakeBin(t, "git-cliff", `#!/bin/sh
echo "git-cliff 2.7.0"
`)
	testutil.FakeBin(t, "gh", `#!/bin/sh
echo "gh 2.x"
`)
	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GITHUB_REPOSITORY", "org/repo")

	out, err := executeRoot("check", "runtime", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "git")
}

func TestCheckRuntime_GitMissing(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
`)
	testutil.FakeBin(t, "git", `#!/bin/sh
exit 127
`)

	_, err := executeRoot("check", "runtime", "--config", cfgPath)
	require.Error(t, err)
}

func TestCheckRuntime_GeneratorMissing(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
`)
	testutil.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "John Doe" ;;
  "config user.email") echo "john@example.com" ;;
  *) exit 0 ;;
esac
`)
	testutil.FakeBin(t, "git-cliff", `#!/bin/sh
exit 127
`)

	_, err := executeRoot("check", "runtime", "--config", cfgPath)
	require.Error(t, err)
}

func TestCheckRuntime_GitUserNameMissing(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
`)
	testutil.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "" ;;
  "config user.email") echo "john@example.com" ;;
  *) exit 0 ;;
esac
`)

	_, err := executeRoot("check", "runtime", "--config", cfgPath)
	require.Error(t, err)
}

func TestCheckRuntime_GitUserEmailMissing(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
`)
	testutil.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "John Doe" ;;
  "config user.email") echo "" ;;
  *) exit 0 ;;
esac
`)

	_, err := executeRoot("check", "runtime", "--config", cfgPath)
	require.Error(t, err)
}

// ---- check cliff ----

func TestCheckCliffChangelog_Valid(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
`)
	testutil.FakeBin(t, "git-cliff", `#!/bin/sh
echo "git-cliff 2.7.0"
exit 0
`)

	out, err := executeRoot("check", "cliff", "changelog", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "ok")
}

func TestCheckCliffChangelog_Invalid(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
`)
	testutil.FakeBin(t, "git-cliff", `#!/bin/sh
case "$1" in
  "--version") echo "git-cliff 2.7.0" ;;
  "--context") echo "error: invalid config" >&2; exit 1 ;;
  *) exit 0 ;;
esac
`)

	_, err := executeRoot("check", "cliff", "changelog", "--config", cfgPath)
	require.Error(t, err)
}

func TestCheckCliffChangelog_NotGitCliff(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: communique
  config: myconfig.yaml
`)
	out, err := executeRoot("check", "cliff", "changelog", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "skip")
}

// ---- check (bare) ----

func TestCheckAll_PassesAll(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
`)
	testutil.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "John Doe" ;;
  "config user.email") echo "john@example.com" ;;
  *) exit 0 ;;
esac
`)
	testutil.FakeBin(t, "git-cliff", `#!/bin/sh
echo "git-cliff 2.7.0"
exit 0
`)

	_, err := executeRoot("check", "--config", cfgPath)
	require.NoError(t, err)
}

func TestCheckAll_ConfigFails(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: invalid-strategy
`)
	_, err := executeRoot("check", "--config", cfgPath)
	require.Error(t, err)
}
