package app_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
)

// clearBranchEnv blanks the CI variables CurrentBranch falls back to on a detached HEAD, so a test
// sees only the git answer it queued.
func clearBranchEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CI_COMMIT_BRANCH", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "BUILD_SOURCEBRANCH", "BUILD_SOURCEBRANCHNAME"} {
		t.Setenv(k, "")
	}
}

func branchCfg(env, branch string) *config.Config {
	return &config.Config{
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			env: {Bump: "auto", Branch: branch},
		},
	}
}
