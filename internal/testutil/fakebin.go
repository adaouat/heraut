package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// FakeBin installs a shell script named `name` in a temp directory and
// prepends that directory to PATH for the duration of test t.
func FakeBin(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("FakeBin: write %s: %v", bin, err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
