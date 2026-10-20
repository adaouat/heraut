package testutil_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/testutil"
)

// Every variable production code reads from a CI job must be cleared, or a test run inside a CI job
// (this repository's own) resolves differently from one run locally.
func TestClearCIEnv_ClearsEveryVariableProductionCodeReads(t *testing.T) {
	read := []string{
		"GITHUB_ACTIONS", "GITHUB_SERVER_URL", "GITHUB_API_URL", "GITHUB_REPOSITORY", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "GITHUB_TOKEN", "GH_TOKEN",
		"GITLAB_CI", "CI_SERVER_URL", "CI_API_V4_URL", "CI_PROJECT_PATH", "CI_PROJECT_URL", "CI_PROJECT_ID", "CI_COMMIT_BRANCH", "CI_JOB_TOKEN", "GITLAB_TOKEN",
		"TF_BUILD", "SYSTEM_COLLECTIONURI", "SYSTEM_TEAMPROJECT", "BUILD_REPOSITORY_NAME", "SYSTEM_ACCESSTOKEN", "AZURE_DEVOPS_TOKEN",
	}
	for _, name := range read {
		t.Setenv(name, "leaked")
	}

	testutil.ClearCIEnv(t)

	for _, name := range read {
		assert.Empty(t, os.Getenv(name), name)
	}
}
