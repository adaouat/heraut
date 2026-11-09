package cmd

import (
	"errors"
	"fmt"
	osexec "os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/exitcode"
)

// A resolver that cannot be built because git failed is a runtime failure; one refused for a flag or
// the config is a config error.
func TestWrapResolverErr(t *testing.T) {
	gitFailed := osexec.Command("sh", "-c", "exit 128").Run()
	var exitErr *osexec.ExitError
	require.ErrorAs(t, gitFailed, &exitErr)
	_, notFound := osexec.LookPath("heraut-no-such-binary")
	require.Error(t, notFound)

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"git exited non-zero", fmt.Errorf("determining current git branch: %w", fmt.Errorf("git: %w", gitFailed)), exitcode.Runtime},
		{"git missing", fmt.Errorf("determining current git branch: %w", notFound), exitcode.Runtime},
		{"a refused flag", errors.New("--set-build-id requires --set-version"), exitcode.Config},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, exitcode.Resolve(wrapResolverErr(tc.err)))
		})
	}
}
