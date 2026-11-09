package e2e_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// Without network, whatsnew falls back to the changelog built into the binary: it still shows the
// releases newer than the running build, and exits 0.
func TestWhatsnew_FallsBackToTheBuiltInChangelogOffline(t *testing.T) {
	changelog, err := os.ReadFile("../CHANGELOG.md")
	require.NoError(t, err)
	latest := regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\]`).FindSubmatch(changelog)
	require.NotNil(t, latest, "the repository changelog has a release heading")

	bin := harness.Binary(t)
	repo := harness.NewRepo(t)
	unreachable := []string{"HTTPS_PROXY=http://127.0.0.1:1", "HTTP_PROXY=http://127.0.0.1:1"}

	res := repo.Run(bin, unreachable, "whatsnew")

	assert.Equal(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout, string(latest[1]), "the newest release of the built-in changelog")
}
