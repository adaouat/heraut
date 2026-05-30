package platforms

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveGlobs expands each glob pattern and returns all matched file paths,
// skipping directories. Returns an error when a pattern matches nothing (strict mode).
func ResolveGlobs(patterns []string) ([]string, error) {
	return resolveGlobs(patterns, func(p string) error {
		return fmt.Errorf("no files matched asset pattern %q", p)
	})
}

// ResolveGlobsLenient expands each glob pattern like ResolveGlobs but calls warn
// instead of returning an error when a pattern matches nothing. Invalid glob syntax
// still returns an error.
func ResolveGlobsLenient(patterns []string, warn func(string)) ([]string, error) {
	return resolveGlobs(patterns, func(p string) error {
		warn(p)
		return nil
	})
}

func resolveGlobs(patterns []string, onEmpty func(string) error) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			if err := onEmpty(pattern); err != nil {
				return nil, err
			}
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
