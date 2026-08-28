package cmd_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelease_Structural(t *testing.T) {
	c := cmd.NewReleaseCmd("v0.0.0-test")
	require.NotNil(t, c)
	assert.Equal(t, "release", c.Use)
	assert.NotEmpty(t, c.Short)
	for _, name := range []string{"version", "build"} {
		assert.NotNil(t, c.Flags().Lookup(name), "flag %q not registered", name)
	}
}

func TestNewReleaseCmd_RegenerateChangelogFlag(t *testing.T) {
	c := cmd.NewReleaseCmd("v0.0.0-test")
	f := c.Flags().Lookup("regenerate-changelog")
	require.NotNil(t, f, "release has a --regenerate-changelog flag")
	assert.Equal(t, "false", f.DefValue)
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
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
`)
	out, err := executeRoot("release", "--config", cfgPath, "--env", "uat",
		"--version", "7.4.1", "--build", "158404", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "uat/7.4.1-158404")
}

// TestRelease_ForeignFile_ShortSummaryNotRepeated mirrors
// TestChangelog_ForeignFile_ShortSummaryNotRepeated for `heraut release`'s own pipe.Run() call
// site — the fix (exitcode.WrapSummary, forge v0.18.0) must apply symmetrically to both.
func TestRelease_ForeignFile_ShortSummaryNotRepeated(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")
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
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
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
	exectest.FakeBin(t, "gh", `#!/bin/sh
case "$*" in
  "--version") echo "gh version 2.x" ;;
  "api repos/test/repo/releases?per_page=1") echo "[]" ;;
  *) exit 1 ;;
esac
`)

	out, err := executeRoot("release", "--config", cfgPath)
	require.Error(t, err)
	assert.Contains(t, out, "no heraut-release anchors",
		"the step reporter must still show the detailed error once")
	assert.NotContains(t, err.Error(), "no heraut-release anchors",
		"the returned error (fang's display) must be a short summary, not a repeat of the detail")
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
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
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
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
	testutil.ClearCIEnv(t)
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
changelog:
  output: CHANGELOG.md
`)
	dir := t.TempDir()
	t.Chdir(dir)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "remote get-url origin") exit 1 ;;
  *) exit 0 ;;
esac
`)
	_, err := executeRoot("release", "--config", cfgPath, "--dry-run")
	require.Error(t, err)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
	assert.Contains(t, err.Error(), "publish destination")
}

func TestRelease_EmptyTargets_ZeroConfigResolves(t *testing.T) {
	// release.targets: [] with no forges: configured is not itself an error — it is the
	// zero-config shape, which is valid as long as a forge auto-detects (CI env or git origin).
	// Only "nothing resolves at all" (TestRelease_NoPlatforms_Error) is the hard config error.
	testutil.ClearCIEnv(t)
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
release:
  targets: []
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "remote get-url origin") echo "https://github.com/acme/widget.git" ;;
  "tag -l v* --sort=-version:refname") echo "v1.0.0" ;;
  "log v1.0.0..HEAD --format=%B"*) printf "feat: new feature\x00" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("release", "--config", cfgPath, "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "[dry-run]")
	// Zero-config publishing must still render a meaningful platform label — not a blank
	// name ("Publish to  —" with nothing between the space and the em dash). See I2:
	// platformConfigFromTarget falls back to the resolved forge type when no forges: entry
	// supplies f.Name.
	assert.Contains(t, out, "Publish to github")
	assert.NotContains(t, out, "Publish to  ", "platform name must not render blank")
}

func TestRelease_VersionFlag_InvalidFormat(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
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
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
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
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
`)
	// git --version succeeds; everything else (config user.name, config user.email,
	// remote get-url origin) exits 1 — the origin failure is swallowed by forge resolution
	// (an explicit forges: entry never depends on it), so it never masks the preflight failure.
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
