package cmd_test

import (
	"bytes"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- structural ----

func TestCheckCmd_Structure(t *testing.T) {
	root := cmd.NewCheckCmd()
	var checkSubs = map[string]bool{}
	for _, sub := range root.Commands() {
		checkSubs[sub.Use] = true
	}
	assert.True(t, checkSubs["config"], "check config missing")
	assert.True(t, checkSubs["runtime"], "check runtime missing")
	assert.False(t, checkSubs["cliff"], "check cliff must be removed (Phase B)")
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

func TestCheckConfig_ShowsConfigFileSource(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
`)
	root := cmd.NewRootCmd("dev")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "config", "--config", cfgPath})
	_, _ = root.ExecuteC()
	out := buf.String()
	assert.Contains(t, out, cfgPath)
	assert.Contains(t, out, "--config")
}

func TestCheckConfig_InvalidPrintsPath(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: badstrategy
`)
	root := cmd.NewRootCmd("dev")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "config", "--config", cfgPath})
	_, _ = root.ExecuteC()
	assert.Contains(t, buf.String(), "versioning.strategy")
}

// ---- check runtime (no config) ----

func TestCheckRuntime_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", "")

	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "John Doe" ;;
  "config user.email") echo "john@example.com" ;;
  *) exit 0 ;;
esac
`)
	exectest.FakeBin(t, "gh", `#!/bin/sh
echo "gh 2.x"
`)
	exectest.FakeBin(t, "glab", `#!/bin/sh
echo "glab 1.x"
`)
	exectest.FakeBin(t, "cog", `#!/bin/sh
echo "cog 6.x"
`)

	out, err := executeRoot("check", "runtime")
	require.NoError(t, err)
	assert.Contains(t, out, "no config")
}

// ---- check runtime ----

func TestCheckRuntime_AllGood(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
changelog: {}
forges:
  - name: github
    platform: github
    repository: acme/widget
release:
  targets:
    - forge: github
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "John Doe" ;;
  "config user.email") echo "john@example.com" ;;
  *) exit 0 ;;
esac
`)
	exectest.FakeBin(t, "gh", `#!/bin/sh
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
	exectest.FakeBin(t, "git", `#!/bin/sh
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
	exectest.FakeBin(t, "git", `#!/bin/sh
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
	exectest.FakeBin(t, "git", `#!/bin/sh
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

// ---- check (bare) ----

func TestCheckAll_PassesAll(t *testing.T) {
	testutil.ClearCIEnv(t)
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
changelog: {}
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "John Doe" ;;
  "config user.email") echo "john@example.com" ;;
  "remote get-url origin") exit 1 ;;
  *) exit 0 ;;
esac
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

func TestCheckAll_NoConfigFile_Degrades(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HERAUT_FILE", "")

	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.x" ;;
  "config user.name") echo "John Doe" ;;
  "config user.email") echo "john@example.com" ;;
  *) exit 0 ;;
esac
`)
	exectest.FakeBin(t, "gh", `#!/bin/sh
echo "gh 2.x"
`)
	exectest.FakeBin(t, "glab", `#!/bin/sh
echo "glab 1.x"
`)
	exectest.FakeBin(t, "cog", `#!/bin/sh
echo "cog 6.x"
`)

	out, err := executeRoot("check")
	require.NoError(t, err)
	assert.Contains(t, out, "no config found")
}
