// Package harness builds and drives the real heraut binary for the e2e suite (ADR-0066).
package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Same shape as the ldflags in .goreleaser.yml, so the version-injection path is exercised.
const ldflags = "-s -w -X main.Version=v0.0.0-e2e"

var (
	mu     sync.Mutex
	binDir string
	built  = map[string]string{}
)

// Binary builds cmd/heraut once per distinct tag set and returns the executable path.
func Binary(t testing.TB, tags ...string) string {
	t.Helper()
	key := strings.Join(tags, ",")

	mu.Lock()
	defer mu.Unlock()
	if path, ok := built[key]; ok {
		return path
	}
	if binDir == "" {
		dir, err := os.MkdirTemp("", "heraut-e2e-")
		if err != nil {
			t.Fatalf("creating build dir: %v", err)
		}
		binDir = dir
	}

	out := filepath.Join(binDir, fmt.Sprintf("heraut-%d", len(built)))
	args := []string{"build", "-ldflags", ldflags, "-o", out}
	if key != "" {
		args = append(args, "-tags", key)
	}
	args = append(args, "./cmd/heraut")

	cmd := exec.Command("go", args...)
	cmd.Dir = moduleRoot(t)
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, combined)
	}
	built[key] = out
	return out
}

// Cleanup removes the build directory; call it from TestMain after m.Run.
func Cleanup() {
	mu.Lock()
	defer mu.Unlock()
	if binDir != "" {
		_ = os.RemoveAll(binDir)
	}
	binDir = ""
	built = map[string]string{}
}

func moduleRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test's working directory")
		}
		dir = parent
	}
}
