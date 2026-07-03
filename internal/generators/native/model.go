package native

// Author is a contributor identity: git-first (Name/Email always present), with an optional
// platform handle resolved from remote enrichment. Email is the first_time identity key.
type Author struct {
	Name     string
	Email    string
	Username string // platform handle, e.g. "octocat"; empty offline
}

// PullRequest is the normalized PR/MR for a commit: flat common fields plus a per-platform
// escape hatch. RefPrefix is "#" (GitHub/Azure) or "!" (GitLab); it is derived at fetch time.
type PullRequest struct {
	Number      int
	URL         string
	AuthorLogin string // PR author handle (drives "by @login")
	RefPrefix   string
	Title       string
	Labels      []string
	Platforms   map[string]any
}

// Contributor is a per-release contributor for the "New Contributors" block.
type Contributor struct {
	Author      Author
	IsFirstTime bool
	PR          *PullRequest // their first PR in this release; nil offline
}
