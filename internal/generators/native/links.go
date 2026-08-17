package native

import (
	"net/url"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// buildCommitURL returns the base URL for commit links given a LinkContext.
// For GitHub: root+"/commit/"; for GitLab: root+"/-/commit/"; for Azure DevOps: root+"/commit/".
// Returns "" when lc is nil — the caller renders a bare hash instead.
func buildCommitURL(lc *port.LinkContext) string {
	if lc == nil {
		return ""
	}
	root := repoRoot(lc)
	if lc.Platform == "gitlab" {
		return root + "/-/commit/"
	}
	return root + "/commit/"
}

// buildCompareURL returns the full compare URL between prev and version.
// Returns "" when lc is nil or prev is empty (first release — no previous tag).
//
// URL shapes by platform:
//   - GitHub:      root+"/compare/"+prev+".."+version
//   - GitLab:      root+"/-/compare/"+prev+".."+version
//   - Azure DevOps: root+"/branchCompare?baseVersion=GT"+prev+"&targetVersion=GT"+version
func buildCompareURL(lc *port.LinkContext, prev, version string) string {
	if lc == nil || prev == "" {
		return ""
	}
	root := repoRoot(lc)
	switch lc.Platform {
	case "gitlab":
		return root + "/-/compare/" + prev + ".." + version
	case "azure_devops":
		return root + "/branchCompare?baseVersion=GT" + prev + "&targetVersion=GT" + version
	default: // github
		return root + "/compare/" + prev + ".." + version
	}
}

// repoRoot builds the repository root URL from a LinkContext.
// For Azure DevOps, the root inserts /_git/ between the project and repository (see ADR-0026).
// For GitHub/GitLab and ambient contexts (Owner/Repo empty), the root is TrimRight(BaseURL)
// with Owner and Repo appended when present.
func repoRoot(lc *port.LinkContext) string {
	if lc.Platform == "azure_devops" {
		return azureRepoRoot(lc)
	}
	root := strings.TrimRight(lc.BaseURL, "/")
	if lc.Owner != "" {
		root += "/" + lc.Owner
	}
	if lc.Repo != "" {
		root += "/" + lc.Repo
	}
	return root
}

// azureRepoRoot builds the repository root URL for an Azure DevOps remote.
// URL shape: TrimRight(BaseURL)+"/"+PathEscape(Owner)+"/_git/"+PathEscape(Repo).
func azureRepoRoot(lc *port.LinkContext) string {
	root := strings.TrimRight(lc.BaseURL, "/")
	if lc.Owner != "" {
		root += "/" + urlPathSegments(lc.Owner)
	}
	if lc.Repo != "" {
		root += "/_git/" + url.PathEscape(lc.Repo)
	}
	return root
}

// urlPathSegments percent-encodes each "/"-delimited segment of s independently,
// preserving the literal "/" separators.
func urlPathSegments(s string) string {
	parts := strings.Split(s, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}
