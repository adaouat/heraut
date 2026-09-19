package pipeline

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

const defaultCommitMessage = "chore(release): ${version}"

type gitHelper struct {
	runner port.Runner
	// interactiveRunner runs commands that may need a real terminal — a GPG pinentry prompt during
	// `git commit`/`git tag -s`, T260 — connecting stdin/stdout/stderr directly instead of
	// capturing them (forge v0.19.0's CmdRunner.Interactive). Falls back to runner when nil, so
	// every construction that doesn't set it (every pre-T260 test, any caller that hasn't opted
	// in) behaves exactly as before.
	interactiveRunner port.Runner
}

func (g *gitHelper) run(name string, args ...string) error {
	_, _, err := g.runner.Run(name, args...)
	return err
}

// interactiveOrRunner returns interactiveRunner, falling back to the regular runner when none is
// configured. Shared by runInteractive and hook execution (T268) so both reach the same
// real-terminal runner instance rather than each tracking their own fallback.
func (g *gitHelper) interactiveOrRunner() port.Runner {
	if g.interactiveRunner != nil {
		return g.interactiveRunner
	}
	return g.runner
}

// runInteractive runs name with args using interactiveRunner, falling back to the regular runner
// when none is configured.
func (g *gitHelper) runInteractive(name string, args ...string) error {
	_, _, err := g.interactiveOrRunner().Run(name, args...)
	return err
}

// commitChangelog stages files (the changelog path plus any hook-declared stage patterns,
// ADR-0061) and commits them with msg, pushing when push is set. Reports whether a commit was
// actually created: when `git add` stages nothing across every path — every file byte-identical
// to the last commit — it returns (false, nil) without committing so the caller can warn and
// continue to tag/publish rather than failing on git's "nothing to commit" exit. A files entry
// that matches nothing on disk is a `git add` failure like any other, propagated as-is — no new
// zero-match detection needed (ADR-0061 Design §4).
func (g *gitHelper) commitChangelog(files []string, msg string, push bool) (bool, error) {
	if err := g.run("git", append([]string{"add"}, files...)...); err != nil {
		return false, fmt.Errorf("git add: %w", err)
	}
	staged, err := g.hasStagedChanges()
	if err != nil {
		return false, err
	}
	if !staged {
		return false, nil
	}
	if err := g.runInteractive("git", "commit", "-m", msg); err != nil {
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
		// -s implies annotated; always provide -m so git does not open an editor. GPG signing can
		// prompt via pinentry, so this runs interactively (T260).
		return g.runInteractive("git", "tag", "-s", tag, "-m", msg)
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
