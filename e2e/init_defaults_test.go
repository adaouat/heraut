package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// init --defaults writes no forges: entry and says which forge the runs will find.
func TestInitDefaults_ReportsTheDetectedForge(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		env    []string
		want   []string
	}{
		{name: "GitHub origin", origin: "git@github.com:acme/app.git", want: []string{"github", "acme/app"}},
		{name: "GitLab.com origin", origin: "https://gitlab.com/acme/app.git", want: []string{"gitlab", "acme/app"}},
		{name: "self-hosted GitLab in its CI", origin: "https://gitlab.example.com/acme/app.git",
			env:  []string{"GITLAB_CI=true", "CI_SERVER_URL=https://gitlab.example.com", "CI_PROJECT_PATH=acme/app"},
			want: []string{"gitlab", "acme/app"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bin := harness.Binary(t)
			repo := harness.NewRepo(t)
			repo.Commit("chore: init")
			repo.Git("remote", "add", "origin", tc.origin)

			res := repo.Run(bin, tc.env, "init", "--defaults")

			require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			cfg := repo.ReadFile(".heraut.yml")
			assert.NotContains(t, cfg, "forges:")
			assert.Contains(t, cfg, "release:")
			for _, w := range tc.want {
				assert.Contains(t, res.Stdout, w)
			}
			assert.Contains(t, res.Stdout, "heraut release will publish to")

			check := repo.Run(bin, tc.env, "check", "config")
			assert.Equal(t, exitOK, check.ExitCode, "the written config is valid:\n%s%s", check.Stdout, check.Stderr)
		})
	}
}

// With nothing to detect, the config is still written, with a warning that heraut release needs a
// forges: entry.
func TestInitDefaults_WarnsWhenNoForgeIsDetected(t *testing.T) {
	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	repo.Commit("chore: init")
	repo.Git("remote", "add", "origin", "git@gitlab.example.com:acme/app.git")

	res := repo.Run(bin, nil, "init", "--defaults")

	require.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.NotContains(t, repo.ReadFile(".heraut.yml"), "forges:")
	assert.Contains(t, res.Stdout+res.Stderr, "no GitHub or GitLab forge detected")
	assert.Contains(t, res.Stdout+res.Stderr, "forges:")
}
