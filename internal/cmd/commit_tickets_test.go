package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitTickets_Registered(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var found bool
	for _, c := range root.Commands() {
		if c.Use != "commit" {
			continue
		}
		for _, sc := range c.Commands() {
			if strings.HasPrefix(sc.Use, "tickets") {
				found = true
			}
		}
	}
	assert.True(t, found, "commit tickets subcommand registered")
}

func ticketsCfgYAML() string {
	return `
version: "1"
versioning:
  strategy: semver
commits:
  tickets:
    - pattern: 'PROJ-([0-9]+)'
      url: 'https://jira.example.com/browse/PROJ-{ticket}'
`
}

func TestCommitTickets_NoTicketsConfigured_ErrorsWithUsage(t *testing.T) {
	testutil.RealGitRepo(t, "v0.1.0")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("version: \"1\"\nversioning:\n  strategy: semver\n"), 0o644))

	_, err := executeRoot("commit", "tickets", "--config", cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commits.tickets")
}

func TestCommitTickets_MatchesAndReportsSummary(t *testing.T) {
	testutil.RealGitRepo(t, "v0.1.0")

	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "fix: resolve PROJ-42")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(ticketsCfgYAML()), 0o644))

	out, err := executeRoot("commit", "tickets", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "PROJ-42")
	assert.Contains(t, out, "https://jira.example.com/browse/PROJ-42")
	assert.Contains(t, out, "1 ticket reference")
}

func TestCommitTickets_NoMatchesAtAll_WarnsButDoesNotError(t *testing.T) {
	testutil.RealGitRepo(t, "v0.1.0")

	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "chore: unrelated change")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(ticketsCfgYAML()), 0o644))

	out, err := executeRoot("commit", "tickets", "--config", cfgPath)
	require.NoError(t, err, "zero matches is a warning, not a failure")
	assert.Contains(t, out, "no ticket reference")
}

func TestCommitTickets_FromLatestTagAndRevRange_MutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(ticketsCfgYAML()), 0o644))

	_, err := executeRoot("commit", "tickets", "v1.0.0..HEAD", "--from-latest-tag", "--config", cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use both")
}
