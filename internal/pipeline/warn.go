package pipeline

import (
	"io"

	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/ui"
)

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
// omission note; otherwise there is nothing to add.
func changelogGenResult(gen port.Generator) (detail string, subs []string) {
	if subs = degradedSubs(gen); subs != nil {
		return "without enrichment", subs
	}
	return "", nil
}

// printResolveWarnings writes every warning the version resolver produced (Result.Warnings) to
// out, right after the "Resolve version" step so it reads as that step's outcome.
func printResolveWarnings(out io.Writer, warnings []string) {
	for _, w := range warnings {
		ui.WarnLines(out, w)
	}
}
