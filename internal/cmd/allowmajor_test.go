package cmd_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowMajorFlag_Registered(t *testing.T) {
	root := cmd.NewRootCmd("v0.0.0-test")
	for _, path := range [][]string{{"release"}, {"changelog"}, {"version", "next"}} {
		c, _, err := root.Find(path)
		require.NoError(t, err)
		f := c.Flags().Lookup("allow-major")
		require.NotNil(t, f, "%v has --allow-major", path)
		assert.Equal(t, "false", f.DefValue)
	}

	current, _, err := root.Find([]string{"version", "current"})
	require.NoError(t, err)
	assert.Nil(t, current.Flags().Lookup("allow-major"), "version current never bumps")
}

const stayAtV0Config = `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  bump:
    stay_at_v0: true
`

func fakeGitBreakingSinceV068(t *testing.T) {
	t.Helper()
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "v0.68.0" ;;
  "log v0.68.0..HEAD --format=%B"*) printf "feat!: break the api\x00" ;;
  *) exit 1 ;;
esac
`)
}

func TestVersionNext_StayAtV0_HoldsBackWithWarningOnStderr(t *testing.T) {
	cfgPath := writeConfig(t, stayAtV0Config)
	fakeGitBreakingSinceV068(t)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "v0.69.0\n", stdout, "stdout must stay exactly the tag")
	assert.Contains(t, stderr, "! major bump held back by versioning.bump.stay_at_v0: v1.0.0 → v0.69.0")
	assert.Contains(t, stderr, "--allow-major")
	assert.Contains(t, stderr, "  - feat!: break the api")
}

func TestVersionNext_StayAtV0_FailingResolvePrintsNoWarning(t *testing.T) {
	cfgPath := writeConfig(t, stayAtV0Config)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "v0.68.0" ;;
  *) exit 1 ;;
esac
`)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath)
	require.ErrorContains(t, err, "reading git log", "the failure must come from Resolve, not from earlier setup")

	assert.Empty(t, stdout, "a failed resolve must not print a tag")
	assert.NotContains(t, stderr, "held back")
	assert.NotContains(t, stderr, "! ", "a failed resolve prints no warning line")
}

func TestVersionNext_StayAtV0_AllowMajorReleasesMajor(t *testing.T) {
	cfgPath := writeConfig(t, stayAtV0Config)
	fakeGitBreakingSinceV068(t)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath, "--allow-major")
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0\n", stdout)
	assert.Empty(t, stderr)
}

func TestVersionNext_WithoutStayAtV0_BehavesAsBefore(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	fakeGitBreakingSinceV068(t)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0\n", stdout)
	assert.Empty(t, stderr)
}

func stayAtV0RealRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	testutil.RealGitRepo(t, "v0.68.0")
	if out, err := exec.Command("git", "commit", "--allow-empty", "-m", "feat!: break the api").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	require.NoError(t, os.WriteFile(".heraut.yml", []byte(stayAtV0Config), 0o644))
}

func TestChangelog_RealGit_StayAtV0_TagsMinorAndWarns(t *testing.T) {
	stayAtV0RealRepo(t)

	out, err := executeRoot("changelog", "--tag", "--no-push")
	require.NoErrorf(t, err, "output:\n%s", out)

	assert.Contains(t, out, "! major bump held back by versioning.bump.stay_at_v0: v1.0.0 → v0.69.0")
	tags, err := exec.Command("git", "tag", "-l", "v0.69.0").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(tags), "v0.69.0")
}

func TestChangelog_RealGit_StayAtV0_WarningGoesToStdout(t *testing.T) {
	stayAtV0RealRepo(t)

	stdout, stderr, err := executeRootSeparateStreams("changelog", "--tag", "--no-push")
	require.NoErrorf(t, err, "stdout:\n%s\nstderr:\n%s", stdout, stderr)

	assert.Contains(t, stdout, "! major bump held back by versioning.bump.stay_at_v0: v1.0.0 → v0.69.0")
	assert.NotContains(t, stderr, "held back", "the pipeline writes its warning to cmd.OutOrStdout(), not stderr")
	tags, err := exec.Command("git", "tag", "-l", "v0.69.0").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(tags), "v0.69.0")
}

func TestChangelog_RealGit_StayAtV0_AllowMajorTagsMajor(t *testing.T) {
	stayAtV0RealRepo(t)

	out, err := executeRoot("changelog", "--tag", "--no-push", "--allow-major")
	require.NoErrorf(t, err, "output:\n%s", out)

	assert.NotContains(t, out, "held back")
	tags, err := exec.Command("git", "tag", "-l", "v1.0.0").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(tags), "v1.0.0")
}

const stayAtV0ReleaseConfig = `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  bump:
    stay_at_v0: true
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
`

func TestRelease_DryRun_StayAtV0_HoldsBackAndWarns(t *testing.T) {
	cfgPath := writeConfig(t, stayAtV0ReleaseConfig)
	fakeGitBreakingSinceV068(t)

	out, err := executeRoot("release", "--config", cfgPath, "--dry-run")
	require.NoError(t, err)

	assert.Contains(t, out, "v0.69.0")
	assert.Contains(t, out, "major bump held back by versioning.bump.stay_at_v0: v1.0.0 → v0.69.0")
}

func TestRelease_DryRun_StayAtV0_AllowMajorReleasesMajor(t *testing.T) {
	cfgPath := writeConfig(t, stayAtV0ReleaseConfig)
	fakeGitBreakingSinceV068(t)

	out, err := executeRoot("release", "--config", cfgPath, "--dry-run", "--allow-major")
	require.NoError(t, err)

	assert.Contains(t, out, "v1.0.0")
	assert.NotContains(t, out, "held back")
}
