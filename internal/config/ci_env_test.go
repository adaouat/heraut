package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

// The names are the wire contract with GitHub Actions, GitLab CI and Azure Pipelines, so a rename
// has to be a deliberate edit of this table, not a side effect.
func TestForgeEnvNames(t *testing.T) {
	tests := []struct{ got, want string }{
		{config.EnvGitHubActions, "GITHUB_ACTIONS"},
		{config.EnvGitHubServerURL, "GITHUB_SERVER_URL"},
		{config.EnvGitHubAPIURL, "GITHUB_API_URL"},
		{config.EnvGitHubRepository, "GITHUB_REPOSITORY"},
		{config.EnvGitHubRefName, "GITHUB_REF_NAME"},
		{config.EnvGitHubRefType, "GITHUB_REF_TYPE"},
		{config.EnvGitHubToken, "GITHUB_TOKEN"},
		{config.EnvGitHubCLIToken, "GH_TOKEN"},
		{config.EnvGitHubEnterpriseCLIToken, "GH_ENTERPRISE_TOKEN"},
		{config.EnvGitHubEnterpriseToken, "GITHUB_ENTERPRISE_TOKEN"},
		{config.EnvGitLabCI, "GITLAB_CI"},
		{config.EnvGitLabServerURL, "CI_SERVER_URL"},
		{config.EnvGitLabAPIURL, "CI_API_V4_URL"},
		{config.EnvGitLabProjectPath, "CI_PROJECT_PATH"},
		{config.EnvGitLabProjectURL, "CI_PROJECT_URL"},
		{config.EnvGitLabProjectID, "CI_PROJECT_ID"},
		{config.EnvGitLabCommitBranch, "CI_COMMIT_BRANCH"},
		{config.EnvGitLabJobToken, "CI_JOB_TOKEN"},
		{config.EnvGitLabToken, "GITLAB_TOKEN"},
		{config.EnvAzureBuild, "TF_BUILD"},
		{config.EnvAzureCollectionURI, "SYSTEM_COLLECTIONURI"},
		{config.EnvAzureTeamProject, "SYSTEM_TEAMPROJECT"},
		{config.EnvAzureRepositoryName, "BUILD_REPOSITORY_NAME"},
		{config.EnvAzureSourceBranch, "BUILD_SOURCEBRANCH"},
		{config.EnvAzureAccessToken, "SYSTEM_ACCESSTOKEN"},
		{config.EnvAzureDevOpsToken, "AZURE_DEVOPS_TOKEN"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, tc.got)
	}
}

// CIEnvVars is what tests clear to stop the ambient CI job from leaking into a resolution: every
// forge name above must be in it, once.
// HERAUT_FILE names the config file in CI; its value doubles as the label of the path source.
func TestEnvConfigFile(t *testing.T) {
	assert.Equal(t, "HERAUT_FILE", config.EnvConfigFile)
	assert.Equal(t, config.PathSource(config.EnvConfigFile), config.SourceEnvVar)
}

func TestCIEnvVars_ListsEveryForgeNameOnce(t *testing.T) {
	want := []string{
		"GITHUB_ACTIONS", "GITHUB_SERVER_URL", "GITHUB_API_URL", "GITHUB_REPOSITORY", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "GITHUB_TOKEN", "GH_TOKEN",
		"GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN",
		"GITLAB_CI", "CI_SERVER_URL", "CI_API_V4_URL", "CI_PROJECT_PATH", "CI_PROJECT_URL", "CI_PROJECT_ID", "CI_COMMIT_BRANCH", "CI_JOB_TOKEN", "GITLAB_TOKEN",
		"TF_BUILD", "SYSTEM_COLLECTIONURI", "SYSTEM_TEAMPROJECT", "BUILD_REPOSITORY_NAME", "BUILD_SOURCEBRANCH", "SYSTEM_ACCESSTOKEN", "AZURE_DEVOPS_TOKEN",
	}
	assert.ElementsMatch(t, want, config.CIEnvVars)
}
