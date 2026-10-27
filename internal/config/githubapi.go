package config

import (
	"fmt"
	"net/url"
	"strings"
)

// GitHubAPIURL returns the REST API root, without a trailing slash, for a GitHub platform whose web
// root is baseURL (empty means github.com): https://api.github.com for github.com, https://api.<host>
// for a GitHub Enterprise Cloud tenant on *.ghe.com, and <baseURL>/api/v3 for GitHub Enterprise Server
// (ADR-0077). In GitHub Actions, GITHUB_API_URL is used instead, but only when GITHUB_SERVER_URL has the
// same origin as baseURL: a runner on github.com publishing to a GHES must not take the runner's API.
// The URL the token will be sent to must pass IsSecureURL.
func GitHubAPIURL(baseURL string, getenv func(string) string) (string, error) {
	if baseURL == "" {
		baseURL = DefaultGitHubBaseURL
	}
	if !IsSecureURL(baseURL) {
		return "", fmt.Errorf("GitHub base URL %q must use https (plain http is accepted only for localhost and loopback addresses)", baseURL)
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parsing GitHub base URL %q: %w", baseURL, err)
	}

	if apiURL := actionsAPIURL(base, getenv); apiURL != "" {
		if !IsSecureURL(apiURL) {
			return "", fmt.Errorf("%s %q must use https (plain http is accepted only for localhost and loopback addresses)", EnvGitHubAPIURL, apiURL)
		}
		return strings.TrimRight(apiURL, "/"), nil
	}

	host := strings.ToLower(base.Hostname())
	switch {
	case host == "github.com":
		return "https://api.github.com", nil
	case strings.HasSuffix(host, ".ghe.com"):
		return base.Scheme + "://api." + strings.ToLower(base.Host), nil
	}
	return strings.TrimRight(baseURL, "/") + "/api/v3", nil
}

// actionsAPIURL returns GITHUB_API_URL when this is a GitHub Actions run on the instance base points
// at, else "".
func actionsAPIURL(base *url.URL, getenv func(string) string) string {
	if getenv(EnvGitHubActions) != "true" {
		return ""
	}
	server, err := url.Parse(getenv(EnvGitHubServerURL))
	if err != nil || !SameOrigin(server, base) {
		return ""
	}
	return getenv(EnvGitHubAPIURL)
}

// SameOrigin reports whether a and b share an origin: scheme, host name and port, ignoring case and
// treating an omitted port as the scheme's default.
func SameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		effectivePort(a) == effectivePort(b)
}

func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	}
	return ""
}
