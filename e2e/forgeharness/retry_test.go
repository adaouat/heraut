//go:build e2e_forge

package forgeharness

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// fakeHeraut is a stand-in binary: in "transient" mode its first call commits and tags locally,
// then reports a rejected push; later calls succeed. "always" rejects every call, "fatal" fails
// with an unrelated error.
func fakeHeraut(t *testing.T) (bin, counter string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "heraut")
	counter = filepath.Join(dir, "count")
	script := `#!/bin/sh
n=$(cat "$FAKE_COUNT" 2>/dev/null || echo 0)
n=$((n+1))
echo $n > "$FAKE_COUNT"
reject() {
  git commit -q --allow-empty -m "chore(release): attempt $n"
  git tag "attempt-$n"
  echo "! [remote rejected] attempt-$n -> attempt-$n (failed)" >&2
  exit 3
}
case "$FAKE_MODE" in
  transient) [ "$n" -eq 1 ] && reject ;;
  always) reject ;;
  fatal) echo "boom: unrelated failure" >&2; exit 2 ;;
esac
echo ok
`
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	return bin, counter
}

func retryWorkspace(t *testing.T) *Workspace {
	t.Helper()
	oldBackoff := retryBackoff
	retryBackoff = 0
	t.Cleanup(func() { retryBackoff = oldBackoff })

	repo := harness.NewRepo(t)
	repo.Commit("chore: seed")
	return &Workspace{Repo: repo, Forge: &localForge{}, RunID: "e2e-1-abcd", Branch: "e2e/e2e-1-abcd"}
}

func calls(t *testing.T, counter string) int {
	t.Helper()
	b, err := os.ReadFile(counter)
	require.NoError(t, err)
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	require.NoError(t, err)
	return n
}

func TestRunRetriesARejectedPushFromACleanState(t *testing.T) {
	w := retryWorkspace(t)
	bin, counter := fakeHeraut(t)
	before := w.Repo.Git("rev-parse", "HEAD")

	res := w.Run(bin, []string{"FAKE_COUNT=" + counter, "FAKE_MODE=transient"}, "release")

	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, 2, calls(t, counter))
	assert.Equal(t, before, w.Repo.Git("rev-parse", "HEAD"), "the failed attempt's commit was rolled back")
	assert.Empty(t, w.Repo.Git("tag", "-l"), "and so was its tag")
}

func TestRunGivesUpAfterThreeRejectedAttempts(t *testing.T) {
	w := retryWorkspace(t)
	bin, counter := fakeHeraut(t)

	res := w.Run(bin, []string{"FAKE_COUNT=" + counter, "FAKE_MODE=always"}, "release")

	assert.Equal(t, 3, res.ExitCode)
	assert.Equal(t, 3, calls(t, counter))
	assert.Contains(t, res.Stderr, "[remote rejected]", "the last attempt's output is what the test sees")
}

func TestRunNeverRetriesAnUnrelatedFailure(t *testing.T) {
	w := retryWorkspace(t)
	bin, counter := fakeHeraut(t)

	res := w.Run(bin, []string{"FAKE_COUNT=" + counter, "FAKE_MODE=fatal"}, "release")

	assert.Equal(t, 2, res.ExitCode)
	assert.Equal(t, 1, calls(t, counter), "a real failure is reported at once")
}

// leakyForge reports the same tags and branches until a failed attempt "pushed" something: from
// its second listing on, the run's tag and branch are there too.
type leakyForge struct {
	*localForge
	listings int
	leakTag  string
	leakBr   string
}

func (l *leakyForge) Tags(prefix string) ([]string, error) {
	l.listings++
	if l.listings == 1 {
		return []string{l.leakTag + "-old"}, nil
	}
	return []string{l.leakTag + "-old", l.leakTag}, nil
}

func (l *leakyForge) Branches(prefix string) ([]string, error) {
	if l.listings <= 1 {
		return nil, nil
	}
	return []string{l.leakBr}, nil
}

func TestRunDeletesWhatTheRejectedAttemptLeftOnTheForge(t *testing.T) {
	w := retryWorkspace(t)
	f := &leakyForge{localForge: &localForge{}, leakTag: w.RunID + "-v0.1.0", leakBr: w.Branch}
	w.Forge = f
	bin, counter := fakeHeraut(t)

	res := w.Run(bin, []string{"FAKE_COUNT=" + counter, "FAKE_MODE=transient"}, "release")

	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, []string{f.leakTag}, f.deletedTags, "only what appeared during the attempt")
	assert.Equal(t, []string{w.Branch}, f.deletedBranches)
}
