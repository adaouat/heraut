// Package github implements port.Forge for GitHub over stdlib net/http, so changelog enrichment
// needs no `gh` CLI on PATH. See ADR-0043.
package github

import (
	"fmt"
	"net/http"
	"strings"
	"time"

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
	return &Forge{id: id, client: client}
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
	return fmt.Sprintf("%s/compare/%s...%s", f.webBase(), from, to)
}

// apiBase returns the GraphQL API root: the explicit APIURL when set (GitHub Actions provides
// GITHUB_API_URL), else api.github.com for github.com and {host}/api/v3 for GitHub Enterprise.
func (f *Forge) apiBase() string {
	if f.id.APIURL != "" {
		return strings.TrimRight(f.id.APIURL, "/")
	}
	host := strings.TrimRight(f.id.Host, "/")
	if host == "https://github.com" || host == "" {
		return "https://api.github.com"
	}
	return host + "/api/v3"
}

// Enrich resolves each commit's associated pull request and linked commit-author handle via
// batched GraphQL queries.
func (f *Forge) Enrich(commits []port.Commit) (port.Enrichment, error) {
	if len(commits) == 0 {
		return port.Enrichment{PRs: map[string]port.PullRequest{}, Authors: map[string]string{}}, nil
	}
	return f.enrichGraphQL(commits)
}
