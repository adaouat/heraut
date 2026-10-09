//go:build e2e_forge

package forgeharness

import (
	"fmt"
	"time"
)

// Sweep deletes releases, tags and branches that carry a run id older than olderThan. A release
// is deleted for every old tag (and every old listed release) before the tag goes, because
// deleting a tag first turns its release into a leaked draft. With dryRun it only reports.
// Resources without a run id are never touched.
func Sweep(f Forge, olderThan time.Duration, now time.Time, dryRun bool) ([]string, error) {
	old := func(name string) bool {
		ts, ok := RunIDTime(name)
		return ok && now.Sub(ts) > olderThan
	}
	var report []string
	do := func(kind, name string, del func(string) error) error {
		report = append(report, kind+" "+name)
		if dryRun {
			return nil
		}
		if err := del(name); err != nil {
			return fmt.Errorf("deleting %s %q: %w", kind, name, err)
		}
		return nil
	}

	tags, err := f.Tags("e2e-")
	if err != nil {
		return report, fmt.Errorf("listing tags: %w", err)
	}
	listed, err := f.Releases("e2e-")
	if err != nil {
		return report, fmt.Errorf("listing releases: %w", err)
	}
	seen := map[string]bool{}
	for _, name := range append(append([]string{}, listed...), tags...) {
		if seen[name] || !old(name) {
			continue
		}
		seen[name] = true
		if err := do("release", name, f.DeleteRelease); err != nil {
			return report, err
		}
	}
	for _, name := range tags {
		if !old(name) {
			continue
		}
		if err := do("tag", name, f.DeleteTag); err != nil {
			return report, err
		}
	}
	branches, err := f.Branches("e2e/")
	if err != nil {
		return report, fmt.Errorf("listing branches: %w", err)
	}
	for _, name := range branches {
		if !old(name) {
			continue
		}
		if err := do("branch", name, f.DeleteBranch); err != nil {
			return report, err
		}
	}
	return report, nil
}
