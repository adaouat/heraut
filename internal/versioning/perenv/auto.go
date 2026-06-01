package perenv

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

func resolveAuto(runner port.Runner, cfg *config.Config, env string, calc VersionCalculator) (versioning.Result, error) {
	tf := tagFormat(cfg, env)

	glob, err := tagfmt.GlobPattern(tf, env)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("building tag glob for %q: %w", env, err)
	}

	stdout, _, err := runner.Run("git", "tag", "-l", glob, "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing tags for env %q: %w", env, err)
	}

	rawTags := splitLines(stdout)

	// Parse bare versions; skip tags that don't match the format.
	var bareVersions []string
	var latestTag string
	for _, tag := range rawTags {
		bare, parseErr := tagfmt.ParseVersion(tf, tag)
		if parseErr == nil {
			if latestTag == "" {
				latestTag = tag
			}
			bareVersions = append(bareVersions, bare)
		}
	}

	var nextVersion string
	if cfg.Versioning.Strategy == "calver-per-env" {
		nextVersion, err = calc.BumpFromDate(bareVersions)
		if err != nil {
			return versioning.Result{}, fmt.Errorf("computing next version: %w", err)
		}
	} else {
		// semver-per-env: fetch commits since the latest tag (skip if no tags yet).
		var commits []string
		if latestTag != "" {
			commits, err = fetchCommitsSince(runner, latestTag)
			if err != nil {
				return versioning.Result{}, err
			}
		}
		nextVersion, err = calc.BumpAuto(bareVersions, commits)
		if err != nil {
			return versioning.Result{}, fmt.Errorf("computing next version: %w", err)
		}
	}

	newTag, err := tagfmt.Render(tf, env, nextVersion, "")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("rendering tag: %w", err)
	}

	return versioning.Result{
		Version:    nextVersion,
		Tag:        newTag,
		CurrentTag: latestTag,
	}, nil
}

func fetchCommitsSince(runner port.Runner, sinceTag string) ([]string, error) {
	stdout, _, err := runner.Run("git", "log", sinceTag+"..HEAD", "--format=%B%x00")
	if err != nil {
		return nil, fmt.Errorf("reading git log: %w", err)
	}
	return parseCommits(stdout), nil
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseCommits(s string) []string {
	var commits []string
	for _, chunk := range strings.Split(s, "\x00") {
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			commits = append(commits, chunk)
		}
	}
	return commits
}
