// Package gitlab implements port.Forge for GitLab over stdlib net/http. REST is the default
// transport because GitLab's GraphQL API rejects CI job tokens outright; see ADR-0043.
package gitlab

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// Forge is the GitLab implementation of port.Forge.
type Forge struct {
	id     port.ForgeIdentity
	client *http.Client
}

var _ port.Forge = (*Forge)(nil)

// New constructs a GitLab forge. A nil client gets a default with a 30s timeout, matching the
// Azure enrichment client (ADR-0035).
func New(id port.ForgeIdentity, client *http.Client) *Forge {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Forge{id: id, client: client}
}

func (f *Forge) Type() string                 { return "gitlab" }
func (f *Forge) Identity() port.ForgeIdentity { return f.id }

// webBase is the project's web root, e.g. https://gitlab.example.com/group/subgroup/project.
func (f *Forge) webBase() string {
	return strings.TrimRight(f.id.Host, "/") + "/" + f.id.Project
}

func (f *Forge) CommitURL(sha string) string { return f.webBase() + "/-/commit/" + sha }
func (f *Forge) ChangeURL(number int) string {
	return fmt.Sprintf("%s/-/merge_requests/%d", f.webBase(), number)
}
func (f *Forge) CompareURL(from, to string) string {
	return fmt.Sprintf("%s/-/compare/%s...%s", f.webBase(), from, to)
}

// apiBase returns the REST/GraphQL API root: the explicit APIURL when set (GitLab CI provides it
// as CI_API_V4_URL), else the conventional {host}/api/v4.
func (f *Forge) apiBase() string {
	if f.id.APIURL != "" {
		return strings.TrimRight(f.id.APIURL, "/")
	}
	return strings.TrimRight(f.id.Host, "/") + "/api/v4"
}

// setAuth applies the auth header matching the token's kind. GitLab requires JOB-TOKEN for a CI
// job token and PRIVATE-TOKEN for a personal/project access token; sending a job token as
// PRIVATE-TOKEN is rejected.
func (f *Forge) setAuth(req *http.Request) {
	switch f.id.TokenKind {
	case port.TokenJob:
		req.Header.Set("JOB-TOKEN", f.id.Token)
	case port.TokenPrivate:
		req.Header.Set("PRIVATE-TOKEN", f.id.Token)
	case port.TokenNone:
	}
}

// Enrich resolves per-commit MR references and author handles. api_mode: graphql opts into the
// batched GraphQL transport (linked @usernames, requires a non-job token); the default is REST.
func (f *Forge) Enrich(commits []port.Commit) (port.Enrichment, error) {
	if len(commits) == 0 {
		return port.Enrichment{PRs: map[string]port.PullRequest{}, Authors: map[string]string{}}, nil
	}
	if f.id.APIMode == "graphql" {
		return f.enrichGraphQL(commits)
	}
	return f.enrichREST(commits)
}

// gitAuthors maps sha → the git author email's local-part, falling back to the git author name.
// REST commit payloads expose no linked GitLab username, so this is the only `by @` source in
// REST mode. The local-part is preferred because an `@handle` should not contain spaces — a git
// display name like "Alice Smith" is not handle-shaped — matching the Azure forge's fallback (see
// ADR-0043). Commits with neither are omitted.
func gitAuthors(commits []port.Commit) map[string]string {
	authors := make(map[string]string, len(commits))
	for _, c := range commits {
		if local, _, ok := strings.Cut(c.Email, "@"); ok && local != "" {
			authors[c.Hash] = local
			continue
		}
		if c.Author != "" {
			authors[c.Hash] = c.Author
		}
	}
	return authors
}
