package native

import "time"

// Author is a contributor identity: git-first (Name/Email always present), with an optional
// platform handle resolved from remote enrichment. Email is the first_time identity key.
type Author struct {
	Name     string
	Email    string
	Username string // platform handle, e.g. "octocat"; empty offline
}

// PullRequest is the normalized PR/MR for a commit: flat common fields plus a per-platform
// escape hatch. RefPrefix is "!" for GitLab MRs and Azure DevOps PRs; GitHub leaves it empty
// and prRef (render.go) defaults it to "#". It is derived at fetch time.
type PullRequest struct {
	Number      int
	URL         string
	AuthorLogin string // PR author handle (drives "by @login")
	RefPrefix   string
	Title       string
	Labels      []string
	Platforms   map[string]any
	// CreatedAt / MergedAt are the PR/MR creation and merge timestamps (remote-only, zero offline).
	CreatedAt time.Time
	MergedAt  time.Time
	// MergedBy is the actor who merged the PR/MR (remote-only; empty Author when unknown).
	MergedBy Author
	// Approvers are the reviewers who approved (best-effort: GitHub + Azure; empty on GitLab).
	Approvers []Author
}

// Contributor is a per-release contributor for the "New Contributors" block.
type Contributor struct {
	Author      Author
	IsFirstTime bool
	PR          *PullRequest // their first PR in this release; nil offline
}
