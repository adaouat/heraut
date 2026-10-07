package cmd_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const branchListedConfig = `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  branches:
    - name: main
    - name: release/*
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
`

func branchGuardGit(t *testing.T, branch string) {
	t.Helper()
	testutil.ClearCIEnv(t)
	for _, k := range []string{"CI_COMMIT_BRANCH", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "BUILD_SOURCEBRANCH", "BUILD_SOURCEBRANCHNAME"} {
		t.Setenv(k, "")
	}
	exectest.FakeBin(t, "git", "#!/bin/sh\ncase \"$*\" in\n  \"rev-parse --abbrev-ref HEAD\") echo \""+branch+"\" ;;\n  *) exit 1 ;;\nesac\n")
}

func TestRelease_UnlistedBranch_IsRefused(t *testing.T) {
	cfgPath := writeConfig(t, branchListedConfig)
	branchGuardGit(t, "feature/x")

	_, err := executeRoot("release", "--config", cfgPath, "--set-version", "1.2.3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `branch "feature/x" (pass --force to release anyway)`)
	assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
}

func TestRelease_UnlistedBranch_ForceAndDryRunPassTheGuard(t *testing.T) {
	for _, flag := range []string{"--force", "--dry-run"} {
		t.Run(flag, func(t *testing.T) {
			cfgPath := writeConfig(t, branchListedConfig)
			branchGuardGit(t, "feature/x")

			_, err := executeRoot("release", "--config", cfgPath, "--set-version", "1.2.3", flag)
			if err != nil {
				assert.NotContains(t, err.Error(), "pass --force to release anyway")
			}
		})
	}
}

func TestChangelog_UnlistedBranch_OnlyTagIsRefused(t *testing.T) {
	t.Run("tag is refused", func(t *testing.T) {
		cfgPath := writeConfig(t, branchListedConfig)
		branchGuardGit(t, "feature/x")

		_, err := executeRoot("changelog", "--config", cfgPath, "--set-version", "1.2.3", "--tag")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pass --force to release anyway")
		assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
	})
	t.Run("no tag is not refused", func(t *testing.T) {
		cfgPath := writeConfig(t, branchListedConfig)
		branchGuardGit(t, "feature/x")

		_, err := executeRoot("changelog", "--config", cfgPath, "--set-version", "1.2.3")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "pass --force to release anyway")
		assert.NotErrorIs(t, err, semver.ErrTagExists)
		assert.Contains(t, err.Error(), "preflight check failed", "the stub git stops the run at preflight, past the branch guard")
	})
}

// A taken version is a collision only when the run would tag it: re-rendering the changelog of an
// already-released version (changelog without --tag) must not run the collision probe.
func TestChangelog_SetVersion_CollisionProbeOnlyWhenTagging(t *testing.T) {
	takenGit := func(t *testing.T) {
		t.Helper()
		testutil.ClearCIEnv(t)
		for _, k := range []string{"CI_COMMIT_BRANCH", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "BUILD_SOURCEBRANCH", "BUILD_SOURCEBRANCHNAME"} {
			t.Setenv(k, "")
		}
		exectest.FakeBin(t, "git", "#!/bin/sh\ncase \"$*\" in\n  \"rev-parse --abbrev-ref HEAD\") echo main ;;\n  \"tag -l v1.2.3 v1.2.3+*\") echo v1.2.3 ;;\n  *) exit 1 ;;\nesac\n")
	}

	t.Run("without --tag the taken version is not a collision", func(t *testing.T) {
		cfgPath := writeConfig(t, branchListedConfig)
		takenGit(t)

		_, err := executeRoot("changelog", "--config", cfgPath, "--set-version", "1.2.3", "--dry-run")
		if err != nil {
			assert.NotErrorIs(t, err, semver.ErrTagExists)
		}
	})
	t.Run("with --tag the taken version is refused", func(t *testing.T) {
		cfgPath := writeConfig(t, branchListedConfig)
		takenGit(t)

		_, err := executeRoot("changelog", "--config", cfgPath, "--set-version", "1.2.3", "--tag", "--dry-run")
		require.ErrorIs(t, err, semver.ErrTagExists)
	})
}

// Branch-rule errors are configuration problems (ADR-0065), so version current exits Config for
// them just as the resolver-building commands do.
func TestVersionCurrent_BranchRuleErrors_ExitConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		branch  string
		wantErr string
	}{
		{
			name: "ambiguous match",
			config: `
version: "1"
versioning:
  strategy: semver
  branches:
    - name: main
    - name: release/*
    - name: release/1.3
      range: 1.3.x
`,
			branch:  "release/1.3",
			wantErr: "matches more than one",
		},
		{
			name:    "underivable range",
			config:  branchListedConfig,
			branch:  "release/legacy",
			wantErr: "cannot derive a maintenance range",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := writeConfig(t, tc.config)
			branchGuardGit(t, tc.branch)

			_, err := executeRoot("version", "current", "--config", cfgPath)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
		})
	}
}

// A --set-version already released elsewhere is a runtime condition, like the auto path's
// collision (ADR-0065), so it exits Runtime rather than Config.
func TestVersionNext_SetVersionCollision_ExitsRuntime(t *testing.T) {
	cfgPath := writeConfig(t, branchListedConfig)
	testutil.ClearCIEnv(t)
	for _, k := range []string{"CI_COMMIT_BRANCH", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "BUILD_SOURCEBRANCH", "BUILD_SOURCEBRANCHNAME"} {
		t.Setenv(k, "")
	}
	exectest.FakeBin(t, "git", "#!/bin/sh\ncase \"$*\" in\n  \"tag -l v1.3.2 v1.3.2+*\") echo \"v1.3.2+7\" ;;\n  *) exit 1 ;;\nesac\n")

	_, err := executeRoot("version", "next", "--config", cfgPath, "--set-version", "1.3.2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag already exists: v1.3.2+7")
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
}
