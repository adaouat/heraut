package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

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
		results = append(results, CommitCheckResult{
			SHA:     fields[0],
			Subject: fields[1],
			Err:     VerifyCommit(cfg, fields[2]),
		})
	}
	return results, nil
}
