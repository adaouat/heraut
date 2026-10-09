//go:build e2e_forge

package forgeharness

import (
	"errors"
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
	var errs []error
	do := func(kind, name string, del func(string) error) {
		report = append(report, kind+" "+name)
		if dryRun {
			return
		}
		if err := del(name); err != nil {
			errs = append(errs, fmt.Errorf("deleting %s %q: %w", kind, name, err))
		}
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
		do("release", name, f.DeleteRelease)
	}
	for _, name := range tags {
		if old(name) {
			do("tag", name, f.DeleteTag)
		}
	}
	branches, err := f.Branches("e2e/")
	if err != nil {
		return report, errors.Join(append(errs, fmt.Errorf("listing branches: %w", err))...)
	}
	for _, name := range branches {
		if old(name) {
			do("branch", name, f.DeleteBranch)
		}
	}
	return report, errors.Join(errs...)
}
