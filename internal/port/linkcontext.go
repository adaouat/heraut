package port

import "net/url"

// APIEnv returns the CLI authentication environment variables for this platform context.
// For GitHub, it returns ["GH_TOKEN=<token>"] when Token is set, plus "GH_HOST=<host>"
// when BaseURL is a non-github.com host (GHES). Returns nil when lc is nil.
// GitLab support (GITLAB_TOKEN) is added in T128.
func (lc *LinkContext) APIEnv() []string {
	if lc == nil {
		return nil
	}
	switch lc.Platform {
	case "github":
		return gitHubAPIEnv(lc.Token, lc.BaseURL)
	default:
		// GitLab (T128) and Azure DevOps (T129): not yet implemented.
		return nil
	}
}

// gitHubAPIEnv builds the env slice for gh CLI authentication. GH_TOKEN is omitted when
// token is empty; GH_HOST is added only for a non-github.com (GHES) BaseURL.
func gitHubAPIEnv(token, baseURL string) []string {
	var env []string
	if token != "" {
		env = append(env, "GH_TOKEN="+token)
	}
	if host := ghesHost(baseURL); host != "" {
		env = append(env, "GH_HOST="+host)
	}
	return env
}

// ghesHost returns the hostname for a self-hosted GitHub Enterprise Server URL.
// Returns "" for the default github.com host, an empty BaseURL, or an unparsable URL.
func ghesHost(baseURL string) string {
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Host == "github.com" {
		return ""
	}
	return u.Host
}
