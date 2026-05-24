package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	os_exec "os/exec"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

var _ port.Runner = (*Runner)(nil)

// Runner executes external CLI commands, with optional dry-run and verbose modes.
type Runner struct {
	DryRun  bool
	Verbose bool
	// Out receives dry-run and verbose log lines; defaults to os.Stderr when nil.
	Out io.Writer
}

// New constructs a Runner.
func New(dryRun, verbose bool) *Runner {
	return &Runner{DryRun: dryRun, Verbose: verbose}
}

// Run executes name with args, returning captured stdout and stderr.
func (r *Runner) Run(name string, args ...string) (string, string, error) {
	return r.RunEnv(nil, name, args...)
}

// RunEnv executes name with args, appending env to the current process environment.
func (r *Runner) RunEnv(env []string, name string, args ...string) (string, string, error) {
	if r.DryRun {
		_, _ = fmt.Fprintf(r.writer(), "[dry-run] %s %s\n", name, strings.Join(args, " "))
		return "", "", nil
	}

	if r.Verbose {
		_, _ = fmt.Fprintf(r.writer(), "[exec] %s %s\n", name, strings.Join(args, " "))
	}

	cmd := os_exec.Command(name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("%s: %w", name, err)
	}

	return stdout.String(), stderr.String(), nil
}

func (r *Runner) writer() io.Writer {
	if r.Out != nil {
		return r.Out
	}
	return os.Stderr
}
