package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeRoot builds a fresh root command, sets args, captures stdout, and runs it.
func executeRoot(args ...string) (string, error) {
	root := cmd.NewRootCmd("dev")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	_, err := root.ExecuteC()
	return buf.String(), err
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// ---- Structural tests ----

func TestVersionCmd_Exists(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var versionCmd, nextCmd, currentCmd, sprintCmd bool
	for _, c := range root.Commands() {
		if c.Use == "version" {
			versionCmd = true
			for _, sc := range c.Commands() {
				switch sc.Use {
				case "next":
					nextCmd = true
				case "current":
					currentCmd = true
				case "sprint":
					for _, ssc := range sc.Commands() {
						if ssc.Use == "bump" {
							sprintCmd = true
						}
					}
				}
			}
		}
	}
	assert.True(t, versionCmd, "version command missing")
	assert.True(t, nextCmd, "version next missing")
	assert.True(t, currentCmd, "version current missing")
	assert.True(t, sprintCmd, "version sprint bump missing")
}

// ---- version next ----

func TestVersionNext_Semver_MinorBump(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "v1.2.3" ;;
  "log v1.2.3..HEAD --format=%B"*) printf "feat: add thing\x00" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "next", "--config", cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "v1.3.0\n", out)
}

func TestVersionNext_Semver_NoTags_InitialVersion(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  initial_version: "0.1.0"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "next", "--config", cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "v0.1.0\n", out)
}

func TestVersionNext_Calver_FirstRelease(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l * --sort=-version:refname") echo "" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "next", "--config", cfgPath)
	require.NoError(t, err)
	// Output should be a valid CalVer tag (year.month.0)
	assert.True(t, strings.Contains(out, ".0\n"), "expected PATCH=0 in first release, got: %q", out)
}

func TestVersionNext_PerEnv_SemverAuto(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l dev/* --sort=-version:refname") echo "dev/1.0.0" ;;
  "log dev/1.0.0..HEAD --format=%B"*) printf "fix: patch\x00" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "next", "--config", cfgPath, "--env", "dev")
	require.NoError(t, err)
	assert.Equal(t, "dev/1.0.1\n", out)
}

func TestVersionNext_BranchGuard_BlocksWrongBranch(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  prod:
    bump: auto
    branch: main
    tag_format: "prod/{version}"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "rev-parse --abbrev-ref HEAD") echo "feature/x" ;;
  *) exit 1 ;;
esac
`)
	_, err := executeRoot("version", "next", "--config", cfgPath, "--env", "prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "branch")
}

func TestVersionNext_BranchGuard_ForceBypasses(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  prod:
    bump: auto
    branch: main
    tag_format: "prod/{version}"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "rev-parse --abbrev-ref HEAD") echo "feature/x" ;;
  "tag -l prod/* --sort=-version:refname") echo "prod/1.0.0" ;;
  "log prod/1.0.0..HEAD --format=%B"*) printf "fix: patch\x00" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "next", "--config", cfgPath, "--env", "prod", "--force")
	require.NoError(t, err)
	assert.Equal(t, "prod/1.0.1\n", out)
}

// ---- version current ----

func TestVersionCurrent_Semver(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") printf "v2.1.0\nv2.0.0\nv1.0.0\n" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "current", "--config", cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "v2.1.0\n", out)
}

func TestVersionCurrent_Calver(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l * --sort=-version:refname") printf "2026.05.3\n2026.05.0\n" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "current", "--config", cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "2026.05.3\n", out)
}

func TestVersionCurrent_PerEnv(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  prod:
    bump: promote
    tag_format: "prod/{version}"
  dev:
    bump: auto
    tag_format: "dev/{version}"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l prod/* --sort=-version:refname") printf "prod/1.5.0\nprod/1.4.0\n" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "current", "--config", cfgPath, "--env", "prod")
	require.NoError(t, err)
	assert.Equal(t, "prod/1.5.0\n", out)
}

func TestVersionCurrent_Bare_PerEnvBuildFormat(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"
environments:
  main:
    bump: auto
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l main/*-* --sort=-version:refname") printf "main/7.4.1-158404\nmain/7.4.0-155398\n" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "current", "--config", cfgPath, "--env", "main", "--bare")
	require.NoError(t, err)
	assert.Equal(t, "7.4.1\n", out)
}

func TestVersionCurrent_Bare_Semver(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") printf "v1.2.3\nv1.2.2\n" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "current", "--config", cfgPath, "--bare")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3\n", out)
}

func TestVersionCurrent_NoTags_Error(t *testing.T) {
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
}

// ---- config validation (T98) ----

func cyclicPerEnvConfig() string {
	return `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
environments:
  prod:
    bump: promote
    source: staging
  staging:
    bump: promote
    source: prod
`
}

func TestVersionNext_InvalidConfig_FailsValidation(t *testing.T) {
	cfgPath := writeConfig(t, cyclicPerEnvConfig())
	exectest.FakeBin(t, "git", `#!/bin/sh
echo ""
`)
	out, err := executeRoot("version", "next", "--config", cfgPath, "--env", "prod")
	require.Error(t, err)
	assert.Contains(t, out, "environments.")
	assert.Contains(t, out, "cycle detected")
}

func TestVersionCurrent_InvalidConfig_FailsValidation(t *testing.T) {
	cfgPath := writeConfig(t, cyclicPerEnvConfig())
	exectest.FakeBin(t, "git", `#!/bin/sh
echo ""
`)
	out, err := executeRoot("version", "current", "--config", cfgPath, "--env", "prod")
	require.Error(t, err)
	assert.Contains(t, out, "environments.")
	assert.Contains(t, out, "cycle detected")
}

// ---- version sprint bump ----

func TestVersionSprintBump_IncrementsValue(t *testing.T) {
	cfgPath := writeConfig(t, `version: "1"
versioning:
  strategy: calver
  format: "YYYY.SPRINT.PATCH"
  sprint: 5
`)
	out, err := executeRoot("version", "sprint", "bump", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "6")

	// Verify the file was updated
	data, _ := os.ReadFile(cfgPath)
	assert.Contains(t, string(data), "sprint: 6")
	assert.NotContains(t, string(data), "sprint: 5")
}

func TestVersionSprintBump_FromZero(t *testing.T) {
	// sprint not present in file → treated as 0, bumped to 1
	cfgPath := writeConfig(t, `version: "1"
versioning:
  strategy: calver
  format: "YYYY.SPRINT.PATCH"
`)
	out, err := executeRoot("version", "sprint", "bump", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "1")

	data, _ := os.ReadFile(cfgPath)
	assert.Contains(t, string(data), "sprint: 1")
}
