package port

import "net/url"

// APIEnv returns the CLI authentication environment variables for this platform context.
// For GitHub, it returns ["GH_TOKEN=<token>"] when Token is set, plus "GH_HOST=<host>"
// when BaseURL is a non-github.com host (GHES). Returns nil when lc is nil.
// GitHub and GitLab are wired; Azure DevOps returns nil.
func (lc *LinkContext) APIEnv() []string {
	if lc == nil {
		return nil
	}
	switch lc.Platform {
	case "github":
		return gitHubAPIEnv(lc.Token, lc.BaseURL)
	case "gitlab":
		return gitLabAPIEnv(lc.Token, lc.BaseURL)
	default:
		// Azure DevOps: not yet implemented.
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
	if host := nonDefaultHost(baseURL, "github.com"); host != "" {
		env = append(env, "GH_HOST="+host)
	}
	return env
}

// gitLabAPIEnv builds the env slice for glab CLI authentication. GITLAB_TOKEN is omitted when
// token is empty; GITLAB_HOST is added only for a non-gitlab.com (self-managed) BaseURL.
func gitLabAPIEnv(token, baseURL string) []string {
	var env []string
	if token != "" {
		env = append(env, "GITLAB_TOKEN="+token)
	}
	if host := nonDefaultHost(baseURL, "gitlab.com"); host != "" {
		env = append(env, "GITLAB_HOST="+host)
	}
	return env
}

// nonDefaultHost returns baseURL's host when it is a self-hosted instance (not defaultHost),
// or "" for the default host, an empty BaseURL, or an unparsable URL.
func nonDefaultHost(baseURL, defaultHost string) string {
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || u.Host == defaultHost {
		return ""
	}
	return u.Host
}
