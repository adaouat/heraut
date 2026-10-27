package platforms

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// CheckAssetGlobs applies the publish-time asset rules (matchAssets) without opening anything, so a
// refusal surfaces before anything is committed or pushed: every match must be a regular file inside
// the repository with a basename no other match has, and, unless allowMissing is set, every pattern
// must match at least one file (ADR-0076). The latter is wrapped in port.ErrAssetUnmatched so a dry
// run, where hooks have not built the files, can tell it apart. With allowMissing a pattern matching
// nothing is left for publishing to warn about.
func CheckAssetGlobs(patterns []string, allowMissing bool) error {
	root, err := realWorkingDir()
	if err != nil {
		return err
	}
	var unmatched []string
	if _, err := matchAssets(root, patterns, func(pattern string) { unmatched = append(unmatched, pattern) }); err != nil {
		return err
	}
	if allowMissing || len(unmatched) == 0 {
		return nil
	}
	quoted := make([]string, len(unmatched))
	for i, pattern := range unmatched {
		quoted[i] = fmt.Sprintf("%q", pattern)
	}
	return fmt.Errorf("asset %s %s no file: build the files in a post_bump hook (or earlier), fix the pattern, or set release.allow_missing_assets: true if a later hook builds them: %w",
		pluralPattern(len(unmatched)), strings.Join(quoted, ", ")+matchVerb(len(unmatched)), port.ErrAssetUnmatched)
}

func pluralPattern(n int) string {
	if n == 1 {
		return "pattern"
	}
	return "patterns"
}

func matchVerb(n int) string {
	if n == 1 {
		return " matches"
	}
	return " match"
}

// realWorkingDir is the working directory with symlinks resolved, the root assets must stay in.
func realWorkingDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolving the repository root: %w", err)
	}
	root, err := filepath.EvalSymlinks(wd)
	if err != nil {
		return "", fmt.Errorf("resolving the repository root %q: %w", wd, err)
	}
	return root, nil
}

// realPathInside fails when path, with symlinks resolved, is not under root, and otherwise returns
// its real path relative to root. The path is made absolute before symlinks are resolved so a
// working directory reached through a symlink compares equal to root.
func realPathInside(root, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving asset %q: %w", path, err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolving asset %q: %w", path, err)
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("asset %q resolves outside the repository (%s): only files under the repository root can be published", path, root)
	}
	return rel, nil
}
