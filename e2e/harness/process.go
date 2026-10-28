package harness

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// Process is a command started in the background by Repo.Start, for a scenario that acts while it
// runs (an interrupt, for one).
type Process struct {
	cmd            *exec.Cmd
	stdout, stderr *syncBuffer
	done           chan struct{}
	err            error
}

// Start runs bin with args inside the repository, with the environment Run uses, and returns at
// once. The test must call Wait.
func (r *Repo) Start(bin string, env []string, args ...string) *Process {
	r.t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = r.Dir
	cmd.Env = append(baseEnv(r.home), env...)
	p := &Process{cmd: cmd, stdout: newSyncBuffer(), stderr: newSyncBuffer(), done: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = p.stdout, p.stderr
	if err := cmd.Start(); err != nil {
		r.t.Fatalf("starting %s %v: %v", bin, args, err)
	}
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()
	return p
}

// WaitForOutput blocks until the process has printed want on stdout, and fails the test after ten
// seconds or when the process exits first.
func (p *Process) WaitForOutput(t testing.TB, want string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for !strings.Contains(p.stdout.String(), want) {
		select {
		case <-p.stdout.written:
		case <-p.done:
			if !strings.Contains(p.stdout.String(), want) {
				t.Fatalf("the process exited before printing %q; stdout:\n%s", want, p.stdout.String())
			}
		case <-deadline:
			t.Fatalf("the process did not print %q within ten seconds; stdout:\n%s", want, p.stdout.String())
		}
	}
}

// Interrupt sends the process SIGINT, as Ctrl-C in a terminal does.
func (p *Process) Interrupt(t testing.TB) {
	t.Helper()
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("interrupting the process: %v", err)
	}
}

// Wait returns the outcome once the process exits, and fails the test if it is still running after
// thirty seconds (the process is then killed).
func (p *Process) Wait(t testing.TB) Result {
	t.Helper()
	select {
	case <-p.done:
	case <-time.After(30 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
		t.Fatalf("the process did not exit within thirty seconds; stdout:\n%s\nstderr:\n%s", p.stdout.String(), p.stderr.String())
	}
	res := Result{Stdout: p.stdout.String(), Stderr: p.stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case p.err == nil:
	case errors.As(p.err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		t.Fatalf("waiting for the process: %v", p.err)
	}
	return res
}

// syncBuffer is a bytes.Buffer safe to write from the process and read from the test, which signals
// each write so a reader can wait for output without polling.
type syncBuffer struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	written chan struct{}
}

func newSyncBuffer() *syncBuffer { return &syncBuffer{written: make(chan struct{}, 1)} }

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	n, err := b.buf.Write(p)
	b.mu.Unlock()
	select {
	case b.written <- struct{}{}:
	default:
	}
	return n, err
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
