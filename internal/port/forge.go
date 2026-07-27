package port

import "time"

// TokenKind selects how a forge token is presented to the API. GitLab distinguishes a CI job
// token (JOB-TOKEN header) from a personal/project access token (PRIVATE-TOKEN header); other
// forges ignore it. See ADR-0043.
type TokenKind int

const (
	TokenNone TokenKind = iota
	TokenJob
	TokenPrivate
)

// ForgeIdentity is a forge's fully resolved connection facts (from config, CI env, or git origin).
type ForgeIdentity struct {
	Type    string // "github" | "gitlab" | "azure_devops"
	Host    string // web host, e.g. "https://gitlab.example.com"
	APIURL  string // API base, e.g. "https://gitlab.example.com/api/v4"
	Project string // "owner/repo" | "group/subgroup/project" | "organization/project"
	// Repository is the repository name when a forge separates it from the project path — Azure
	// DevOps addresses a repo as organization/project + repository. GitHub and GitLab carry the
	// full path in Project and leave this empty.
	Repository string
	Token      string
	TokenKind  TokenKind
	APIMode    string // "rest" | "graphql"
}

// Author is a resolved platform user handle.
type Author struct{ Username string }

// PullRequest is the per-commit PR/MR metadata a Forge resolves. RefPrefix is "#" for a GitHub PR
// and "!" for a GitLab MR.
type PullRequest struct {
	Number      int
	URL         string
	Title       string
	AuthorLogin string
	Labels      []string
	RefPrefix   string
	CreatedAt   time.Time
	MergedAt    time.Time
	MergedBy    Author
	Approvers   []Author
}

// Enrichment is a forge's per-commit remote data: sha→PR and sha→author-handle.
type Enrichment struct {
	PRs     map[string]PullRequest
	Authors map[string]string
}

// Commit is the minimal per-commit input a Forge needs to resolve enrichment: the SHA plus the
// local git author name/email/date.
type Commit struct {
	Hash   string
	Author string // git author name (%an)
	Email  string // git author email (%ae)
	Date   time.Time
}

// Forge is one code-hosting platform heraut talks to: it exposes its resolved identity, builds web
// links, and fetches per-commit PR/MR + author metadata. One implementation per platform type lives
// in internal/forge/. See ADR-0043.
type Forge interface {
	Type() string
	Identity() ForgeIdentity
	CommitURL(sha string) string
	ChangeURL(number int) string
	CompareURL(from, to string) string
	Enrich(commits []Commit) (Enrichment, error)
}
