package app

import (
	"bytes"
	"os/exec"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/testutil"
)

// --verbose echoes each command's output; the origin URL can carry a token, so forge resolution reads
// it quietly (forge's runner logs the command line under the same switch, so that one is not logged).
func TestResolveForge_VerboseNeverPrintsTheOriginURL(t *testing.T) {
	testutil.ClearCIEnv(t)
	testutil.RealGitRepo(t, "v0.1.0")
	require.NoError(t, exec.Command("git", "remote", "add", "origin", "https://oauth2:glpat-FAKESECRET123@gitlab.example.com/acme/app.git").Run())
	var log bytes.Buffer
	verbose := execadapter.New(false, true)
	verbose.Out = &log

	for _, runner := range []struct {
		name string
		r    func() any
	}{
		{"plain runner", func() any { return verbose }},
		{"memoizing runner", func() any { return NewMemoizingRunner(verbose) }},
	} {
		t.Run(runner.name, func(t *testing.T) {
			log.Reset()
			resolved, err := resolveForge(runner.r().(interface {
				Run(string, ...string) (string, string, error)
				RunEnv([]string, string, ...string) (string, string, error)
				RunDir(string, []string, string, ...string) (string, string, error)
			}), func(string) string { return "" }, &config.Config{})
			require.NoError(t, err)
			_ = resolved
			assert.NotContains(t, log.String(), "FAKESECRET123")
		})
	}
}
