package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The choices the platform wizard's questions make, kept apart from the huh forms (which need a
// terminal) so each can be pinned here.

func TestApplyPlatformSnapshot_CarriesEveryPassthroughFieldAndNothingElse(t *testing.T) {
	p := PlatformAnswer{Type: "gitlab", Project: "acme/widget"}
	orig := PlatformAnswer{
		Type: "gitlab", Project: "old/project", Repository: "old/repo",
		Name: "corp", BaseURL: "https://git.example.com", Draft: true, TokenEnv: "CI_JOB_TOKEN", APIMode: "graphql",
	}

	applyPlatformSnapshot(&p, orig)

	assert.Equal(t, PlatformAnswer{
		Type: "gitlab", Project: "acme/widget",
		Name: "corp", BaseURL: "https://git.example.com", Draft: true, TokenEnv: "CI_JOB_TOKEN", APIMode: "graphql",
	}, p, "type, project and repository stay as asked; only the five carried fields are copied")
}

func TestPrefillPlatformLocation(t *testing.T) {
	tests := []struct {
		name string
		in   PlatformAnswer
		want PlatformAnswer
	}{
		{"github gets the detected repository", PlatformAnswer{Type: "github"}, PlatformAnswer{Type: "github", Repository: "acme/widget"}},
		{"gitlab gets the detected project", PlatformAnswer{Type: "gitlab"}, PlatformAnswer{Type: "gitlab", Project: "acme/widget"}},
		{"a value already set is kept", PlatformAnswer{Type: "github", Repository: "mine/repo"}, PlatformAnswer{Type: "github", Repository: "mine/repo"}},
		{"another type is left alone", PlatformAnswer{Type: "azure_devops"}, PlatformAnswer{Type: "azure_devops"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.in
			prefillPlatformLocation(&p, "acme/widget")
			assert.Equal(t, tc.want, p)
		})
	}
}

func TestTokenSelectOptions_KnownNamesThenCustom(t *testing.T) {
	assert.Equal(t, []string{"GH_TOKEN", "custom"}, tokenSelectOptions("github"))
	assert.Equal(t, []string{"GITLAB_TOKEN", "CI_JOB_TOKEN", "custom"}, tokenSelectOptions("gitlab"))
	assert.Equal(t, []string{"custom"}, tokenSelectOptions("other"))
}

func TestAPIModeOrDefault(t *testing.T) {
	assert.Equal(t, "rest", apiModeOrDefault(""))
	assert.Equal(t, "graphql", apiModeOrDefault("graphql"))
	assert.Equal(t, "rest", apiModeOrDefault("rest"))
}

func TestRequiredPlatformFields(t *testing.T) {
	require.Error(t, requireGitHubRepository(""))
	require.Error(t, requireGitHubRepository("   "))
	assert.ErrorContains(t, requireGitHubRepository(""), "repository is required for GitHub")
	assert.NoError(t, requireGitHubRepository("acme/widget"))

	require.Error(t, requireTokenEnv(""))
	assert.ErrorContains(t, requireTokenEnv(" "), "token env var is required")
	assert.NoError(t, requireTokenEnv("MY_TOKEN"))
}
