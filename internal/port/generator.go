package port

// LinkContext carries the per-platform link-resolution context used to render
// commit/PR/MR links in release notes. A nil *LinkContext means "no per-platform
// context — fall through to ambient CI detection" (the single-platform path).
// See ADR-0021 / ADR-0020.
type LinkContext struct {
	BaseURL  string // resolved per-platform web base URL, e.g. "https://gitlab.example.com"
	Owner    string // org / namespace (GitLab group[/subgroup])
	Repo     string // repository / project name
	Platform string // "github" | "gitlab" — selects PR (/pull/N) vs MR (/-/merge_requests/N) link shape
}

// Generator produces changelog or release-notes text.
//
// Generate renders the content for tag. When lc is non-nil, the generator resolves
// links against that per-platform context (each implementation translating it into
// its own native mechanism); when nil, the generator falls through to whatever
// ambient detection it already performs.
type Generator interface {
	Check() error
	Validate() error
	Generate(tag string, lc *LinkContext) (string, error)
}
