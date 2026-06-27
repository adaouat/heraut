// Package native is heraut's built-in, zero-external-dependency content generator
// (ADR-0032). This file covers commit collection: walking a git revision range into
// structured records and resolving the previous tag for compare links. Conventional-commit
// classification (T123), rendering (T124), and remote enrichment (Phase 2) build on it.
package native

import (
	"fmt"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/port"
)

// logFormat is the git-log --format string: six \x01-delimited fields per commit, each
// commit terminated by a \x00 NUL. Body is last because it may contain newlines; the
// field and record separators are control bytes that never occur in commit messages.
// Mirrors the NUL-delimited pattern in internal/versioning/semver/resolver.go.
const logFormat = "%H%x01%an%x01%ae%x01%cI%x01%s%x01%b%x00"

const (
	fieldSep  = "\x01"
	commitSep = "\x00"
	logFields = 6
)

// rawCommit is one commit collected from git history, before conventional-commit parsing
// (T123) or remote enrichment (Phase 2). Fields map 1:1 to the logFormat placeholders.
type rawCommit struct {
	Hash    string    // %H — full hash (truncated at render time)
	Author  string    // %an — author name
	Email   string    // %ae — author email
	Date    time.Time // %cI — committer date (strict ISO-8601)
	Subject string    // %s — subject line
	Body    string    // %b — body without the subject
}

// collectCommits runs `git log [revRange] --format=logFormat` and parses the output into
// rawCommits. An empty revRange walks the full history (no range argument); a range like
// "v1.0.0..v1.1.0" scopes the walk to that span.
func collectCommits(runner port.Runner, revRange string) ([]rawCommit, error) {
	args := []string{"log"}
	if revRange != "" {
		args = append(args, revRange)
	}
	args = append(args, "--format="+logFormat)

	stdout, _, err := runner.Run("git", args...)
	if err != nil {
		return nil, fmt.Errorf("reading git log: %w", err)
	}
	return parseRawCommits(stdout)
}

// parseRawCommits splits the NUL-terminated git-log stream into rawCommits. git joins
// --format entries with a newline, so every record after the first carries a leading "\n"
// from that join; Trim removes it (and any trailing newline %b left before the NUL)
// without touching the field content.
func parseRawCommits(stdout string) ([]rawCommit, error) {
	var commits []rawCommit
	for chunk := range strings.SplitSeq(stdout, commitSep) {
		chunk = strings.Trim(chunk, "\n")
		if chunk == "" {
			continue
		}
		fields := strings.SplitN(chunk, fieldSep, logFields)
		if len(fields) != logFields {
			return nil, fmt.Errorf("malformed git log record: expected %d fields, got %d", logFields, len(fields))
		}
		date, err := time.Parse(time.RFC3339, fields[3])
		if err != nil {
			return nil, fmt.Errorf("parsing commit date %q: %w", fields[3], err)
		}
		commits = append(commits, rawCommit{
			Hash:    fields[0],
			Author:  fields[1],
			Email:   fields[2],
			Date:    date,
			Subject: fields[4],
			Body:    fields[5],
		})
	}
	return commits, nil
}

// previousTag resolves the most recent tag preceding tag, scoped to glob (an env tag glob
// from tagfmt.GlobPattern, computed by the caller — generators may not import tagfmt). An
// empty glob matches all tags. Resolution delegates to `git describe --abbrev=0 <tag>^`,
// which honours git's own topological tag ordering, matching git-cliff's tag walk. A first
// release (no earlier tag) returns "" with a nil error rather than an error.
func previousTag(runner port.Runner, tag, glob string) (string, error) {
	args := []string{"describe", "--tags", "--abbrev=0"}
	if glob != "" {
		args = append(args, "--match", glob)
	}
	args = append(args, tag+"^")

	stdout, stderr, err := runner.Run("git", args...)
	if err != nil {
		if noEarlierTag(stderr) {
			return "", nil
		}
		return "", fmt.Errorf("resolving previous tag for %s: %w", tag, err)
	}
	return strings.TrimSpace(stdout), nil
}

// noEarlierTag reports whether a `git describe` failure means "no tag precedes this commit"
// (a first release) rather than a real error. Mirrors the stderr probes used by
// app.ResolveFromLatestTag (T121): "No names found" (git 2.x) / "No tags can describe".
func noEarlierTag(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "no names found") || strings.Contains(s, "no tags can describe")
}
