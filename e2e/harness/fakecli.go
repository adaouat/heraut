package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fakeScript = `#!/bin/sh
d=$(dirname "$0")
{ printf "%s" "%[1]s"; for a in "$@"; do printf " [%%s]" "$a"; done; echo; } >> "$d/calls.log"
if [ "$1" = release ] && [ -f "$d/fail-%[1]s" ]; then
  echo "boom" >&2
  exit 1
fi
exit 0
`

// FakeCLI installs an executable called name in a directory that Run puts first on PATH. It
// appends "name [arg] [arg]" to a call log shared by every fake of this repository and exits 0.
func (r *Repo) FakeCLI(name string) {
	r.t.Helper()
	if r.binDir == "" {
		r.binDir = r.t.TempDir()
	}
	script := fmt.Sprintf(fakeScript, name)
	if err := os.WriteFile(filepath.Join(r.binDir, name), []byte(script), 0o755); err != nil {
		r.t.Fatalf("installing fake %s: %v", name, err)
	}
}

// FailReleases makes the named fake exit 1 on any "release …" subcommand (calls are still recorded).
func (r *Repo) FailReleases(name string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.binDir, "fail-"+name), nil, 0o644); err != nil {
		r.t.Fatalf("marking %s as failing: %v", name, err)
	}
}

// CLICalls returns every call recorded by the fakes, oldest first.
func (r *Repo) CLICalls() []string {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.binDir, "calls.log"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		r.t.Fatalf("reading the call log: %v", err)
	}
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// runEnv is baseEnv with the fake directory (if any) prepended to PATH.
func (r *Repo) runEnv() []string {
	env := baseEnv(r.home)
	if r.binDir == "" {
		return env
	}
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + r.binDir + string(os.PathListSeparator) + strings.TrimPrefix(kv, "PATH=")
		}
	}
	return env
}
