package app

import (
	"errors"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// CheckoutError reports a checkout heraut cannot release from: a shallow clone, or a detached
// HEAD with no branch to push the changelog commit to. Commands map it to the Runtime exit code.
type CheckoutError struct{ msg string }

func (e *CheckoutError) Error() string { return e.msg }

// IsUnusableCheckout reports whether err, or an error it wraps, is a CheckoutError.
func IsUnusableCheckout(err error) bool {
	var ce *CheckoutError
	return errors.As(err, &ce)
}

const unshallowHint = `fetch the full history with git fetch --unshallow --tags (GitHub Actions: actions/checkout with fetch-depth: 0; GitLab CI: GIT_DEPTH: "0")`

// CheckFullHistory refuses a shallow clone. A cut history can hide the last release tag or part of
// the commit range, so the version and the changelog would be wrong. When git cannot answer, it
// returns nil: the other git calls of the run report a broken repository.
func CheckFullHistory(runner port.Runner) error {
	if !isShallow(runner) {
		return nil
	}
	return &CheckoutError{msg: "the repository is a shallow clone, so heraut cannot see every commit and tag to resolve the version and write the changelog; " + unshallowHint}
}

// ShallowCloneWarning is the warning a read-only command prints in a shallow clone, or "" when
// the history is complete.
func ShallowCloneWarning(runner port.Runner) string {
	if !isShallow(runner) {
		return ""
	}
	return "the repository is a shallow clone: tags and commits outside the fetched history are missing, so this answer may be wrong; " + unshallowHint
}

func isShallow(runner port.Runner) bool {
	out, _, err := runner.Run("git", "rev-parse", "--is-shallow-repository")
	return err == nil && strings.TrimSpace(out) == "true"
}
