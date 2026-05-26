package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/ui"
	"github.com/stretchr/testify/assert"
)

// All step tests use *bytes.Buffer (not a terminal), so StartStep and
// StartPlainStep both fall back to plain mode. No animation goroutines start.

func TestStep_Done_NoResult(t *testing.T) {
	w := &bytes.Buffer{}
	s := ui.StartPlainStep(w, "git")
	s.Done("")
	got := w.String()
	assert.Equal(t, "✓ git\n", got)
}

func TestStep_Done_WithResult(t *testing.T) {
	w := &bytes.Buffer{}
	s := ui.StartPlainStep(w, "git")
	s.Done("2.49.0")
	got := w.String()
	assert.Equal(t, "✓ git — 2.49.0\n", got)
}

func TestStep_Fail(t *testing.T) {
	w := &bytes.Buffer{}
	s := ui.StartPlainStep(w, "git user.name")
	s.Fail("not configured; run: git config user.name <name>")
	got := w.String()
	assert.Equal(t, "✗ git user.name — not configured; run: git config user.name <name>\n", got)
}

func TestStep_Skip(t *testing.T) {
	w := &bytes.Buffer{}
	s := ui.StartPlainStep(w, "working tree")
	s.Skip("2 uncommitted change(s)")
	got := w.String()
	assert.Equal(t, "! working tree — 2 uncommitted change(s)\n", got)
}

func TestStep_StartStep_PlainOnBuffer(t *testing.T) {
	// StartStep with a non-terminal writer must behave exactly like StartPlainStep.
	w := &bytes.Buffer{}
	s := ui.StartStep(w, "working tree")
	s.Done("clean")
	got := w.String()
	assert.Equal(t, "✓ working tree — clean\n", got)
}

func TestStep_Fail_MultiLine(t *testing.T) {
	w := &bytes.Buffer{}
	s := ui.StartPlainStep(w, "git")
	s.Fail("main error\n  hint: fix this")
	got := w.String()
	assert.True(t, strings.HasPrefix(got, "✗ git — main error\n"), got)
	assert.Contains(t, got, "hint: fix this")
}
