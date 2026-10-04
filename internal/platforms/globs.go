package platforms

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveGlobsLenient expands each glob pattern and returns all matched file paths, skipping
// directories. A pattern matching nothing calls warn instead of failing; invalid glob syntax
// still returns an error.
func ResolveGlobsLenient(patterns []string, warn func(string)) ([]string, error) {
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
			if !info.IsDir() {
				files = append(files, m)
			}
		}
	}
	return files, nil
}
