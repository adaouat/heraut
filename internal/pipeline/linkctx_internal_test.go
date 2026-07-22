package pipeline

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAmbientLinkContext covers the ambient-CI host detection relocated from Tera into Go
// (T75 / ADR-0022). Env is fully pinned with t.Setenv so the result is deterministic even
// when the suite runs inside GitHub Actions (which sets GITHUB_SERVER_URL).
func TestAmbientLinkContext(t *testing.T) {
	tests := []struct {
		name     string
		ciURL    string // CI_PROJECT_URL
		ciServer string // CI_SERVER_URL
		ciPath   string // CI_PROJECT_PATH
		ghServer string // GITHUB_SERVER_URL
		ghRepo   string // GITHUB_REPOSITORY
		want     *port.LinkContext
	}{
		{
			name:     "gitlab from CI_SERVER_URL + CI_PROJECT_PATH (owner/repo split, subgroup)",
			ciServer: "https://gitlab.example.com",
			ciPath:   "grp/sub/proj",
			want:     &port.LinkContext{BaseURL: "https://gitlab.example.com", Owner: "grp/sub", Repo: "proj", Platform: "gitlab"},
		},
		{
			name:  "gitlab from CI_PROJECT_URL only (fallback — links only, no enrichment)",
			ciURL: "https://gitlab.example.com/grp/proj",
			want:  &port.LinkContext{BaseURL: "https://gitlab.example.com/grp/proj", Platform: "gitlab"},
		},
		{
			name:     "github from GITHUB_SERVER_URL + GITHUB_REPOSITORY (owner/repo populated)",
			ghServer: "https://github.com",
			ghRepo:   "acme/widget",
			want:     &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
		},
		{
			name:     "github enterprise host",
			ghServer: "https://github.acme.com",
			ghRepo:   "acme/widget",
			want:     &port.LinkContext{BaseURL: "https://github.acme.com", Owner: "acme", Repo: "widget", Platform: "github"},
		},
		{
			name:     "gitlab split wins when github also present",
			ciServer: "https://gitlab.example.com",
			ciPath:   "grp/proj",
			ghServer: "https://github.com",
			ghRepo:   "acme/widget",
			want:     &port.LinkContext{BaseURL: "https://gitlab.example.com", Owner: "grp", Repo: "proj", Platform: "gitlab"},
		},
		{
			name:     "CI_PROJECT_URL fallback wins over github",
			ciURL:    "https://gitlab.example.com/grp/proj",
			ghServer: "https://github.com",
			ghRepo:   "acme/widget",
			want:     &port.LinkContext{BaseURL: "https://gitlab.example.com/grp/proj", Platform: "gitlab"},
		},
		{
			name: "no ambient host → nil",
			want: nil,
		},
		{
			name:     "github server without repository → nil",
			ghServer: "https://github.com",
			want:     nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CI_PROJECT_URL", tc.ciURL)
			t.Setenv("CI_SERVER_URL", tc.ciServer)
			t.Setenv("CI_PROJECT_PATH", tc.ciPath)
			t.Setenv("GITHUB_SERVER_URL", tc.ghServer)
			t.Setenv("GITHUB_REPOSITORY", tc.ghRepo)
			assert.Equal(t, tc.want, ambientLinkContext())
		})
	}
}

// TestRemoteLinkContext covers the explicit changelog.remote → port.LinkContext
// translation (ADR-0026): a generic, type-discriminated config block that never grants
// publish capability, unlike a release.platforms entry.
func TestRemoteLinkContext(t *testing.T) {
	tests := []struct {
		name     string
		r        *config.Remote
		tokenEnv string // env var to set for the token assertion, if any
		token    string
		want     *port.LinkContext
	}{
		{name: "nil remote returns nil", r: nil, want: nil},
		{
			name: "github",
			r:    &config.Remote{Type: "github", Repository: "acme/widget"},
			want: &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
		},
		{
			name:     "github reads default token env",
			r:        &config.Remote{Type: "github", Repository: "acme/widget"},
			tokenEnv: "GITHUB_TOKEN", token: "ghtok",
			want: &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github", Token: "ghtok"},
		},
		{
			name:     "github reads custom token_env override",
			r:        &config.Remote{Type: "github", Repository: "acme/widget", TokenEnv: "MY_GH_TOKEN"},
			tokenEnv: "MY_GH_TOKEN", token: "customtok",
			want: &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github", Token: "customtok"},
		},
		{
			name: "gitlab nested namespace",
			r:    &config.Remote{Type: "gitlab", Project: "group/sub/proj"},
			want: &port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group/sub", Repo: "proj", Platform: "gitlab"},
		},
		{
			name:     "gitlab reads default token env",
			r:        &config.Remote{Type: "gitlab", Project: "grp/proj"},
			tokenEnv: "GITLAB_TOKEN", token: "gltok",
			want: &port.LinkContext{BaseURL: "https://gitlab.com", Owner: "grp", Repo: "proj", Platform: "gitlab", Token: "gltok"},
		},
		{
			name: "azure_devops uses project as owner directly (already organization/project)",
			r:    &config.Remote{Type: "azure_devops", Project: "group1/sub-group", Repository: "myApp"},
			want: &port.LinkContext{BaseURL: "https://dev.azure.com", Owner: "group1/sub-group", Repo: "myApp", Platform: "azure_devops"},
		},
		{
			name:     "azure_devops reads default token env",
			r:        &config.Remote{Type: "azure_devops", Project: "group1/sub-group", Repository: "myApp"},
			tokenEnv: "AZURE_DEVOPS_TOKEN", token: "adotok",
			want: &port.LinkContext{BaseURL: "https://dev.azure.com", Owner: "group1/sub-group", Repo: "myApp", Platform: "azure_devops", Token: "adotok"},
		},
		{
			name: "azure_devops honours base_url override",
			r:    &config.Remote{Type: "azure_devops", Project: "group1/sub-group", Repository: "myApp", BaseURL: "https://devops.example.com"},
			want: &port.LinkContext{BaseURL: "https://devops.example.com", Owner: "group1/sub-group", Repo: "myApp", Platform: "azure_devops"},
		},
		{
			name: "github honours base_url (GHES)",
			r:    &config.Remote{Type: "github", Repository: "acme/widget", BaseURL: "https://github.acme.com"},
			want: &port.LinkContext{BaseURL: "https://github.acme.com", Owner: "acme", Repo: "widget", Platform: "github"},
		},
		{
			name: "gitlab honours base_url (self-managed, subgroup)",
			r:    &config.Remote{Type: "gitlab", Project: "internal-tools/ci-cd/catalog/release-notes", BaseURL: "https://git.cross-systems.ch"},
			want: &port.LinkContext{BaseURL: "https://git.cross-systems.ch", Owner: "internal-tools/ci-cd/catalog", Repo: "release-notes", Platform: "gitlab"},
		},
		{
			name: "base_url trailing slash trimmed",
			r:    &config.Remote{Type: "gitlab", Project: "grp/proj", BaseURL: "https://git.example.com/"},
			want: &port.LinkContext{BaseURL: "https://git.example.com", Owner: "grp", Repo: "proj", Platform: "gitlab"},
		},
		{
			name: "unrecognized type returns nil",
			r:    &config.Remote{Type: "bitbucket", Repository: "acme/widget"},
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.tokenEnv != "" {
				t.Setenv(tc.tokenEnv, tc.token)
			}
			assert.Equal(t, tc.want, remoteLinkContext(tc.r))
		})
	}
}

// TestChangelogLinkContext_ExplicitRemoteWinsOverAmbient confirms an explicit
// changelog.remote block takes priority over ambient CI detection (ADR-0026) —
// the existing ambient → single-platform → nil chain is otherwise unchanged.
func TestChangelogLinkContext_ExplicitRemoteWinsOverAmbient(t *testing.T) {
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example.com/grp/proj")
	t.Setenv("GITHUB_SERVER_URL", "")
	t.Setenv("GITHUB_REPOSITORY", "")

	p := &Pipeline{cfg: &Config{ChangelogRemote: &config.Remote{Type: "github", Repository: "acme/widget"}}}
	got := p.changelogLinkContext()
	require.NotNil(t, got)
	assert.Equal(t, "github", got.Platform)
	assert.Equal(t, "acme", got.Owner)
	assert.Equal(t, "widget", got.Repo)
}

// TestChangelogLinkContext_FallsThroughWithoutExplicitRemote confirms the existing
// ambient fallback is unchanged when no changelog.remote is configured.
func TestChangelogLinkContext_FallsThroughWithoutExplicitRemote(t *testing.T) {
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example.com/grp/proj")
	t.Setenv("GITHUB_SERVER_URL", "")
	t.Setenv("GITHUB_REPOSITORY", "")

	p := &Pipeline{cfg: &Config{}}
	got := p.changelogLinkContext()
	require.NotNil(t, got)
	assert.Equal(t, "gitlab", got.Platform)
}
