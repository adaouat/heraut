//go:build e2e_forge

package forgeharness

import (
	"slices"
	"strings"
	"time"

	"github.com/adaouat/heraut/e2e/harness"
)

const retryAttempts = 3

var retryBackoff = 5 * time.Second

// Run is Repo.Run for a heraut invocation that pushes to the sandbox. GitHub intermittently
// answers a push with `! [remote rejected] … (failed | missing necessary objects)`; such a run
// is rolled back and repeated, up to three attempts. Any other outcome is returned as it is, so a
// real regression is never masked.
func (w *Workspace) Run(bin string, env []string, args ...string) harness.Result {
	var res harness.Result
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		snap := w.snapshot()
		res = w.Repo.Run(bin, env, args...)
		if res.ExitCode == 0 || !strings.Contains(res.Stdout+res.Stderr, "[remote rejected]") || attempt == retryAttempts {
			return res
		}
		w.rollback(snap)
		time.Sleep(retryBackoff)
	}
	return res
}

type runSnapshot struct {
	head     string
	tags     []string
	forgeTag []string
	branches []string
}

func (w *Workspace) snapshot() runSnapshot {
	tags, _ := w.Forge.Tags(w.RunID)
	branches, _ := w.Forge.Branches(w.Branch)
	return runSnapshot{
		head:     w.Repo.Git("rev-parse", "HEAD"),
		tags:     strings.Fields(w.Repo.Git("tag", "-l")),
		forgeTag: tags,
		branches: branches,
	}
}

// rollback undoes what the failed attempt left behind: its local commit and tags, and the run's
// tags and branch that reached the forge before the rejection. A tag the forge already held is
// never touched. Cleanup at the end of the test still removes anything this misses.
func (w *Workspace) rollback(s runSnapshot) {
	w.Repo.Git("reset", "-q", "--hard", s.head)
	for _, tag := range strings.Fields(w.Repo.Git("tag", "-l")) {
		if !slices.Contains(s.tags, tag) {
			w.Repo.Git("tag", "-d", tag)
		}
	}
	if tags, err := w.Forge.Tags(w.RunID); err == nil {
		for _, tag := range tags {
			if !slices.Contains(s.forgeTag, tag) {
				_ = w.Forge.DeleteTag(tag)
			}
		}
	}
	if branches, err := w.Forge.Branches(w.Branch); err == nil {
		for _, b := range branches {
			if !slices.Contains(s.branches, b) {
				_ = w.Forge.DeleteBranch(b)
			}
		}
	}
}
