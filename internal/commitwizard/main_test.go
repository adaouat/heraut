package commitwizard_test

import (
	"os"
	"testing"

	"github.com/adaouat/heraut/internal/testutil"
)

// TestMain keeps the developer's git config and HERAUT_FILE out of these tests, which run real git.
func TestMain(m *testing.M) {
	testutil.IsolateProcessEnv()
	os.Exit(m.Run())
}
