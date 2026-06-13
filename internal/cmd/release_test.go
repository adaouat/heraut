package cmd_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelease_Structural(t *testing.T) {
	c := cmd.NewReleaseCmd()
	require.NotNil(t, c)
	assert.Equal(t, "release", c.Use)
	assert.NotEmpty(t, c.Short)
	for _, name := range []string{"version", "build"} {
		assert.NotNil(t, c.Flags().Lookup(name), "flag %q not registered", name)
	}
}

func TestRelease_BuildRequiresVersion(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"
`)
	_, err := executeRoot("release", "--config", cfgPath, "--env", "uat", "--build", "12345")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--version")
}

func TestRelease_BuildRejectsInvalidValue(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"
`)
	_, err := executeRoot("release", "--config", cfgPath, "--env", "uat",
		"--version", "7.4.1", "--build", "bad/value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build")
}

func TestRelease_Build_DryRun_RendersTag(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"
environments:
  uat:
    bump: auto
release:
  platforms:
    - platform: github
      name: github
      repository: test/repo
`)
	out, err := executeRoot("release", "--config", cfgPath, "--env", "uat",
		"--version", "7.4.1", "--build", "158404", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "uat/7.4.1-158404")
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
release:
  platforms:
    - platform: github
      name: github
      repository: test/repo
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
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

func TestRelease_NoPlatforms_Error(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
changelog:
  generator: git-cliff
  output: CHANGELOG.md
`)
	_, err := executeRoot("release", "--config", cfgPath, "--dry-run")
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
	assert.Contains(t, err.Error(), "platform")
}

func TestRelease_EmptyPlatforms_Error(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
release:
  platforms: []
`)
	_, err := executeRoot("release", "--config", cfgPath, "--dry-run")
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
	assert.Contains(t, err.Error(), "platform")
}

func TestRelease_VersionFlag_InvalidFormat(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
release:
  platforms:
    - platform: github
      name: github
      repository: test/repo
`)
	tests := []struct {
		name    string
		version string
	}{
		{"whitespace only", " "},
		{"contains space", "1.2 .3"},
		{"trailing space", "1.2.3 "},
		{"contains tab", "1.2.3\t"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := executeRoot("release", "--config", cfgPath, "--version", tc.version, "--dry-run")
			require.Error(t, err)
			assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
		})
	}
}

func TestRelease_VersionFlag_ValidFormats(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
release:
  platforms:
    - platform: github
      name: github
      repository: test/repo
`)
	tests := []struct {
		name    string
		version string
	}{
		{"with v prefix", "v1.2.3"},
		{"without v prefix", "1.2.3"},
		{"zeros", "v0.0.0"},
		{"large numbers", "v1.10.100"},
		{"bare word", "notaversion"},
		{"only major", "v1"},
		{"only major.minor", "v1.2"},
		{"v prefix only", "v"},
		{"non-numeric", "va.b.c"},
		{"calver year.patch", "2024.03"},
		{"calver full", "2024.03.15.2"},
		{"pre-release", "1.2.3-rc.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := executeRoot("release", "--config", cfgPath, "--version", tc.version, "--dry-run")
			require.NoError(t, err)
			assert.Contains(t, out, "[dry-run]")
		})
	}
}

func TestRelease_PreflightFail_GitIdentityMissing(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
release:
  platforms:
    - platform: github
      name: github
      repository: test/repo
`)
	// git --version succeeds; everything else (config user.name, config user.email) exits 1.
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "--version") echo "git version 2.49.0" ;;
  *) exit 1 ;;
esac
`)
	_, err := executeRoot("release", "--config", cfgPath)
	require.Error(t, err)
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}
