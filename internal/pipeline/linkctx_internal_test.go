package pipeline

import (
	"testing"

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

// TestChangelogLinkContext_FallsThroughWithoutExplicitRemote confirms the existing
// ambient fallback is unchanged when no forge identity is resolved.
func TestChangelogLinkContext_FallsThroughWithoutExplicitRemote(t *testing.T) {
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example.com/grp/proj")
	t.Setenv("GITHUB_SERVER_URL", "")
	t.Setenv("GITHUB_REPOSITORY", "")

	p := &Pipeline{cfg: &Config{}}
	got := p.changelogLinkContext()
	require.NotNil(t, got)
	assert.Equal(t, "gitlab", got.Platform)
}

// TestLinkContextFromIdentity covers the resolved-forge → LinkContext translation (ADR-0043):
// links resolve from the same source as enrichment.
func TestLinkContextFromIdentity(t *testing.T) {
	tests := []struct {
		name string
		id   port.ForgeIdentity
		want *port.LinkContext
	}{
		{name: "zero identity → nil", id: port.ForgeIdentity{}, want: nil},
		{name: "missing type → nil", id: port.ForgeIdentity{Host: "https://gitlab.example.com", Project: "group/proj"}, want: nil},
		{name: "missing host → nil", id: port.ForgeIdentity{Type: "gitlab", Project: "group/proj"}, want: nil},
		{
			name: "gitlab nested subgroup",
			id:   port.ForgeIdentity{Type: "gitlab", Host: "https://gitlab.example.com", Project: "group/subgroup/project", Token: "tok"},
			want: &port.LinkContext{BaseURL: "https://gitlab.example.com", Owner: "group/subgroup", Repo: "project", Platform: "gitlab", Token: "tok"},
		},
		{
			name: "host trailing slash trimmed",
			id:   port.ForgeIdentity{Type: "github", Host: "https://github.com/", Project: "acme/widget"},
			want: &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, linkContextFromIdentity(tc.id))
		})
	}
}

// TestChangelogLinkContext_ForgeIdentityWinsOverAmbient confirms a resolved forge identity
// (ADR-0043) takes priority over ambient CI detection.
func TestChangelogLinkContext_ForgeIdentityWinsOverAmbient(t *testing.T) {
	t.Setenv("CI_PROJECT_URL", "https://gitlab.example.com/grp/proj")
	t.Setenv("GITHUB_SERVER_URL", "")
	t.Setenv("GITHUB_REPOSITORY", "")

	id := port.ForgeIdentity{Type: "gitlab", Host: "https://gitlab.example.com", Project: "group/subgroup/project"}
	p := &Pipeline{cfg: &Config{ForgeIdentity: &id}}
	got := p.changelogLinkContext()
	require.NotNil(t, got)
	assert.Equal(t, "gitlab", got.Platform)
	assert.Equal(t, "group/subgroup", got.Owner)
	assert.Equal(t, "project", got.Repo)
}

// TestChangelogPipelineLinkContext covers the standalone `heraut changelog` flow: it resolves
// the same precedence chain as the release pipeline (resolved forge → ambient), so a forge
// resolved from the git origin renders links outside CI instead of bare hashes.
func TestChangelogPipelineLinkContext(t *testing.T) {
	forgeID := port.ForgeIdentity{Type: "gitlab", Host: "https://gitlab.example.com", Project: "group/subgroup/project"}

	tests := []struct {
		name string
		cfg  *ChangelogConfig
		amb  string // CI_PROJECT_URL, when the ambient fallback should apply
		want *port.LinkContext
	}{
		{
			name: "forge identity used when nothing else is configured",
			cfg:  &ChangelogConfig{ForgeIdentity: &forgeID},
			want: &port.LinkContext{BaseURL: "https://gitlab.example.com", Owner: "group/subgroup", Repo: "project", Platform: "gitlab"},
		},
		{
			name: "forge identity wins over ambient CI",
			cfg:  &ChangelogConfig{ForgeIdentity: &forgeID},
			amb:  "https://gitlab.example.com/other/proj",
			want: &port.LinkContext{BaseURL: "https://gitlab.example.com", Owner: "group/subgroup", Repo: "project", Platform: "gitlab"},
		},
		{
			name: "falls back to ambient when no forge is resolved",
			cfg:  &ChangelogConfig{},
			amb:  "https://gitlab.example.com/grp/proj",
			want: &port.LinkContext{BaseURL: "https://gitlab.example.com/grp/proj", Platform: "gitlab"},
		},
		{
			name: "nil when nothing resolves",
			cfg:  &ChangelogConfig{},
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CI_PROJECT_URL", tc.amb)
			t.Setenv("CI_SERVER_URL", "")
			t.Setenv("CI_PROJECT_PATH", "")
			t.Setenv("GITHUB_SERVER_URL", "")
			t.Setenv("GITHUB_REPOSITORY", "")

			p := &ChangelogPipeline{cfg: tc.cfg}
			assert.Equal(t, tc.want, p.changelogLinkContext())
		})
	}
}
