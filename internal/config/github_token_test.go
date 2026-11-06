package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

// The GitHub token chain is read the same way by publishing and by pull-request enrichment: gh's
// names for the host, first set wins.
func TestGitHubTokenEnvs(t *testing.T) {
	public := []string{"GH_TOKEN", "GITHUB_TOKEN"}
	enterprise := []string{"GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_TOKEN"}
	tests := []struct {
		baseURL string
		want    []string
	}{
		{"https://github.com", public},
		{"https://GitHub.com/", public},
		{"https://acme.ghe.com", public},
		{"https://github.example.com", enterprise},
		{"https://github.com.example.com", enterprise},
	}
	for _, tc := range tests {
		t.Run(tc.baseURL, func(t *testing.T) {
			assert.Equal(t, tc.want, config.GitHubTokenEnvs(tc.baseURL))
		})
	}
}
