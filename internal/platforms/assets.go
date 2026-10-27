package platforms

import (
	"fmt"
	"os"
	"path/filepath"
)

// Asset is a release asset opened for upload. Path is the match as the glob returned it, for
// messages; Name is its basename, the name it is published under. Size comes from the open file.
type Asset struct {
	Path string
	Name string
	Size int64
	File *os.File
}

// afterStat and beforeOpen run around an asset's confinement check, so tests can swap the file there.
var (
	afterStat  = func(string) {}
	beforeOpen = func(string) {}
)

// OpenAssets resolves the asset globs with the rules of matchAssets and opens every match, so an
// upload reads the file that was checked, not whatever the path names by the time it is read
// (NEW-002, ADR-0077). The open goes through an os.Root on the repository root, which refuses a
// path that leaves it even if a symlink is swapped in after the check, and os.SameFile confirms the
// descriptor is the file that was checked. On success the caller closes the files with
// CloseAssets; on error none is left open.
func OpenAssets(patterns []string, warn func(string)) ([]Asset, error) {
	rootPath, err := realWorkingDir()
	if err != nil {
		return nil, err
	}
	candidates, err := matchAssets(rootPath, patterns, warn)
	if err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("opening the repository root %q: %w", rootPath, err)
	}
	defer func() { _ = root.Close() }()

	assets := make([]Asset, 0, len(candidates))
	for _, c := range candidates {
		a, err := openChecked(root, c.path, c.rel, c.checked)
		if err != nil {
			CloseAssets(assets)
			return nil, err
		}
		assets = append(assets, a)
	}
	return assets, nil
}

// candidate is a glob match that passed matchAssets: path as the glob returned it, rel its real
// path relative to the repository root, checked what os.Stat saw.
type candidate struct {
	path, rel string
	checked   os.FileInfo
}

// matchAssets expands the globs under the publishing rules, without opening anything: directories
// are skipped, anything else that is not a regular file is refused (opening a FIFO would block the
// release), every match must lie inside rootPath once symlinks are resolved, and two matches with
// the same basename are refused, since they would be published under one name. A pattern with no
// file calls warn. Both the pre-release check and publishing apply these rules.
func matchAssets(rootPath string, patterns []string, warn func(string)) ([]candidate, error) {
	var candidates []candidate
	pathByName := map[string]string{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		matched := false
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil {
				return nil, fmt.Errorf("stat %q: %w", m, err)
			}
			if info.IsDir() {
				continue
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("asset %q is not a regular file: only files can be published", m)
			}
			afterStat(m)
			rel, err := realPathInside(rootPath, m)
			if err != nil {
				return nil, err
			}
			name := filepath.Base(m)
			if prev, ok := pathByName[name]; ok {
				return nil, fmt.Errorf("assets %q and %q would both be published as %q: rename one or narrow the patterns", prev, m, name)
			}
			pathByName[name] = m
			candidates = append(candidates, candidate{path: m, rel: rel, checked: info})
			matched = true
		}
		if !matched {
			warn(pattern)
		}
	}
	return candidates, nil
}

func openChecked(root *os.Root, path, rel string, checked os.FileInfo) (Asset, error) {
	beforeOpen(path)
	f, err := root.OpenFile(rel, assetOpenFlags, 0)
	if err != nil {
		return Asset{}, fmt.Errorf("opening asset %q: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return Asset{}, fmt.Errorf("stat %q: %w", path, err)
	}
	if !os.SameFile(checked, info) {
		_ = f.Close()
		return Asset{}, fmt.Errorf("asset %q changed after it was checked: refusing to publish it", path)
	}
	return Asset{Path: path, Name: filepath.Base(path), Size: info.Size(), File: f}, nil
}

// CloseAssets closes every file OpenAssets returned.
func CloseAssets(assets []Asset) {
	for _, a := range assets {
		_ = a.File.Close()
	}
}
