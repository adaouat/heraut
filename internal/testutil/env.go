package testutil

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
)

// ClearCIEnv neutralizes every CI marker and token variable production code reads
// (config.CIEnvVars), so a test's resolution outcome depends only on the variables it explicitly
// sets rather than the ambient environment the test happens to run in (e.g. this repo's own
// GitHub Actions CI).
func ClearCIEnv(t *testing.T) {
	t.Helper()
	for _, k := range config.CIEnvVars {
		t.Setenv(k, "")
	}
}
