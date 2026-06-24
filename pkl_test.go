package heraut_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPklBuiltinPackages asserts the real `pkl` CLI accepts and can package
// pkl/Builtins.pkl — MockRunner-style contract tests can't catch a Pkl syntax
// or type error the real tool would reject. Skips when pkl is not on PATH;
// runs in CI where mise installs it. See ADR-0029.
func TestPklBuiltinPackages(t *testing.T) {
	if _, err := exec.LookPath("pkl"); err != nil {
		t.Skip("pkl not on PATH")
	}

	outDir := t.TempDir()
	cmd := exec.Command("pkl", "project", "package", "pkl/",
		"--output-path", outDir+"/",
		"--skip-publish-check",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "real pkl must accept pkl/Builtins.pkl: %s", out)

	zipPath := filepath.Join(outDir, "heraut@0.0.0-dev.zip")
	require.FileExists(t, zipPath)
}
