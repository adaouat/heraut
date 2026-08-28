package cmd_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChangelogCmd(t *testing.T) {
	c := cmd.NewChangelogCmd("v0.0.0-test")
	require.NotNil(t, c)
	assert.Equal(t, "changelog", c.Use)
	assert.NotEmpty(t, c.Short)

	for _, name := range []string{"commit", "tag", "no-push", "version", "build"} {
		assert.NotNil(t, c.Flags().Lookup(name), "flag %q not registered", name)
	}
}

func TestNewChangelogCmd_RegenerateFlag(t *testing.T) {
	c := cmd.NewChangelogCmd("v0.0.0-test")
	f := c.Flags().Lookup("regenerate")
	require.NotNil(t, f, "changelog has a --regenerate flag")
	assert.Equal(t, "false", f.DefValue)
}

func TestChangelog_BuildRejectsInvalidValue(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"
`)
	_, err := executeRoot("changelog", "--config", cfgPath, "--env", "uat",
		"--version", "7.4.1", "--build", "bad/value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build")
}

func TestChangelog_BuildRequiresVersion(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"
`)
	_, err := executeRoot("changelog", "--config", cfgPath, "--env", "uat", "--build", "12345")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--version")
}

func TestChangelog_VersionFlag_RejectsWhitespace(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	_, err := executeRoot("changelog", "--config", cfgPath, "--version", "1.2.3 ", "--dry-run")
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
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
  output: CHANGELOG.md
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
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

func TestChangelog_DryRun_NoPush(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
changelog:
  output: CHANGELOG.md
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "v1.0.0" ;;
  "log v1.0.0..HEAD --format=%B"*) printf "feat: new feature\x00" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("changelog", "--config", cfgPath, "--tag", "--no-push", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "no push")
	assert.NotContains(t, out, "would push")
}

// TestChangelog_ForeignFile_ShortSummaryNotRepeated reproduces the double-display bug: a
// generate failure's detailed error was already shown once by the step reporter, and the error
// RunE returns must not repeat it verbatim — fang's top-level error panel shows a short summary
// instead (exitcode.WrapSummary, forge v0.18.0).
func TestChangelog_ForeignFile_ShortSummaryNotRepeated(t *testing.T) {
	changelogDir := t.TempDir()
	outPath := filepath.Join(changelogDir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(outPath, []byte("# Changelog\n\nsome content with no anchors\n"), 0o644))

	cfgPath := writeConfig(t, fmt.Sprintf(`
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
changelog:
  output: %s
`, outPath))

	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.49.0" ;;
  "config user.name") echo "Test User" ;;
  "config user.email") echo "test@example.com" ;;
  "tag -l v* --sort=-version:refname") echo "v1.0.0" ;;
  "log v1.0.0..HEAD --format=%B"*) printf "feat: new feature\x00" ;;
  *) exit 1 ;;
esac
`)

	out, err := executeRoot("changelog", "--config", cfgPath)
	require.Error(t, err)
	assert.Contains(t, out, "no heraut-release anchors",
		"the step reporter must still show the detailed error once")
	assert.NotContains(t, err.Error(), "no heraut-release anchors",
		"the returned error (fang's display) must be a short summary, not a repeat of the detail")
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}

func TestChangelog_PreflightFail_GitIdentityMissing(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	// git --version succeeds; everything else (config user.name, config user.email) exits 1.
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.49.0" ;;
  *) exit 1 ;;
esac
`)
	_, err := executeRoot("changelog", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}
