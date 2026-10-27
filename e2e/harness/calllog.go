package harness

import (
	"os"
	"path/filepath"
	"strings"
)

// APICalls returns every request the stand-ins of this repository received, oldest first, as
// "forge [METHOD] [path]".
func (r *Repo) APICalls() []string {
	r.t.Helper()
	if r.logDir == "" {
		r.t.Fatal("APICalls called before any stand-in was started: an empty result would prove nothing")
	}
	b, err := os.ReadFile(filepath.Join(r.logDir, "calls.log"))
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
