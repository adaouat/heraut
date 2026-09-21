package cmd_test

import (
	"path/filepath"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingGit installs a git that fails every call, so a passing test proves the command never
// touched git history (no tag listing, no log walk) to compute its output.
func failingGit(t *testing.T) {
	t.Helper()
	exectest.FakeBin(t, "git", "#!/bin/sh\nexit 1\n")
}

const semverConfig = `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`

const manualSemverConfig = `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  bump:
    mode: manual
`

const perEnvBuildConfig = `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  uat:
    bump: auto
    tag_format: "{env}/{version}-{build}"
`

const branchGuardedPerEnvConfig = `
version: "1"
versioning:
  strategy: semver-per-env
environments:
  prod:
    bump: auto
    branch: main
    tag_format: "prod/{version}"
`

func TestVersionNext_OverrideFlags_Registered(t *testing.T) {
	root := cmd.NewRootCmd("v0.0.0-test")

	next, _, err := root.Find([]string{"version", "next"})
	require.NoError(t, err)
	for _, name := range []string{"set-version", "set-build-id"} {
		f := next.Flags().Lookup(name)
		require.NotNil(t, f, "version next has --%s", name)
		assert.Equal(t, "", f.DefValue)
		assert.Equal(t, "string", f.Value.Type())
	}

	current, _, err := root.Find([]string{"version", "current"})
	require.NoError(t, err)
	for _, name := range []string{"set-version", "set-build-id"} {
		assert.Nil(t, current.Flags().Lookup(name), "version current never renders an override: --%s", name)
	}
}

func TestVersionNext_SetVersion_PrintsTagWithoutGit(t *testing.T) {
	tests := []struct {
		name   string
		config string
		args   []string
		want   string
	}{
		{"bare version gets the default prefix", semverConfig, []string{"--set-version", "1.2.3"}, "v1.2.3\n"},
		{"prefixed version is not double-prefixed", semverConfig, []string{"--set-version", "v1.2.3"}, "v1.2.3\n"},
		{
			"custom tag_prefix",
			`
version: "1"
versioning:
  strategy: semver
  tag_prefix: "rel-"
`,
			[]string{"--set-version", "1.2.3"},
			"rel-1.2.3\n",
		},
		{"manual bump mode is satisfied", manualSemverConfig, []string{"--set-version", "0.2.0"}, "v0.2.0\n"},
		{
			"per-env tag_format with a build ID",
			perEnvBuildConfig,
			[]string{"--env", "uat", "--set-version", "0.2.0", "--set-build-id", "42"},
			"uat/0.2.0-42\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := writeConfig(t, tc.config)
			failingGit(t)

			args := append([]string{"version", "next", "--config", cfgPath}, tc.args...)
			stdout, stderr, err := executeRootSeparateStreams(args...)
			require.NoError(t, err)

			assert.Equal(t, tc.want, stdout, "stdout must be exactly the tag")
			assert.Empty(t, stderr)
		})
	}
}

func TestVersionNext_ManualMode_WithoutSetVersion_StillFails(t *testing.T) {
	cfgPath := writeConfig(t, manualSemverConfig)
	failingGit(t)

	stdout, _, err := executeRootSeparateStreams("version", "next", "--config", cfgPath)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "--set-version")
	assert.Equal(t, exitcode.Runtime, exitcode.Resolve(err))
	assert.Empty(t, stdout)
}

func TestVersionNext_BuildTagFormat_WithoutBuildID_ExplainsHowToSupplyOne(t *testing.T) {
	cfgPath := writeConfig(t, perEnvBuildConfig)
	failingGit(t)

	stdout, _, err := executeRootSeparateStreams("version", "next", "--config", cfgPath, "--env", "uat", "--set-version", "0.2.0")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "--set-build-id")
	assert.Contains(t, err.Error(), "version next")
	assert.Equal(t, exitcode.Config, exitcode.Resolve(err))
	assert.Empty(t, stdout)
}

func TestVersionNext_SetVersion_StillEnforcesBranchGuard(t *testing.T) {
	tests := []struct {
		name       string
		gitBranch  string
		extraArgs  []string
		wantTag    string
		wantErrMsg string
	}{
		{
			name:       "wrong branch is refused",
			gitBranch:  "feature/x",
			wantErrMsg: `must be operated from branch "main", but the current branch is "feature/x"`,
		},
		{
			name:      "matching branch prints the overridden tag",
			gitBranch: "main",
			wantTag:   "prod/1.2.3\n",
		},
		{
			name:      "force lets the wrong branch through",
			gitBranch: "feature/x",
			extraArgs: []string{"--force"},
			wantTag:   "prod/1.2.3\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := writeConfig(t, branchGuardedPerEnvConfig)
			exectest.FakeBin(t, "git", "#!/bin/sh\ncase \"$*\" in\n  \"rev-parse --abbrev-ref HEAD\") echo \""+tc.gitBranch+"\" ;;\n  *) exit 1 ;;\nesac\n")

			args := append([]string{"version", "next", "--config", cfgPath, "--env", "prod", "--set-version", "1.2.3"}, tc.extraArgs...)
			stdout, stderr, err := executeRootSeparateStreams(args...)

			if tc.wantErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrMsg)
				assert.Equal(t, exitcode.Runtime, exitcode.Resolve(err))
				assert.Empty(t, stdout)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, stdout, "stdout must be exactly the tag")
			assert.Empty(t, stderr)
		})
	}
}

func TestVersionNext_OverrideFlagValidation_FailsBeforeConfigIsRead(t *testing.T) {
	missingConfig := filepath.Join(t.TempDir(), "does-not-exist.yml")

	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"build ID without version", []string{"--set-build-id", "42"}, "--set-build-id requires --set-version"},
		{"version with whitespace", []string{"--set-version", "1.2 3"}, "must not contain whitespace"},
		{"build ID with slash", []string{"--set-version", "1.2.3", "--set-build-id", "a/b"}, "must not contain '/'"},
		{"build ID with whitespace", []string{"--set-version", "1.2.3", "--set-build-id", "a b"}, "must not contain whitespace"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"version", "next", "--config", missingConfig}, tc.args...)
			stdout, _, err := executeRootSeparateStreams(args...)
			require.Error(t, err)

			assert.Contains(t, err.Error(), tc.wantMsg, "must be the flag validation error, not the missing-config error")
			assert.Equal(t, exitcode.Config, exitcode.Resolve(err))
			assert.Empty(t, stdout)
		})
	}
}

func TestVersionNext_SetVersion_AllowMajorIsANoOp(t *testing.T) {
	cfgPath := writeConfig(t, stayAtV0Config)
	failingGit(t)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath, "--set-version", "1.2.3", "--allow-major")
	require.NoError(t, err)

	assert.Equal(t, "v1.2.3\n", stdout)
	assert.Empty(t, stderr, "a static version is never held back, so nothing is warned about")
}
