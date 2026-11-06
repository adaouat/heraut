package forge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/port"
)

// Pull-request enrichment reads GitHub's token the way publishing does: GH_TOKEN, then
// GITHUB_TOKEN, or the enterprise names on GitHub Enterprise Server.
func TestResolve_GitHubTokenChain(t *testing.T) {
	const ghes = "https://github.example.com"
	explicit := func(baseURL string) *config.Config {
		return &config.Config{Forges: []config.Forge{{Name: "gh", Type: "github", Repository: "acme/app", BaseURL: baseURL}}}
	}
	tests := []struct {
		name      string
		cfg       *config.Config
		env       map[string]string
		origin    string
		wantToken string
		wantKind  port.TokenKind
	}{
		{name: "explicit forge, GH_TOKEN only", cfg: explicit(""), env: map[string]string{"GH_TOKEN": "gh"},
			wantToken: "gh", wantKind: port.TokenPrivate},
		{name: "explicit forge, GITHUB_TOKEN only", cfg: explicit(""), env: map[string]string{"GITHUB_TOKEN": "gt"},
			wantToken: "gt", wantKind: port.TokenPrivate},
		{name: "explicit forge, GH_TOKEN first", cfg: explicit(""), env: map[string]string{"GH_TOKEN": "gh", "GITHUB_TOKEN": "gt"},
			wantToken: "gh", wantKind: port.TokenPrivate},
		{name: "GitHub Enterprise Server, enterprise names first", cfg: explicit(ghes),
			env:       map[string]string{"GH_ENTERPRISE_TOKEN": "e1", "GITHUB_ENTERPRISE_TOKEN": "e2", "GH_TOKEN": "gh"},
			wantToken: "e1", wantKind: port.TokenPrivate},
		{name: "GitHub Enterprise Server, then GH_TOKEN", cfg: explicit(ghes), env: map[string]string{"GH_TOKEN": "gh"},
			wantToken: "gh", wantKind: port.TokenPrivate},
		{name: "GitHub Enterprise Server ignores GITHUB_TOKEN", cfg: explicit(ghes), env: map[string]string{"GITHUB_TOKEN": "gt"},
			wantKind: port.TokenNone},
		{name: "token_env still wins", cfg: &config.Config{Forges: []config.Forge{{Name: "gh", Type: "github", Repository: "acme/app", TokenEnv: "MY_TOKEN"}}},
			env: map[string]string{"MY_TOKEN": "mine", "GH_TOKEN": "gh"}, wantToken: "mine", wantKind: port.TokenPrivate},
		{name: "github.com origin, GH_TOKEN only", cfg: &config.Config{}, env: map[string]string{"GH_TOKEN": "gh"},
			origin: "git@github.com:acme/app.git", wantToken: "gh", wantKind: port.TokenPrivate},
		{name: "github.com origin, GITHUB_TOKEN only", cfg: &config.Config{}, env: map[string]string{"GITHUB_TOKEN": "gt"},
			origin: "git@github.com:acme/app.git", wantToken: "gt", wantKind: port.TokenPrivate},
		{name: "explicit forge in GitHub Actions keeps the job's GITHUB_TOKEN first", cfg: explicit(""),
			env:       map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "acme/app", "GITHUB_TOKEN": "job", "GH_TOKEN": "gh"},
			wantToken: "job", wantKind: port.TokenPrivate},
		{name: "GitHub Actions keeps the job's GITHUB_TOKEN first", cfg: &config.Config{},
			env:       map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "acme/app", "GITHUB_TOKEN": "job", "GH_TOKEN": "gh"},
			wantToken: "job", wantKind: port.TokenPrivate},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := forge.Resolve(tc.cfg, env(tc.env), tc.origin)
			require.NoError(t, err)
			require.Len(t, got.Forges, 1)
			assert.Equal(t, tc.wantToken, got.Forges[0].Token)
			assert.Equal(t, tc.wantKind, got.Forges[0].TokenKind)
		})
	}
}

// GH_TOKEN never picks the forge on its own: with no forges:, CI or recognized origin, only the
// three default variables count, so GH_TOKEN next to GITLAB_TOKEN is not ambiguous.
func TestResolve_GHTokenIsNotADetectionCandidate(t *testing.T) {
	got, err := forge.Resolve(&config.Config{}, env(map[string]string{"GH_TOKEN": "gh", "GITLAB_TOKEN": "gl"}), "")
	require.NoError(t, err)
	require.Len(t, got.Forges, 1)
	assert.Equal(t, "gitlab", got.Forges[0].Type)

	got, err = forge.Resolve(&config.Config{}, env(map[string]string{"GH_TOKEN": "gh"}), "")
	require.NoError(t, err)
	assert.Empty(t, got.Forges, "GH_TOKEN alone detects nothing")
}
