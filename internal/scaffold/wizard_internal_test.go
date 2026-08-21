package scaffold

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSchemaURL(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"dev", "https://raw.githubusercontent.com/adaouat/heraut/main/schema.json"},
		{"", "https://raw.githubusercontent.com/adaouat/heraut/main/schema.json"},
		{"v1.2.3", "https://raw.githubusercontent.com/adaouat/heraut/v1.2.3/schema.json"},
		{"v2.0.0", "https://raw.githubusercontent.com/adaouat/heraut/v2.0.0/schema.json"},
	}
	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			assert.Equal(t, tc.want, buildSchemaURL(tc.version))
		})
	}
}

func TestResolveFormatChoice_Empty(t *testing.T) {
	choice, custom := resolveFormatChoice("")
	assert.Equal(t, calverPresets[0].format, choice)
	assert.Equal(t, "", custom)
}

func TestResolveFormatChoice_KnownPreset(t *testing.T) {
	choice, custom := resolveFormatChoice("YYYY.MM.PATCH")
	assert.Equal(t, "YYYY.MM.PATCH", choice)
	assert.Equal(t, "", custom)
}

func TestResolveFormatChoice_AllPresets(t *testing.T) {
	for _, p := range calverPresets {
		if p.format == "custom" {
			continue
		}
		choice, custom := resolveFormatChoice(p.format)
		assert.Equal(t, p.format, choice, "preset %s should round-trip", p.format)
		assert.Equal(t, "", custom)
	}
}

func TestResolveFormatChoice_CustomFormat(t *testing.T) {
	choice, custom := resolveFormatChoice("YYYY.MM.DD.WW.PATCH")
	assert.Equal(t, "custom", choice)
	assert.Equal(t, "YYYY.MM.DD.WW.PATCH", custom)
}

func TestParseRemoteProject(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"ssh with .git", "git@github.com:owner/repo.git", "owner/repo"},
		{"ssh without .git", "git@github.com:owner/repo", "owner/repo"},
		{"https with .git", "https://github.com/owner/repo.git", "owner/repo"},
		{"https without .git", "https://github.com/owner/repo", "owner/repo"},
		{"gitlab ssh", "git@gitlab.com:namespace/project.git", "namespace/project"},
		{"gitlab https", "https://gitlab.com/namespace/project.git", "namespace/project"},
		{"nested gitlab groups", "git@gitlab.com:namespace/group/project.git", "namespace/group/project"},
		{"self-hosted https", "https://git.company.com/team/service.git", "team/service"},
		{"ssh scheme", "ssh://git@gitlab.com/namespace/project.git", "namespace/project"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseRemoteProject(tc.url))
		})
	}
}

func TestDetectPlatform_GitLabCI(t *testing.T) {
	typ, project := detectPlatform(func(k string) string {
		m := map[string]string{"GITLAB_CI": "true", "CI_PROJECT_PATH": "group/project"}
		return m[k]
	}, "")
	assert.Equal(t, "gitlab", typ)
	assert.Equal(t, "group/project", project)
}

// TestDetectPlatform_SelfHostedFallsBackToAnyHostParsing pins that self-hosted GitLab/GitHub
// Enterprise remotes — which forge.DetectForWizard cannot type, since parseGitOrigin only
// recognizes github.com/gitlab.com/dev.azure.com — still get their project path pre-filled via
// parseRemoteProject's any-host parsing, exactly like today.
func TestDetectPlatform_SelfHostedFallsBackToAnyHostParsing(t *testing.T) {
	typ, project := detectPlatform(func(string) string { return "" }, "https://git.company.com/team/service.git")
	assert.Equal(t, "", typ)
	assert.Equal(t, "team/service", project)
}

// TestDetectPlatform_AzureDevOpsNotOfferedByWizardFallsBackToPathParsing pins that a detected
// azure_devops type — not one of the wizard's two platform Select options (gitlab/github) — is
// discarded, falling back to any-host path parsing instead of feeding an invalid type into the
// Select.
func TestDetectPlatform_AzureDevOpsNotOfferedByWizardFallsBackToPathParsing(t *testing.T) {
	typ, project := detectPlatform(func(k string) string {
		m := map[string]string{"TF_BUILD": "true", "SYSTEM_TEAMPROJECT": "myproject", "BUILD_REPOSITORY_NAME": "myrepo"}
		return m[k]
	}, "https://dev.azure.com/myorg/myproject/_git/myrepo")
	assert.Equal(t, "", typ)
	assert.Equal(t, "myorg/myproject/_git/myrepo", project)
}

func TestDetectPlatform_NoDetection(t *testing.T) {
	typ, project := detectPlatform(func(string) string { return "" }, "")
	assert.Equal(t, "", typ)
	assert.Equal(t, "", project)
}

func TestMatchPlatformSnapshot_SingleMatch(t *testing.T) {
	original := []PlatformAnswer{
		{Type: "github", Name: "gh-internal", BaseURL: "https://github.example.com", Draft: true, Prerelease: true},
	}
	rebuilt := []PlatformAnswer{
		{Type: "github", Repository: "org/repo", TokenEnv: "GH_TOKEN"},
	}
	result := matchPlatformSnapshot(original, rebuilt)
	require.Len(t, result, 1)
	assert.Equal(t, "gh-internal", result[0].Name)
	assert.Equal(t, "https://github.example.com", result[0].BaseURL)
	assert.True(t, result[0].Draft)
	assert.True(t, result[0].Prerelease)
	assert.Equal(t, "org/repo", result[0].Repository)
	assert.Equal(t, "GH_TOKEN", result[0].TokenEnv)
}

func TestMatchPlatformSnapshot_NoMatchForNewEntry(t *testing.T) {
	original := []PlatformAnswer{
		{Type: "github", Name: "gh-one", Draft: true},
	}
	rebuilt := []PlatformAnswer{
		{Type: "github", Repository: "org/repo-1"},
		{Type: "github", Repository: "org/repo-2"},
	}
	result := matchPlatformSnapshot(original, rebuilt)
	require.Len(t, result, 2)
	assert.Equal(t, "gh-one", result[0].Name)
	assert.True(t, result[0].Draft)
	assert.Equal(t, "", result[1].Name, "extra rebuilt entry has no snapshot match")
	assert.False(t, result[1].Draft)
}

func TestMatchPlatformSnapshot_TypeScoped(t *testing.T) {
	original := []PlatformAnswer{
		{Type: "github", Name: "gh-internal", BaseURL: "https://github.example.com"},
		{Type: "gitlab", Name: "gl-com", BaseURL: "https://gitlab.com"},
	}
	rebuilt := []PlatformAnswer{
		{Type: "github", Repository: "org/repo"},
		{Type: "gitlab", Project: "ns/project"},
	}
	result := matchPlatformSnapshot(original, rebuilt)
	require.Len(t, result, 2)
	assert.Equal(t, "gh-internal", result[0].Name)
	assert.Equal(t, "https://github.example.com", result[0].BaseURL)
	assert.Equal(t, "gl-com", result[1].Name)
	assert.Equal(t, "https://gitlab.com", result[1].BaseURL)
}

func TestMatchPlatformSnapshot_EmptyRebuilt(t *testing.T) {
	original := []PlatformAnswer{{Type: "github", Name: "gh-com", Draft: true}}
	result := matchPlatformSnapshot(original, nil)
	assert.Empty(t, result)
}

func TestMatchEnvSnapshot_SingleMatch(t *testing.T) {
	original := []EnvAnswer{
		{Name: "prod", Bump: "auto", Changelog: &config.ContentDriver{Output: "CHANGELOG.md"}},
	}
	rebuilt := []EnvAnswer{
		{Name: "prod", Bump: "auto"},
	}
	result := matchEnvSnapshot(original, rebuilt)
	require.Len(t, result, 1)
	assert.Equal(t, "prod", result[0].Name)
	require.NotNil(t, result[0].Changelog)
	assert.Equal(t, "CHANGELOG.md", result[0].Changelog.Output)
}

func TestMatchEnvSnapshot_NoMatchForRenamedEntry(t *testing.T) {
	original := []EnvAnswer{
		{Name: "prod", Bump: "auto", Changelog: &config.ContentDriver{Output: "CHANGELOG.md"}},
	}
	rebuilt := []EnvAnswer{
		{Name: "production", Bump: "auto"},
	}
	result := matchEnvSnapshot(original, rebuilt)
	require.Len(t, result, 1)
	assert.Equal(t, "production", result[0].Name)
	assert.Nil(t, result[0].Changelog, "renamed entry has no snapshot match")
}

func TestMatchEnvSnapshot_MultipleEnvs(t *testing.T) {
	driver := &config.ContentDriver{Output: "CHANGELOG.md"}
	rel := &config.EnvRelease{Notes: &config.ContentDriver{}}
	original := []EnvAnswer{
		{Name: "dev", Bump: "auto"},
		{Name: "prod", Bump: "promote", Changelog: driver, Release: rel},
	}
	rebuilt := []EnvAnswer{
		{Name: "dev", Bump: "auto"},
		{Name: "prod", Bump: "promote"},
	}
	result := matchEnvSnapshot(original, rebuilt)
	require.Len(t, result, 2)
	assert.Nil(t, result[0].Changelog)
	require.NotNil(t, result[1].Changelog)
	assert.Equal(t, "CHANGELOG.md", result[1].Changelog.Output)
	require.NotNil(t, result[1].Release)
}

func TestMatchEnvSnapshot_EmptyRebuilt(t *testing.T) {
	original := []EnvAnswer{
		{Name: "prod", Changelog: &config.ContentDriver{Output: "CHANGELOG.md"}},
	}
	result := matchEnvSnapshot(original, nil)
	assert.Empty(t, result)
}

func TestResolveTokenChoice(t *testing.T) {
	tests := []struct {
		name         string
		platformType string
		existing     string
		wantChoice   string
		wantCustom   string
	}{
		// empty → platform default pre-selected
		{"github default", "github", "", "GH_TOKEN", ""},
		{"gitlab default", "gitlab", "", "GITLAB_TOKEN", ""},
		{"unknown platform default", "other", "", "custom", ""},
		// known tokens round-trip
		{"GH_TOKEN known", "github", "GH_TOKEN", "GH_TOKEN", ""},
		{"GITLAB_TOKEN known", "gitlab", "GITLAB_TOKEN", "GITLAB_TOKEN", ""},
		{"CI_JOB_TOKEN known", "gitlab", "CI_JOB_TOKEN", "CI_JOB_TOKEN", ""},
		// custom values
		{"github custom token", "github", "MY_GITHUB_PAT", "custom", "MY_GITHUB_PAT"},
		{"gitlab custom token", "gitlab", "MY_GITLAB_TOKEN", "custom", "MY_GITLAB_TOKEN"},
		// wrong-platform token treated as custom
		{"GH_TOKEN on gitlab is custom", "gitlab", "GH_TOKEN", "custom", "GH_TOKEN"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			choice, custom := resolveTokenChoice(tc.platformType, tc.existing)
			assert.Equal(t, tc.wantChoice, choice)
			assert.Equal(t, tc.wantCustom, custom)
		})
	}
}

func TestPlatformDisplayNames(t *testing.T) {
	names := platformDisplayNames([]PlatformAnswer{
		{Type: "github"},
		{Type: "gitlab"},
		{Type: "gitlab"},
		{Type: "github", Name: "gh-internal"},
	})
	assert.Equal(t, []string{"github", "gitlab", "gitlab-2", "gh-internal"}, names)
}

func TestShouldPromptEnrichmentForge(t *testing.T) {
	assert.False(t, shouldPromptEnrichmentForge(nil))
	assert.False(t, shouldPromptEnrichmentForge([]PlatformAnswer{{Type: "github"}}))
	assert.True(t, shouldPromptEnrichmentForge([]PlatformAnswer{{Type: "github"}, {Type: "gitlab"}}))
}

// TestApplyPublishChoice_DeclinedClearsStalePlatforms pins the T199 bug fix: on the
// edit-existing-config path, ConfigToAnswers pre-populates a.Platforms from the on-disk config
// before RunWizard ever runs. If the user then declines "Publish releases?", a.Platforms must be
// cleared — otherwise the stale, pre-populated slice survives into GenerateYAML even though the
// user said no.
func TestApplyPublishChoice_DeclinedClearsStalePlatforms(t *testing.T) {
	a := &Answers{
		PublishReleases: false,
		Platforms: []PlatformAnswer{
			{Type: "github", Repository: "acme/widget", TokenEnv: "GH_TOKEN"},
		},
	}

	err := applyPublishChoice(a)

	require.NoError(t, err)
	assert.Empty(t, a.Platforms, "declining publish must clear stale pre-populated platforms")
}

func TestHideAPIMode(t *testing.T) {
	tests := []struct {
		name         string
		platformType string
		tokenChoice  string
		want         bool
	}{
		{"github always hidden", "github", "GH_TOKEN", true},
		{"gitlab with CI_JOB_TOKEN hidden", "gitlab", "CI_JOB_TOKEN", true},
		{"gitlab with GITLAB_TOKEN shown", "gitlab", "GITLAB_TOKEN", false},
		{"gitlab with custom token shown", "gitlab", "custom", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hideAPIMode(tc.platformType, tc.tokenChoice))
		})
	}
}
