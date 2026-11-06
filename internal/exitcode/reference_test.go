package exitcode_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/exitcode"
)

// The exit codes reference has a row for every code heraut exits with and every promotion guard, so
// a new code cannot ship undocumented.
func TestExitCodesReference_ListsEveryCode(t *testing.T) {
	page, err := os.ReadFile("../../docs/reference/exit-codes.md")
	require.NoError(t, err)

	for _, code := range []int{exitcode.OK, exitcode.Usage, exitcode.Config, exitcode.Runtime, exitcode.Promotion, exitcode.Internal} {
		assert.Contains(t, string(page), fmt.Sprintf("\n| %d |", code), "a table row for exit code %d", code)
	}
	for _, guard := range []string{"E001", "E002", "E003"} {
		assert.Contains(t, string(page), "| `"+guard+"` |", "a row for promotion guard %s", guard)
	}
}
