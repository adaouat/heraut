package config

// The environment variables heraut reads from a CI job, and the token variables it asks the user to
// set, spelled once. Every other package refers to these rather than writing the name again, as it
// does for the forges' default web hosts (DefaultGitHubBaseURL and friends).
//
// Two token names per forge are deliberate and are not interchangeable: EnvGitHubToken is the
// variable GitHub Actions provides and forge detection reads, EnvGitHubCLIToken is the GitHub
// publish driver's first choice, named after the `gh` CLI's convention. The driver reads the
// enterprise names for GitHub Enterprise Server, as `gh` does (ADR-0077).
const (
	// GitHub Actions.
	EnvGitHubActions    = "GITHUB_ACTIONS"
	EnvGitHubServerURL  = "GITHUB_SERVER_URL"
	EnvGitHubAPIURL     = "GITHUB_API_URL"
	EnvGitHubRepository = "GITHUB_REPOSITORY"
	EnvGitHubRefName    = "GITHUB_REF_NAME"
	EnvGitHubRefType    = "GITHUB_REF_TYPE"
	EnvGitHubToken      = "GITHUB_TOKEN"
	EnvGitHubCLIToken   = "GH_TOKEN"

	EnvGitHubEnterpriseCLIToken = "GH_ENTERPRISE_TOKEN"
	EnvGitHubEnterpriseToken    = "GITHUB_ENTERPRISE_TOKEN"

	// GitLab CI.
	EnvGitLabCI           = "GITLAB_CI"
	EnvGitLabServerURL    = "CI_SERVER_URL"
	EnvGitLabAPIURL       = "CI_API_V4_URL"
	EnvGitLabProjectPath  = "CI_PROJECT_PATH"
	EnvGitLabProjectURL   = "CI_PROJECT_URL"
	EnvGitLabProjectID    = "CI_PROJECT_ID"
	EnvGitLabCommitBranch = "CI_COMMIT_BRANCH"
	EnvGitLabJobToken     = "CI_JOB_TOKEN"
	EnvGitLabToken        = "GITLAB_TOKEN"

	// Azure Pipelines.
	EnvAzureBuild          = "TF_BUILD"
	EnvAzureCollectionURI  = "SYSTEM_COLLECTIONURI"
	EnvAzureTeamProject    = "SYSTEM_TEAMPROJECT"
	EnvAzureRepositoryName = "BUILD_REPOSITORY_NAME"
	EnvAzureSourceBranch   = "BUILD_SOURCEBRANCH"
	EnvAzureAccessToken    = "SYSTEM_ACCESSTOKEN"
	EnvAzureDevOpsToken    = "AZURE_DEVOPS_TOKEN"
)

// CIEnvVars lists every variable above. Tests clear them all so a resolution depends only on what
// the test sets, not on the CI job the test happens to run in.
var CIEnvVars = []string{
	EnvGitHubActions, EnvGitHubServerURL, EnvGitHubAPIURL, EnvGitHubRepository, EnvGitHubRefName, EnvGitHubRefType, EnvGitHubToken, EnvGitHubCLIToken,
	EnvGitHubEnterpriseCLIToken, EnvGitHubEnterpriseToken,
	EnvGitLabCI, EnvGitLabServerURL, EnvGitLabAPIURL, EnvGitLabProjectPath, EnvGitLabProjectURL, EnvGitLabProjectID, EnvGitLabCommitBranch, EnvGitLabJobToken, EnvGitLabToken,
	EnvAzureBuild, EnvAzureCollectionURI, EnvAzureTeamProject, EnvAzureRepositoryName, EnvAzureSourceBranch, EnvAzureAccessToken, EnvAzureDevOpsToken,
}
