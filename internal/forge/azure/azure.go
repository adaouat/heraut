// Package azure implements port.Forge for Azure DevOps over stdlib net/http. Azure exposes no
// linked-handle field on commits, so commit-author handles are rendered locally from the git
// author email's local-part (ADR-0043, ADR-0035, T151).
package azure

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// Forge is the Azure DevOps implementation of port.Forge.
type Forge struct {
	id     port.ForgeIdentity
	client *http.Client
}

var _ port.Forge = (*Forge)(nil)

// New constructs an Azure DevOps forge. A nil client gets a default with a 30s timeout, matching
// the other forges.
func New(id port.ForgeIdentity, client *http.Client) *Forge {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Forge{id: id, client: client}
}

func (f *Forge) Type() string                 { return "azure_devops" }
func (f *Forge) Identity() port.ForgeIdentity { return f.id }

// webBase is the repository's web root, e.g. https://dev.azure.com/org/project/_git/repo.
func (f *Forge) webBase() string {
	return strings.TrimRight(f.id.Host, "/") + "/" + f.id.Project + "/_git/" + f.id.Repository
}

func (f *Forge) CommitURL(sha string) string { return f.webBase() + "/commit/" + sha }
func (f *Forge) ChangeURL(number int) string {
	return fmt.Sprintf("%s/pullrequest/%d", f.webBase(), number)
}
func (f *Forge) CompareURL(from, to string) string {
	return fmt.Sprintf("%s/branchCompare?baseVersion=GT%s&targetVersion=GT%s", f.webBase(), from, to)
}

// Enrich resolves the pull request associated with each commit SHA plus a local commit-author
// handle via one batched Azure DevOps pullrequestquery POST.
func (f *Forge) Enrich(commits []port.Commit) (port.Enrichment, error) {
	if len(commits) == 0 {
		return port.Enrichment{PRs: map[string]port.PullRequest{}, Authors: map[string]string{}}, nil
	}
	shas := make([]string, 0, len(commits))
	for _, c := range commits {
		shas = append(shas, c.Hash)
	}
	prs, err := f.enrichPRs(shas)
	if err != nil {
		return port.Enrichment{}, err
	}
	return port.Enrichment{PRs: prs, Authors: commitAuthors(commits)}, nil
}

// commitAuthors maps each commit SHA to its author handle — the git author email's local-part,
// via authorLogin. Azure exposes no identity resolvable from a git email (T151 spike), so this
// local render is the only source and it makes no API call. Commits whose author yields no
// handle are omitted.
func commitAuthors(commits []port.Commit) map[string]string {
	authors := make(map[string]string, len(commits))
	for _, c := range commits {
		if h := authorLogin(identityRef{DisplayName: c.Author, UniqueName: c.Email}); h != "" {
			authors[c.Hash] = h
		}
	}
	return authors
}

// authorLogin renders an "@handle"-friendly author from an Azure identity: the local-part of
// uniqueName (usually an email) when present, else the full displayName. Azure has no login handle.
func authorLogin(id identityRef) string {
	if id.UniqueName != "" {
		if local, _, ok := strings.Cut(id.UniqueName, "@"); ok && local != "" {
			return local
		}
		return id.UniqueName
	}
	return id.DisplayName
}
