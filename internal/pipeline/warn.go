package pipeline

import "github.com/adaouat/heraut/internal/port"

// gitlabRegenWarning returns a one-line caution when a full changelog regeneration will enrich
// every section against a GitLab remote — GitLab has no batched per-commit-MR primitive, so it is
// one API call per commit (slow / rate-limited). GitHub and Azure batch, so they need no warning.
func gitlabRegenWarning(regenerate bool, lc *port.LinkContext) []string {
	if regenerate && lc != nil && lc.Platform == "gitlab" {
		return []string{"--regenerate re-fetches PR metadata one call per commit on GitLab; this may be slow and hit rate limits"}
	}
	return nil
}
