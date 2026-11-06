package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning/branch"
)

// commitPushBranch returns the branch the changelog commit is pushed to. It is empty when a branch
// is checked out, where `git push origin HEAD` updates that branch. From a detached HEAD (GitLab CI
// checks out the commit) it is the CI job's branch, which must have been fetched as
// origin/<branch>: pushing to a branch the remote may not have would create one, and a pull
// request's merge ref is never a branch to release from.
func commitPushBranch(runner port.Runner) (string, error) {
	// symbolic-ref names the branch even before its first commit, where rev-parse HEAD fails; it
	// fails only when HEAD is not a branch.
	if out, _, err := runner.Run("git", "symbolic-ref", "-q", "--short", "HEAD"); err == nil && strings.TrimSpace(out) != "" {
		return "", nil
	}
	name, ok := branch.CIBranch()
	if !ok {
		return "", &CheckoutError{msg: "HEAD is detached and no CI branch variable names the branch to push the changelog commit to: check out the branch first (git checkout -B <branch>)"}
	}
	if _, _, err := runner.Run("git", "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+name); err != nil {
		return "", &CheckoutError{msg: fmt.Sprintf("HEAD is detached and origin/%s was not fetched, so the changelog commit has no branch to go to: release from a branch pipeline, or check out the branch first (git checkout -B <branch>)", name)}
	}
	return name, nil
}
