package cmd_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const buildIDReleaseConfig = `
version: "1"
versioning:
  strategy: semver-per-env
forges:
  - name: github
    platform: github
    repository: test/repo
release:
  targets:
    - forge: github
environments:
  uat:
    bump: auto
    tag_format: "{env}/{version}-{build}"
`

// buildIDAutoGit answers exactly the git calls a per-env auto resolution makes, so a run reaches
// the tag-format rendering step, which is where the missing build ID surfaces.
func buildIDAutoGit(t *testing.T) {
	t.Helper()
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l uat/*-* --sort=-version:refname") echo "uat/0.1.0-1" ;;
  "log uat/0.1.0-1..HEAD --format=%B"*) printf "fix: x\x00" ;;
  *) exit 1 ;;
esac
`)
}

func TestMissingBuildID_IsConfigError_OnEveryPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		// autoGit installs a git that answers the auto-resolution calls; the explicit-override
		// path never touches git history, so it gets a git that fails every call.
		autoGit bool
	}{
		{"version next, auto path", []string{"version", "next", "--env", "uat"}, true},
		{"changelog, auto path", []string{"changelog", "--env", "uat", "--dry-run"}, true},
		{"release, auto path", []string{"release", "--env", "uat", "--dry-run"}, true},
		{"version next, explicit --set-version", []string{"version", "next", "--env", "uat", "--set-version", "0.2.0"}, false},
		{"changelog, explicit --set-version", []string{"changelog", "--env", "uat", "--dry-run", "--set-version", "0.2.0"}, false},
		{"release, explicit --set-version", []string{"release", "--env", "uat", "--dry-run", "--set-version", "0.2.0"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := writeConfig(t, buildIDReleaseConfig)
			if tc.autoGit {
				buildIDAutoGit(t)
			} else {
				failingGit(t)
			}

			args := append(append([]string{}, tc.args...), "--config", cfgPath)
			stdout, _, err := executeRootSeparateStreams(args...)
			require.Error(t, err)

			assert.Equal(t, exitcode.Config, cmd.ExitCode(err))
			// A pipeline failure shows its detail in the step output and returns only a short
			// summary; either way the user must be told how to supply a build ID.
			assert.Contains(t, stdout+err.Error(), "--set-build-id")
		})
	}
}
