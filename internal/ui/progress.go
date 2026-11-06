package ui

import "strings"

// StepFn is the type of a function that wraps one named pipeline step. The
// pipeline calls it per step; app wires it to a forge ui.Spinner (see
// app.spinnerReporter). When the reporter is nil, the pipeline calls fn
// directly.
//
//   - result: shown as "✓ [N/M] name — result" when non-empty; omitted otherwise
//   - subs:   zero or more sub-result labels printed as indented lines beneath; a label wrapped
//     in Warning prints as a warning ("!")
//   - err:    step failure; rendered "✗ [N/M] name — err"
type StepFn func(name string, fn func() (result string, subs []string, err error)) error

// warningMark starts a sub-result line that is an advisory rather than a success. It holds NUL
// bytes, which never occur in text heraut prints, so it can't be mistaken for content.
const warningMark = "\x00warning\x00"

// Warning marks a sub-result line as a warning: the reporter prints it with "!" instead of "✓".
func Warning(sub string) string { return warningMark + sub }

// SplitSubs separates a step's sub-result lines into successes and warnings, in order, removing the
// warning mark.
func SplitSubs(subs []string) (ok, warnings []string) {
	for _, s := range subs {
		if w, found := strings.CutPrefix(s, warningMark); found {
			warnings = append(warnings, w)
			continue
		}
		ok = append(ok, s)
	}
	return ok, warnings
}
