package testutil

import "testing"

// ClearCIEnv neutralizes every CI marker forge detection keys off (internal/forge/detect.go),
// so a test's resolution outcome depends only on the variables it explicitly sets rather than the
// ambient environment the test happens to run in (e.g. this repo's own GitHub Actions CI).
func ClearCIEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"GITHUB_ACTIONS", "GITHUB_SERVER_URL", "GITHUB_API_URL", "GITHUB_REPOSITORY", "GITHUB_TOKEN",
		"GITLAB_CI", "CI_SERVER_URL", "CI_API_V4_URL", "CI_PROJECT_PATH", "CI_JOB_TOKEN", "GITLAB_TOKEN",
		"TF_BUILD", "SYSTEM_COLLECTIONURI", "SYSTEM_TEAMPROJECT", "SYSTEM_ACCESSTOKEN", "AZURE_DEVOPS_TOKEN",
	} {
		t.Setenv(k, "")
	}
}
