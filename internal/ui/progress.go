package ui

// StepFn is the type of a function that wraps one named pipeline step. The
// pipeline calls it per step; app wires it to a forge ui.Spinner (see
// app.spinnerReporter). When the reporter is nil, the pipeline calls fn
// directly.
//
//   - result: shown as "✓ [N/M] name — result" when non-empty; omitted otherwise
//   - subs:   zero or more sub-result labels printed as indented lines beneath
//   - err:    step failure; rendered "✗ [N/M] name — err"
type StepFn func(name string, fn func() (result string, subs []string, err error)) error
