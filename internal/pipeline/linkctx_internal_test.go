package pipeline

import (
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

// TestAmbientLinkContext covers the ambient-CI host detection relocated from Tera into Go
// (T75 / ADR-0022). Env is fully pinned with t.Setenv so the result is deterministic even
// when the suite runs inside GitHub Actions (which sets GITHUB_SERVER_URL).
func TestAmbientLinkContext(t *testing.T) {
	tests := []struct {
		name     string
		ciURL    string // CI_PROJECT_URL
		ghServer string // GITHUB_SERVER_URL
		ghRepo   string // GITHUB_REPOSITORY
		want     *port.LinkContext
	}{
		{
			name:  "gitlab from CI_PROJECT_URL (full self-hosted root)",
			ciURL: "https://gitlab.example.com/grp/proj",
			want:  &port.LinkContext{BaseURL: "https://gitlab.example.com/grp/proj", Platform: "gitlab"},
		},
		{
			name:     "github from GITHUB_SERVER_URL + GITHUB_REPOSITORY",
			ghServer: "https://github.com",
			ghRepo:   "acme/widget",
			want:     &port.LinkContext{BaseURL: "https://github.com/acme/widget", Platform: "github"},
		},
		{
			name:     "github enterprise host",
			ghServer: "https://github.acme.com",
			ghRepo:   "acme/widget",
			want:     &port.LinkContext{BaseURL: "https://github.acme.com/acme/widget", Platform: "github"},
		},
		{
			name:     "CI_PROJECT_URL wins when both present",
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
			t.Setenv("GITHUB_SERVER_URL", tc.ghServer)
			t.Setenv("GITHUB_REPOSITORY", tc.ghRepo)
			assert.Equal(t, tc.want, ambientLinkContext())
		})
	}
}
