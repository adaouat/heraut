package harness

import (
	"bytes"
	"errors"
	"os/exec"
)

// Result is the observable outcome of one binary invocation.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes bin with args inside the repository. The environment is built from scratch
// (PATH, HOME, NO_COLOR, ...) plus env, so ambient CI variables never reach the binary.
func (r *Repo) Run(bin string, env []string, args ...string) Result {
	r.t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = r.Dir
	cmd.Env = append(r.runEnv(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		r.t.Fatalf("running %s %v: %v", bin, args, err)
	}
	return res
}
