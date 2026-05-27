package ui

import (
	"fmt"
	"io"
)

// StepFn is the type of a function that wraps one named pipeline step.
// name is shown while the step is running and on the completion line.
// fn performs the work. It returns:
//   - result: shown as "✓ [N/M] name — result" when non-empty; omitted otherwise
//   - subs:   zero or more sub-result labels, each printed as an indented
//             "  ✓ label" line beneath the parent completion line
//   - err:    step failure; the reporter renders "✗ [N/M] name — err"
//
// When the StepFn field on a pipeline is nil, the pipeline calls fn directly
// and discards result and subs — existing behaviour preserved, tests unaffected.
type StepFn func(name string, fn func() (result string, subs []string, err error)) error

// Progress runs a sequence of named pipeline steps, labelling each one with a
// [N/total] counter and an animated spinner in TTY environments.
//
// Non-TTY writers (CI, pipes, dry-run) automatically fall back to plain lines
// through the existing StartStep → StartPlainStep detection in step.go.
type Progress struct {
	out   io.Writer
	total int
	curr  int
}

// NewProgress constructs a Progress that writes to out and labels each step
// [N/total]. total may be 0 when the caller cannot pre-compute the count;
// steps then render as [N/0], which is a degraded-but-non-crashing state.
func NewProgress(out io.Writer, total int) *Progress {
	return &Progress{out: out, total: total}
}

// Step wraps one named pipeline step. It satisfies StepFn.
//
// If fn is nil, Step completes immediately as a successful no-op.
func (p *Progress) Step(name string, fn func() (result string, subs []string, err error)) error {
	p.curr++
	label := fmt.Sprintf("[%d/%d] %s", p.curr, p.total, name)
	step := StartStep(p.out, label)

	if fn == nil {
		step.Done("")
		return nil
	}

	result, subs, err := fn()
	if err != nil {
		step.Fail(err.Error())
		return err
	}

	step.Done(result)
	for _, sub := range subs {
		printSubResult(p.out, sub)
	}
	return nil
}

// printSubResult writes one indented sub-result line beneath a completed step.
// Example output: "     ✓ assets uploaded"
func printSubResult(out io.Writer, label string) {
	_, _ = fmt.Fprintln(out, "     "+Success(out, label))
}
