package commitwizard

import (
	"fmt"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// hasStaged reports whether the index has staged changes. Uses --name-only (empty output
// = nothing staged) rather than --quiet's exit code, which keeps the check trivial to
// model with MockRunner.
func hasStaged(r port.Runner) (bool, error) {
	out, _, err := r.Run("git", "diff", "--cached", "--name-only")
	if err != nil {
		return false, fmt.Errorf("checking staged changes: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

// stageAll runs `git add -A`.
func stageAll(r port.Runner) error {
	if _, _, err := r.Run("git", "add", "-A"); err != nil {
		return fmt.Errorf("staging all changes: %w", err)
	}
	return nil
}

// commit writes message to a temp file and runs `git commit -F <file>` (plus -a when all),
// matching the temp-file pattern the gitcliff generator uses. The temp file is always
// removed, including on error paths.
func commit(r port.Runner, message string, all bool) error {
	f, err := os.CreateTemp("", "heraut-commit-*.txt")
	if err != nil {
		return fmt.Errorf("creating temp commit message file: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()

	if _, err := f.WriteString(message); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing temp commit message file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing temp commit message file: %w", err)
	}

	args := []string{"commit"}
	if all {
		args = append(args, "-a")
	}
	args = append(args, "-F", f.Name())
	if _, _, err := r.Run("git", args...); err != nil {
		return fmt.Errorf("running git commit: %w", err)
	}
	return nil
}
