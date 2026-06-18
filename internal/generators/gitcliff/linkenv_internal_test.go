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
				"GITHUB_REPO=acme/widget",
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
				"GITLAB_REPO=group/sub/proj",
			},
		},
		{
			// Ambient-resolved context: full root in BaseURL, empty Owner/Repo (see the
			// ambient resolver). linkEnv composes the same {remote} with no URL-splitting.
			// No GITHUB_REPO / GITLAB_REPO injected — ambient contexts have no split owner/repo.
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
			t.Setenv("GITHUB_TOKEN", "")
			t.Setenv("GITLAB_TOKEN", "")
			t.Setenv("GITHUB_REPO", "")
			t.Setenv("GITLAB_REPO", "")
			assert.Equal(t, tc.want, linkEnv(&tc.lc))
		})
	}
}

func TestLinkEnv_TokenInjection(t *testing.T) {
	tests := []struct {
		name          string
		lc            port.LinkContext
		envOverride   map[string]string
		wantTokenPair string // "KEY=val" expected in result, or "" if not expected
	}{
		{
			name:          "github token injected when GITHUB_TOKEN not set",
			lc:            port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "w", Platform: "github", Token: "mytoken"},
			envOverride:   map[string]string{"GITHUB_TOKEN": ""},
			wantTokenPair: "GITHUB_TOKEN=mytoken",
		},
		{
			name:          "github token not injected when GITHUB_TOKEN already set",
			lc:            port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "w", Platform: "github", Token: "mytoken"},
			envOverride:   map[string]string{"GITHUB_TOKEN": "existing"},
			wantTokenPair: "",
		},
		{
			name:          "gitlab token injected when GITLAB_TOKEN not set",
			lc:            port.LinkContext{BaseURL: "https://gitlab.com", Owner: "grp", Repo: "p", Platform: "gitlab", Token: "gltoken"},
			envOverride:   map[string]string{"GITLAB_TOKEN": ""},
			wantTokenPair: "GITLAB_TOKEN=gltoken",
		},
		{
			name:          "gitlab token not injected when GITLAB_TOKEN already set",
			lc:            port.LinkContext{BaseURL: "https://gitlab.com", Owner: "grp", Repo: "p", Platform: "gitlab", Token: "gltoken"},
			envOverride:   map[string]string{"GITLAB_TOKEN": "existing"},
			wantTokenPair: "",
		},
		{
			name:          "no injection when Token is empty",
			lc:            port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "w", Platform: "github", Token: ""},
			envOverride:   map[string]string{"GITHUB_TOKEN": ""},
			wantTokenPair: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GITHUB_REPO", "")
			t.Setenv("GITLAB_REPO", "")
			for k, v := range tc.envOverride {
				t.Setenv(k, v)
			}
			got := linkEnv(&tc.lc)
			if tc.wantTokenPair != "" {
				assert.Contains(t, got, tc.wantTokenPair)
			} else {
				for _, entry := range got {
					assert.NotContains(t, entry, "TOKEN=")
				}
			}
		})
	}
}

func TestLinkEnv_RepoInjection(t *testing.T) {
	tests := []struct {
		name        string
		lc          port.LinkContext
		envOverride map[string]string
		wantEntry   string // "KEY=val" expected in result, or "" if not expected
	}{
		{
			name:        "github repo injected when GITHUB_REPO not set",
			lc:          port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
			envOverride: map[string]string{"GITHUB_REPO": ""},
			wantEntry:   "GITHUB_REPO=acme/widget",
		},
		{
			name:        "github repo not injected when GITHUB_REPO already set",
			lc:          port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
			envOverride: map[string]string{"GITHUB_REPO": "existing/repo"},
			wantEntry:   "",
		},
		{
			name:        "gitlab repo injected when GITLAB_REPO not set",
			lc:          port.LinkContext{BaseURL: "https://gitlab.com", Owner: "grp", Repo: "proj", Platform: "gitlab"},
			envOverride: map[string]string{"GITLAB_REPO": ""},
			wantEntry:   "GITLAB_REPO=grp/proj",
		},
		{
			name:        "gitlab repo not injected when GITLAB_REPO already set",
			lc:          port.LinkContext{BaseURL: "https://gitlab.com", Owner: "grp", Repo: "proj", Platform: "gitlab"},
			envOverride: map[string]string{"GITLAB_REPO": "other/repo"},
			wantEntry:   "",
		},
		{
			name:        "gitlab nested group repo injected as full path",
			lc:          port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group/sub", Repo: "proj", Platform: "gitlab"},
			envOverride: map[string]string{"GITLAB_REPO": ""},
			wantEntry:   "GITLAB_REPO=group/sub/proj",
		},
		{
			name:        "no injection when Owner is empty (ambient context)",
			lc:          port.LinkContext{BaseURL: "https://gitlab.example.com/grp/proj", Repo: "proj", Platform: "gitlab"},
			envOverride: map[string]string{"GITLAB_REPO": ""},
			wantEntry:   "",
		},
		{
			name:        "no injection when Repo is empty",
			lc:          port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Platform: "github"},
			envOverride: map[string]string{"GITHUB_REPO": ""},
			wantEntry:   "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", "")
			t.Setenv("GITLAB_TOKEN", "")
			for k, v := range tc.envOverride {
				t.Setenv(k, v)
			}
			got := linkEnv(&tc.lc)
			if tc.wantEntry != "" {
				assert.Contains(t, got, tc.wantEntry)
			} else {
				for _, entry := range got {
					assert.False(t, hasPrefix(entry, "GITHUB_REPO=") || hasPrefix(entry, "GITLAB_REPO="),
						"unexpected repo entry injected: %s", entry)
				}
			}
		})
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
