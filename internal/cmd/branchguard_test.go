package cmd_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/testutil"
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
		if err != nil {
			assert.NotContains(t, err.Error(), "pass --force to release anyway")
		}
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
