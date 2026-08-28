package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// ResolveFromLatestTag returns a rev-range string of the form "<tag>..HEAD".
// With cfg it delegates to CurrentTag (strategy-aware); without cfg it falls
// back to git describe --tags --abbrev=0.
// Returns ("", true, nil) when no tags exist — the caller should warn and check
// full history. Returns ("", false, err) on unexpected git failures.
func ResolveFromLatestTag(runner port.Runner, cfg *config.Config, env string) (string, bool, error) {
	if cfg != nil {
		tag, err := CurrentTag(runner, cfg, env)
		if err != nil {
			if errors.Is(err, errNoTagsFound) {
				return "", true, nil
			}
			return "", false, fmt.Errorf("resolving latest tag: %w", err)
		}
		return tag + "..HEAD", false, nil
	}

	stdout, stderr, err := runner.Run("git", "describe", "--tags", "--abbrev=0")
	if err != nil {
		if strings.Contains(stderr, "No names found") || strings.Contains(stderr, "No tags can describe") {
			return "", true, nil
		}
		return "", false, fmt.Errorf("resolving latest tag via git describe: %w", err)
	}
	return strings.TrimSpace(stdout) + "..HEAD", false, nil
}

// CommitCheckResult records the outcome of validating one commit in a range.
type CommitCheckResult struct {
	SHA     string // git's abbreviated hash (%h) — length varies with repo size/core.abbrev
	Subject string // first line of the message
	Err     error  // nil = valid (or skipped merge/fixup)
}

// CheckCommitRange validates every commit in revRange (or the full history reachable from
// HEAD when revRange is "") against the same grammar and type allow-list VerifyCommit
// applies to a single message. Every commit is evaluated — an invalid commit does not
// stop the scan. Merge and fixup commits are skipped via VerifyCommit's existing,
// unconditional skip — no separate handling needed here.
func CheckCommitRange(runner port.Runner, cfg *config.Config, revRange string) ([]CommitCheckResult, error) {
	args := []string{"log"}
	if revRange != "" {
		args = append(args, revRange)
	}
	args = append(args, "--format=%h%x01%s%x01%B%x00")

	stdout, _, err := runner.Run("git", args...)
	if err != nil {
		return nil, fmt.Errorf("listing commits in range %q: %w", revRange, err)
	}

	var results []CommitCheckResult
	for _, record := range strings.Split(stdout, "\x00") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		fields := strings.SplitN(record, "\x01", 3)
		if len(fields) != 3 {
			continue
		}
		_, err := VerifyCommit(cfg, fields[2])
		results = append(results, CommitCheckResult{
			SHA:     fields[0],
			Subject: fields[1],
			Err:     err,
		})
	}
	return results, nil
}
