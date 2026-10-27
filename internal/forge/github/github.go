// Package github implements port.Forge for GitHub over stdlib net/http, so changelog enrichment
// needs no `gh` CLI on PATH. See ADR-0043.
package github

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// Forge is the GitHub implementation of port.Forge.
type Forge struct {
	id     port.ForgeIdentity
	client *http.Client
}

var _ port.Forge = (*Forge)(nil)

// New constructs a GitHub forge. A nil client gets a default with a 30s timeout, matching the
// other forges.
func New(id port.ForgeIdentity, client *http.Client) *Forge {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Forge{id: id, client: withTokenSafeRedirects(client)}
}

// maxRedirects mirrors net/http's default limit, which a custom CheckRedirect replaces.
const maxRedirects = 10

// withTokenSafeRedirects returns a copy of c whose redirects never carry the token away from the
// origin it was sent to. net/http strips Authorization across hosts but keeps it on an https to
// http downgrade of the same host, which would send the token in clear. A CheckRedirect the caller
// set is kept. Same rule as the GitLab client.
func withTokenSafeRedirects(c *http.Client) *http.Client {
	if c.CheckRedirect != nil {
		return c
	}
	safe := *c
	safe.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		first := via[0]
		if req.URL.Host != first.URL.Host || (first.URL.Scheme == "https" && req.URL.Scheme != "https") {
			req.Header.Del("Authorization")
		}
		return nil
	}
	return &safe
}

func (f *Forge) Type() string                 { return "github" }
func (f *Forge) Identity() port.ForgeIdentity { return f.id }

// webBase is the repository's web root, e.g. https://github.com/acme/widget.
func (f *Forge) webBase() string {
	return strings.TrimRight(f.id.Host, "/") + "/" + f.id.Project
}

func (f *Forge) CommitURL(sha string) string { return f.webBase() + "/commit/" + sha }
func (f *Forge) ChangeURL(number int) string {
	return fmt.Sprintf("%s/pull/%d", f.webBase(), number)
}
func (f *Forge) CompareURL(from, to string) string {
	return fmt.Sprintf("%s/compare/%s...%s", f.webBase(), port.URLTag(from), port.URLTag(to))
}

// apiBase returns the REST API root: the identity's APIURL, which forge resolution always sets for
// GitHub, else the root config.GitHubAPIURL derives from the host (api.github.com, api.<host> for a
// *.ghe.com tenant, {host}/api/v3 for GitHub Enterprise Server).
func (f *Forge) apiBase() (string, error) {
	if f.id.APIURL != "" {
		return strings.TrimRight(f.id.APIURL, "/"), nil
	}
	return config.GitHubAPIURL(f.id.Host, func(string) string { return "" })
}

// graphqlEndpoint returns the GraphQL API endpoint. github.com and *.ghe.com serve it at
// {api root}/graphql (https://api.github.com/graphql, https://api.<tenant>.ghe.com/graphql); GitHub
// Enterprise Server serves it at {host}/api/graphql — a sibling of /api/v3 (REST), not nested under
// it. An explicit APIURL (as GITHUB_API_URL sets it on GHES, "{host}/api/v3") is normalized the same
// way: strip a trailing /api/v3 before appending /api/graphql.
func (f *Forge) graphqlEndpoint() (string, error) {
	base, err := f.apiBase()
	if err != nil {
		return "", err
	}
	if u, err := url.Parse(base); err == nil {
		host := strings.ToLower(u.Hostname())
		if host == "api.github.com" || (strings.HasPrefix(host, "api.") && strings.HasSuffix(host, ".ghe.com")) {
			return base + "/graphql", nil
		}
	}
	return strings.TrimSuffix(base, "/api/v3") + "/api/graphql", nil
}

// Enrich resolves each commit's associated pull request and linked commit-author handle via
// batched GraphQL queries. ref is unused: GitHub resolves per-commit, not via a ref-anchored walk.
func (f *Forge) Enrich(commits []port.Commit, _ string) (port.Enrichment, error) {
	if len(commits) == 0 {
		return port.Enrichment{PRs: map[string]port.PullRequest{}, Authors: map[string]string{}}, nil
	}
	return f.enrichGraphQL(commits)
}
