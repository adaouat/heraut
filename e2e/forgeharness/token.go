//go:build e2e_forge

package forgeharness

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// GitHubToken returns a token from the environment, else from the gh login.
func GitHubToken() (string, error) {
	if v := firstEnv("HERAUT_E2E_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"); v != "" {
		return v, nil
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "", fmt.Errorf("no GitHub token: set HERAUT_E2E_GITHUB_TOKEN or run gh auth login")
	}
	return strings.TrimSpace(string(out)), nil
}

// GitLabToken returns a token from the environment, else from the glab login.
func GitLabToken() (string, error) {
	if v := firstEnv("HERAUT_E2E_GITLAB_TOKEN", "GITLAB_TOKEN"); v != "" {
		return v, nil
	}
	out, err := exec.Command("glab", "auth", "status", "--show-token").CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if i := strings.Index(line, "Token"); i >= 0 {
				if j := strings.LastIndex(line, ": "); j >= 0 {
					if tok := strings.TrimSpace(line[j+2:]); tok != "" {
						return tok, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("no GitLab token: set HERAUT_E2E_GITLAB_TOKEN or run glab auth login")
}
