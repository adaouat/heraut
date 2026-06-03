package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"

	forgeui "github.com/adaouat/forge/ui"
)

// Step represents a single named check or pipeline step.
// In TTY environments it shows an animated spinner while the work runs, then
// replaces it with a final status line. In non-TTY (CI, pipe, dry-run) it
// falls back to plain text so log files stay readable.
type Step struct {
	out   io.Writer
	msg   string
	plain bool

	mu   sync.Mutex
	stop chan struct{}
	done bool
}

var spinningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

// StartStep begins a step with automatic TTY detection.
// Animates a spinner when the writer is a terminal, falls back to plain text
// otherwise.
func StartStep(out io.Writer, msg string) *Step {
	return newStep(out, msg, !forgeui.IsTTY(out))
}

// StartPlainStep always uses plain lines regardless of terminal. Use in
// dry-run mode where output lines must not be overwritten by the spinner.
func StartPlainStep(out io.Writer, msg string) *Step {
	return newStep(out, msg, true)
}

func newStep(out io.Writer, msg string, plain bool) *Step {
	s := &Step{out: out, msg: msg, plain: plain, stop: make(chan struct{})}
	if !plain {
		go s.animate()
	}
	return s
}

func (s *Step) animate() {
	sp := spinner.MiniDot
	ticker := time.NewTicker(sp.FPS)
	defer ticker.Stop()
	frames := sp.Frames
	i := 0
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			if !s.done {
				_, _ = fmt.Fprintf(s.out, "\r  %s  %s", spinningStyle.Render(frames[i%len(frames)]), s.msg)
			}
			s.mu.Unlock()
			i++
		}
	}
}

// Done completes the step successfully.
// Prints "✓ step name — result" (or "✓ step name" when result is empty).
func (s *Step) Done(result string) {
	s.complete(func() {
		label := s.msg
		if result != "" {
			label = s.msg + " — " + result
		}
		_, _ = fmt.Fprintln(s.out, Success(s.out, label))
	})
}

// Fail completes the step with a failure.
// Prints "✗ step name — first_line" with any additional lines indented beneath.
func (s *Step) Fail(detail string) {
	s.complete(func() {
		lines := strings.Split(strings.TrimRight(detail, "\n"), "\n")
		_, _ = fmt.Fprintln(s.out, Err(s.out, s.msg+" — "+lines[0]))
		for _, line := range lines[1:] {
			_, _ = fmt.Fprintf(s.out, "       %s\n", strings.TrimLeft(line, " "))
		}
	})
}

// Skip completes the step as an advisory warning (IsWarn=true).
// Prints "! step name — detail".
func (s *Step) Skip(detail string) {
	s.complete(func() {
		_, _ = fmt.Fprintln(s.out, Warn(s.out, s.msg+" — "+detail))
	})
}

func (s *Step) complete(print func()) {
	if s.plain {
		print()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = true
	close(s.stop)
	// Clear the spinner line, then print the final result on a clean line.
	_, _ = fmt.Fprintf(s.out, "\r%s\r", strings.Repeat(" ", len(s.msg)+10))
	print()
}
