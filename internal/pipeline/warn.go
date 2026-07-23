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

// degradedSubs returns the sub-result lines when gen degraded (enrichment failed under the
// "optional" policy): the underlying failure reason (when the generator exposes DegradedReason)
// followed by the generic omission note. Empty when the generator did not degrade. Degraded and
// DegradedReason are optional interfaces so the pipeline stays decoupled from concrete generators.
func degradedSubs(gen port.Generator) []string {
	d, ok := gen.(interface{ Degraded() bool })
	if !ok || !d.Degraded() {
		return nil
	}
	var subs []string
	if r, ok := gen.(interface{ DegradedReason() string }); ok {
		if reason := r.DegradedReason(); reason != "" {
			subs = append(subs, reason)
		}
	}
	return append(subs, "remote metadata unavailable — PR authors/numbers omitted")
}

// changelogGenResult builds the completed-step detail and sub-results for a changelog generation
// step. On degrade the step is labelled "without enrichment" and lists the failure reason plus the
// omission note; the GitLab regeneration rate-limit heads-up is shown only when enrichment did NOT
// degrade — on a degrade it is misleading, since the fetch already failed.
func changelogGenResult(regenerate bool, lc *port.LinkContext, gen port.Generator) (detail string, subs []string) {
	if subs = degradedSubs(gen); subs != nil {
		return "without enrichment", subs
	}
	return "", gitlabRegenWarning(regenerate, lc)
}
