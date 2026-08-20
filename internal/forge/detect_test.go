package forge_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/forge"
	"github.com/stretchr/testify/assert"
)

func TestDetectForWizard_GitLabCI(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(map[string]string{
		"GITLAB_CI":       "true",
		"CI_PROJECT_PATH": "group/subgroup/project",
	}), "")
	assert.True(t, ok)
	assert.Equal(t, "gitlab", typ)
	assert.Equal(t, "group/subgroup/project", project)
}

func TestDetectForWizard_GitHubActions(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_REPOSITORY": "acme/widget",
	}), "")
	assert.True(t, ok)
	assert.Equal(t, "github", typ)
	assert.Equal(t, "acme/widget", project)
}

func TestDetectForWizard_AzureDevOpsUsesRepository(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(map[string]string{
		"TF_BUILD":              "true",
		"SYSTEM_TEAMPROJECT":    "myproject",
		"BUILD_REPOSITORY_NAME": "myrepo",
	}), "")
	assert.True(t, ok)
	assert.Equal(t, "azure_devops", typ)
	assert.Equal(t, "myproject", project)
}

func TestDetectForWizard_GitOriginFallback(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(nil), "git@gitlab.com:group/project.git")
	assert.True(t, ok)
	assert.Equal(t, "gitlab", typ)
	assert.Equal(t, "group/project", project)
}

// TestDetectForWizard_AmbientTokenAloneIsNotEnough pins that DetectForWizard, unlike
// forge.Resolve's zero-config path, never falls back to inspecting which ambient token env var
// happens to be set. The wizard always asks the user to pick (or confirm) a type explicitly when
// detection is inconclusive, rather than guessing from an ambient token.
func TestDetectForWizard_AmbientTokenAloneIsNotEnough(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(map[string]string{"GITHUB_TOKEN": "tok"}), "")
	assert.False(t, ok)
	assert.Equal(t, "", typ)
	assert.Equal(t, "", project)
}

func TestDetectForWizard_UnknownHost(t *testing.T) {
	typ, project, ok := forge.DetectForWizard(env(nil), "https://git.company.com/team/service.git")
	assert.False(t, ok)
	assert.Equal(t, "", typ)
	assert.Equal(t, "", project)
}
