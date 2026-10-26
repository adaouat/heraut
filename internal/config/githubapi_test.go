package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestGitHubAPIURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"empty base defaults to github.com", "", "https://api.github.com"},
		{"github.com", "https://github.com", "https://api.github.com"},
		{"github.com trailing slash", "https://github.com/", "https://api.github.com"},
		{"github.com mixed case", "https://GitHub.COM", "https://api.github.com"},
		{"github.com explicit default port", "https://github.com:443", "https://api.github.com"},
		{"ghe.com tenant", "https://octo.ghe.com", "https://api.octo.ghe.com"},
		{"ghe.com tenant mixed case, trailing slash", "https://Octo.GHE.com/", "https://api.octo.ghe.com"},
		{"GHES", "https://ghe.example.com", "https://ghe.example.com/api/v3"},
		{"GHES trailing slash", "https://ghe.example.com/", "https://ghe.example.com/api/v3"},
		{"GHES port", "https://ghe.example.com:8443", "https://ghe.example.com:8443/api/v3"},
		{"GHES path prefix", "https://example.com/github", "https://example.com/github/api/v3"},
		{"loopback test double over http", "http://127.0.0.1:34567", "http://127.0.0.1:34567/api/v3"},
		{"name that only ends in ghe.com is GHES", "https://notghe.com", "https://notghe.com/api/v3"},
		{"ghe.com as a subdomain label is GHES", "https://ghe.com.evil.example", "https://ghe.com.evil.example/api/v3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := config.GitHubAPIURL(tc.baseURL, noEnv)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGitHubAPIURL_InsecureBase(t *testing.T) {
	for _, base := range []string{
		"http://ghe.example.com",
		"http://localhost.evil.com",
		"ftp://ghe.example.com",
		"ghe.example.com",
		"not a url",
	} {
		t.Run(base, func(t *testing.T) {
			_, err := config.GitHubAPIURL(base, noEnv)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "https")
		})
	}
}

func TestGitHubAPIURL_ActionsEnv(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		env     map[string]string
		want    string
	}{
		{
			name:    "same origin: GITHUB_API_URL wins",
			baseURL: "https://ghe.example.com",
			env:     actionsEnv("https://ghe.example.com", "https://ghe.example.com/custom/api/"),
			want:    "https://ghe.example.com/custom/api",
		},
		{
			name:    "same origin compared without case and default port",
			baseURL: "https://GHE.example.com:443/",
			env:     actionsEnv("https://ghe.example.com", "https://ghe.example.com/custom/api"),
			want:    "https://ghe.example.com/custom/api",
		},
		{
			name:    "runner on github.com publishing to a GHES: derived",
			baseURL: "https://ghe.example.com",
			env:     actionsEnv("https://github.com", "https://api.github.com"),
			want:    "https://ghe.example.com/api/v3",
		},
		{
			name:    "same host, other port: derived",
			baseURL: "https://ghe.example.com:8443",
			env:     actionsEnv("https://ghe.example.com", "https://ghe.example.com/custom/api"),
			want:    "https://ghe.example.com:8443/api/v3",
		},
		{
			name:    "same host, other scheme: derived",
			baseURL: "https://ghe.example.com",
			env:     actionsEnv("http://ghe.example.com", "http://ghe.example.com/custom/api"),
			want:    "https://ghe.example.com/api/v3",
		},
		{
			name:    "outside Actions the variables are ignored",
			baseURL: "https://ghe.example.com",
			env: map[string]string{
				config.EnvGitHubServerURL: "https://ghe.example.com",
				config.EnvGitHubAPIURL:    "https://ghe.example.com/custom/api",
			},
			want: "https://ghe.example.com/api/v3",
		},
		{
			name:    "Actions without GITHUB_API_URL: derived",
			baseURL: "https://ghe.example.com",
			env:     actionsEnv("https://ghe.example.com", ""),
			want:    "https://ghe.example.com/api/v3",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := config.GitHubAPIURL(tc.baseURL, mapEnv(tc.env))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGitHubAPIURL_InsecureActionsAPIURL(t *testing.T) {
	_, err := config.GitHubAPIURL("https://ghe.example.com",
		mapEnv(actionsEnv("https://ghe.example.com", "http://ghe.example.com/api/v3")))
	require.Error(t, err)
	assert.Contains(t, err.Error(), config.EnvGitHubAPIURL)
}

func actionsEnv(serverURL, apiURL string) map[string]string {
	return map[string]string{
		config.EnvGitHubActions:   "true",
		config.EnvGitHubServerURL: serverURL,
		config.EnvGitHubAPIURL:    apiURL,
	}
}

func mapEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func noEnv(string) string { return "" }
