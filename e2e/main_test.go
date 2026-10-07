package e2e_test

import (
	"os"
	"testing"

	"github.com/adaouat/heraut/e2e/harness"
)

func TestMain(m *testing.M) {
	code := m.Run()
	harness.Cleanup()
	os.Exit(code)
}
