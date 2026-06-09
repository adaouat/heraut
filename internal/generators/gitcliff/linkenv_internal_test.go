package gitcliff

import (
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

// TestLinkEnv is the testability payoff of T75: the per-platform URL-prefix shapes that
// used to live as untestable Tera branching now resolve in Go and are asserted exactly.
func TestLinkEnv(t *testing.T) {
	tests := []struct {
		name string
		lc   port.LinkContext
		want []string
	}{
		{
			name: "github",
			lc:   port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
			want: []string{
				"HERAUT_REMOTE_URL=https://github.com/acme/widget",
				"HERAUT_PLATFORM=github",
				"HERAUT_COMMIT_URL=https://github.com/acme/widget/commit/",
				"HERAUT_PR_URL=https://github.com/acme/widget/pull/",
				"HERAUT_PR_LABEL=#",
				"HERAUT_COMPARE_URL=https://github.com/acme/widget/compare/",
			},
		},
		{
			name: "gitlab nested namespace routes under /-/",
			lc:   port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group/sub", Repo: "proj", Platform: "gitlab"},
			want: []string{
				"HERAUT_REMOTE_URL=https://gitlab.com/group/sub/proj",
				"HERAUT_PLATFORM=gitlab",
				"HERAUT_COMMIT_URL=https://gitlab.com/group/sub/proj/-/commit/",
				"HERAUT_PR_URL=https://gitlab.com/group/sub/proj/-/merge_requests/",
				"HERAUT_PR_LABEL=!",
				"HERAUT_COMPARE_URL=https://gitlab.com/group/sub/proj/-/compare/",
			},
		},
		{
			// Ambient-resolved context: full root in BaseURL, empty Owner/Repo (see the
			// ambient resolver). linkEnv composes the same {remote} with no URL-splitting.
			name: "gitlab ambient full-root, empty owner/repo",
			lc:   port.LinkContext{BaseURL: "https://gitlab.example.com/grp/proj", Platform: "gitlab"},
			want: []string{
				"HERAUT_REMOTE_URL=https://gitlab.example.com/grp/proj",
				"HERAUT_PLATFORM=gitlab",
				"HERAUT_COMMIT_URL=https://gitlab.example.com/grp/proj/-/commit/",
				"HERAUT_PR_URL=https://gitlab.example.com/grp/proj/-/merge_requests/",
				"HERAUT_PR_LABEL=!",
				"HERAUT_COMPARE_URL=https://gitlab.example.com/grp/proj/-/compare/",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, linkEnv(&tc.lc))
		})
	}
}
