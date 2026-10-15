package platforms

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveGlobsLenient expands each glob pattern and returns all matched file paths, skipping
// directories. A pattern matching nothing calls warn instead of failing; invalid glob syntax
// still returns an error.
//
// Assets are confined to the working directory (the repository root): a match whose real path,
// after resolving symlinks, lies outside it is refused, so a pattern such as /etc/* or ../* cannot
// publish files that are not part of the project. A relative match whose name starts with "-" is
// returned as "./name" so it cannot be read as an option by the tool it is passed to.
func ResolveGlobsLenient(patterns []string, warn func(string)) ([]string, error) {
	root, err := realWorkingDir()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			warn(pattern)
			continue
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil {
				return nil, fmt.Errorf("stat %q: %w", m, err)
			}
			if info.IsDir() {
				continue
			}
			if err := requireInside(root, m); err != nil {
				return nil, err
			}
			files = append(files, optionSafe(m))
		}
	}
	return files, nil
}

// CheckAssetGlobs applies the publish-time asset rules without publishing, so a refusal surfaces
// before anything is committed or pushed. A pattern matching nothing is not reported here: the
// files may still be built by a later hook, and publishing warns about it.
func CheckAssetGlobs(patterns []string) error {
	_, err := ResolveGlobsLenient(patterns, func(string) {})
	return err
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

// requireInside fails when path, with symlinks resolved, is not under root. The path is made
// absolute before symlinks are resolved so a working directory reached through a symlink compares
// equal to root.
func requireInside(root, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving asset %q: %w", path, err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return fmt.Errorf("resolving asset %q: %w", path, err)
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("asset %q resolves outside the repository (%s): only files under the repository root can be published", path, root)
	}
	return nil
}

func optionSafe(path string) string {
	if !filepath.IsAbs(path) && strings.HasPrefix(path, "-") {
		return "./" + path
	}
	return path
}
