package ui_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All progress tests use *bytes.Buffer (non-terminal), so StartStep falls back
// to plain mode — no spinner goroutines, deterministic output.

func TestProgress_Step_CounterIncrements(t *testing.T) {
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 3)

	_ = p.Step("first", func() (string, []string, error) { return "", nil, nil })
	_ = p.Step("second", func() (string, []string, error) { return "", nil, nil })
	_ = p.Step("third", func() (string, []string, error) { return "", nil, nil })

	out := w.String()
	assert.Contains(t, out, "[1/3] first")
	assert.Contains(t, out, "[2/3] second")
	assert.Contains(t, out, "[3/3] third")
}

func TestProgress_Step_Result_Present(t *testing.T) {
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 1)

	err := p.Step("Resolve version", func() (string, []string, error) {
		return "v1.2.3", nil, nil
	})

	require.NoError(t, err)
	assert.Contains(t, w.String(), "[1/1] Resolve version — v1.2.3")
}

func TestProgress_Step_Result_Absent(t *testing.T) {
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 1)

	err := p.Step("Commit changelog", func() (string, []string, error) {
		return "", nil, nil
	})

	require.NoError(t, err)
	got := w.String()
	// Label appears without the " — " suffix.
	assert.Contains(t, got, "[1/1] Commit changelog")
	assert.NotContains(t, got, " — ")
}

func TestProgress_Step_SubResults(t *testing.T) {
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 1)

	err := p.Step("Publish to github", func() (string, []string, error) {
		return "https://example.com/releases/v1.0.0", []string{"assets uploaded"}, nil
	})

	require.NoError(t, err)
	out := w.String()
	// Parent line with URL.
	assert.Contains(t, out, "[1/1] Publish to github — https://example.com/releases/v1.0.0")
	// Sub-result on its own indented line, after the parent line.
	assert.Contains(t, out, "assets uploaded")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.GreaterOrEqual(t, len(lines), 2, "expected at least parent + sub-result line")
	subLine := lines[len(lines)-1]
	assert.True(t, strings.HasPrefix(subLine, "  "), "sub-result should be indented: %q", subLine)
}

func TestProgress_Step_MultipleSubResults(t *testing.T) {
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 1)

	err := p.Step("Publish", func() (string, []string, error) {
		return "https://example.com", []string{"assets uploaded", "checksums verified"}, nil
	})

	require.NoError(t, err)
	out := w.String()
	assert.Contains(t, out, "assets uploaded")
	assert.Contains(t, out, "checksums verified")
}

func TestProgress_Step_Failure(t *testing.T) {
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 3)

	err := p.Step("Create tag", func() (string, []string, error) {
		return "", nil, errors.New("tag already exists")
	})

	require.Error(t, err)
	assert.EqualError(t, err, "tag already exists")
	got := w.String()
	// Failure line contains the step label and the error detail.
	assert.Contains(t, got, "[1/3] Create tag")
	assert.Contains(t, got, "tag already exists")
	// The failure symbol.
	assert.Contains(t, got, "✗")
}

func TestProgress_Step_ZeroTotal(t *testing.T) {
	// total == 0 is allowed; renders [N/0] without crashing.
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 0)

	err := p.Step("any step", func() (string, []string, error) {
		return "ok", nil, nil
	})

	require.NoError(t, err)
	assert.Contains(t, w.String(), "[1/0] any step")
}

func TestProgress_Step_NilFn(t *testing.T) {
	// A nil fn must not panic; it should behave as a successful no-op step.
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 1)

	require.NotPanics(t, func() {
		_ = p.Step("no-op", nil)
	})
}

func TestProgress_StepFn_Assignable(t *testing.T) {
	// Verify that (*Progress).Step satisfies the StepFn type at compile time.
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 1)
	var fn ui.StepFn = p.Step
	err := fn("via StepFn", func() (string, []string, error) {
		return "assigned", nil, nil
	})
	require.NoError(t, err)
	assert.Contains(t, w.String(), "via StepFn — assigned")
}

func TestProgress_Step_SuccessSymbol(t *testing.T) {
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 1)

	_ = p.Step("Push tag", func() (string, []string, error) { return "", nil, nil })

	assert.Contains(t, w.String(), "✓")
}

func TestProgress_Step_ErrorReturnedToCallerUnwrapped(t *testing.T) {
	// The error returned by Step must be the same error returned by fn,
	// not a wrapped version, so callers can use errors.Is/As.
	w := &bytes.Buffer{}
	p := ui.NewProgress(w, 1)
	sentinel := errors.New("sentinel")

	err := p.Step("failing step", func() (string, []string, error) {
		return "", nil, fmt.Errorf("outer: %w", sentinel)
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, sentinel), "errors.Is must traverse the chain")
}
