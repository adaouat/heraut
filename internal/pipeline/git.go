package pipeline

import (
	"fmt"
	"strconv"
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
	// pushBranch names the branch the changelog commit goes to from a detached HEAD; empty pushes
	// HEAD to the checked-out branch.
	pushBranch string
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
// continue to tag/publish rather than failing on git's "nothing to commit" exit. HEAD is still
// pushed in that case when it is ahead of its upstream: a previous run may have committed locally
// and failed at the push, and the tag pushed afterwards would otherwise point at a commit on no
// remote branch (T360). A files entry
// that matches nothing on disk is a `git add` failure like any other, propagated as-is — no new
// zero-match detection needed (ADR-0061 Design §4). pushOptions go to that push (ADR-0079).
func (g *gitHelper) commitChangelog(files []string, msg string, push bool, pushOptions []string) (bool, error) {
	if err := g.run("git", append([]string{"add"}, files...)...); err != nil {
		return false, fmt.Errorf("git add: %w", err)
	}
	staged, err := g.hasStagedChanges()
	if err != nil {
		return false, err
	}
	if staged {
		if err := g.runInteractive("git", "commit", "-m", msg); err != nil {
			return false, fmt.Errorf("git commit: %w", err)
		}
	}
	if push && (staged || g.aheadOfUpstream()) {
		if err := g.run("git", pushArgs(g.commitPushRef(), pushOptions)...); err != nil {
			return false, fmt.Errorf("git push: %w", err)
		}
	}
	return staged, nil
}

// commitPushRef is the refspec the changelog commit is pushed with. A detached HEAD has no branch
// for `git push origin HEAD` to update, so it names the CI job's branch.
func (g *gitHelper) commitPushRef() string {
	if g.pushBranch == "" {
		return "HEAD"
	}
	return "HEAD:refs/heads/" + g.pushBranch
}

// aheadOfUpstream reports whether HEAD holds commits its upstream lacks. It answers false when it
// cannot tell — a detached HEAD (the norm in CI checkouts) or a branch without an upstream — where
// pushing HEAD would fail and the run has nothing of its own to deliver.
func (g *gitHelper) aheadOfUpstream() bool {
	out, _, err := g.runner.Run("git", "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	return err == nil && n > 0
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

// pushTag pushes tag by its full ref: a branch with the same name would make the bare name ambiguous
// ("src refspec … matches more than one").
func (g *gitHelper) pushTag(tag string, pushOptions []string) error {
	if err := g.run("git", pushArgs("refs/tags/"+tag, pushOptions)...); err != nil {
		return fmt.Errorf("git push %s: %w", tag, err)
	}
	return nil
}

// pushArgs builds `push [--push-option=<v>…] origin <ref>`. Each option is one
// --push-option=<v> argument, so a value starting with "-" is never read as a flag (ADR-0079).
func pushArgs(ref string, pushOptions []string) []string {
	args := []string{"push"}
	for _, o := range pushOptions {
		args = append(args, "--push-option="+o)
	}
	return append(args, "origin", ref)
}

// withPushOptions appends a push's options to its [dry-run] line, as git receives them.
func withPushOptions(line string, pushOptions []string) string {
	if len(pushOptions) == 0 {
		return line
	}
	flags := make([]string, len(pushOptions))
	for i, o := range pushOptions {
		flags[i] = "--push-option=" + o
	}
	return line + " (" + strings.Join(flags, " ") + ")"
}

func commitMessage(template, version string) string {
	if template == "" {
		template = defaultCommitMessage
	}
	return strings.ReplaceAll(template, "${version}", version)
}
