package gitlab

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
)

// The CI job token goes only to the instance the job runs on (CI_SERVER_URL, gitlab.com when unset).
// Any other host gets the configured token, gitlab.com included when the job runs elsewhere.
func TestCredentials_JobTokenOnlyForTheJobsInstance(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		server    string
		wantToken string
		wantJob   bool
	}{
		{name: "gitlab.com target, gitlab.com job", baseURL: "https://gitlab.com", server: "https://gitlab.com", wantToken: "job-token", wantJob: true},
		{name: "no base_url, gitlab.com job", server: "https://gitlab.com", wantToken: "job-token", wantJob: true},
		{name: "gitlab.com target, no CI_SERVER_URL", baseURL: "https://gitlab.com", wantToken: "job-token", wantJob: true},
		{name: "self-hosted target, its own job", baseURL: "https://gitlab.example.com", server: "https://gitlab.example.com", wantToken: "job-token", wantJob: true},
		{name: "gitlab.com target, self-hosted job", baseURL: "https://gitlab.com", server: "https://gitlab.example.com", wantToken: "personal-token"},
		{name: "self-hosted target, gitlab.com job", baseURL: "https://gitlab.example.com", server: "https://gitlab.com", wantToken: "personal-token"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testutil.ClearCIEnv(t)
			t.Setenv(config.EnvGitLabCI, "true")
			t.Setenv(config.EnvGitLabServerURL, tc.server)
			t.Setenv(config.EnvGitLabJobToken, "job-token")
			t.Setenv("GITLAB_TOKEN", "personal-token")
			p := New(&config.Platform{Type: "gitlab", BaseURL: tc.baseURL, Project: "acme/app"})

			token, job, err := p.credentials()

			require.NoError(t, err)
			assert.Equal(t, tc.wantToken, token)
			assert.Equal(t, tc.wantJob, job)
		})
	}
}
