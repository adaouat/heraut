package platforms

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveGlobs expands each glob pattern and returns all matched file paths,
// skipping directories so that globs like "dist/*" never pass a directory to a release CLI.
func ResolveGlobs(patterns []string) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no files matched asset pattern %q", pattern)
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil {
				return nil, fmt.Errorf("stat %q: %w", m, err)
			}
			if !info.IsDir() {
				files = append(files, m)
			}
		}
	}
	return files, nil
}
