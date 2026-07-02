package pipeline

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

const defaultCommitMessage = "chore(release): ${version}"

type gitHelper struct {
	runner port.Runner
}

func (g *gitHelper) run(name string, args ...string) error {
	_, _, err := g.runner.Run(name, args...)
	return err
}

// commitChangelog stages file and commits it with msg, pushing when push is set. It
// reports whether a commit was actually created: when `git add` stages nothing — the
// changelog is byte-identical to the last commit, e.g. a re-run after a partial release
// or a release with no changelog-worthy commits — it returns (false, nil) without
// committing so the caller can warn and continue to tag/publish rather than failing on
// git's "nothing to commit" exit.
func (g *gitHelper) commitChangelog(file, msg string, push bool) (bool, error) {
	if err := g.run("git", "add", file); err != nil {
		return false, fmt.Errorf("git add: %w", err)
	}
	staged, err := g.hasStagedChanges()
	if err != nil {
		return false, err
	}
	if !staged {
		return false, nil
	}
	if err := g.run("git", "commit", "-m", msg); err != nil {
		return false, fmt.Errorf("git commit: %w", err)
	}
	if push {
		if err := g.run("git", "push", "origin", "HEAD"); err != nil {
			return false, fmt.Errorf("git push: %w", err)
		}
	}
	return true, nil
}

// hasStagedChanges reports whether the index holds any staged change. A genuine git
// failure is returned as an error rather than misread as "nothing staged".
func (g *gitHelper) hasStagedChanges() (bool, error) {
	out, _, err := g.runner.Run("git", "diff", "--cached", "--name-only")
	if err != nil {
		return false, fmt.Errorf("git diff --cached: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

func (g *gitHelper) tag(tag, msg string, annotated, sign bool) error {
	if sign {
		// -s implies annotated; always provide -m so git does not open an editor.
		return g.run("git", "tag", "-s", tag, "-m", msg)
	}
	if annotated {
		return g.run("git", "tag", "-a", tag, "-m", msg)
	}
	return g.run("git", "tag", tag)
}

func (g *gitHelper) pushTag(tag string) error {
	if err := g.run("git", "push", "origin", tag); err != nil {
		return fmt.Errorf("git push %s: %w", tag, err)
	}
	return nil
}

func commitMessage(template, version string) string {
	if template == "" {
		template = defaultCommitMessage
	}
	return strings.ReplaceAll(template, "${version}", version)
}
