package semver

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
)

const defaultPrefix = "v"
const defaultInitialVersion = "0.1.0"

// Resolver resolves the next SemVer version.
type Resolver struct {
	runner          port.Runner
	cfg             *config.Config
	versionOverride string
}

// New constructs a SemVer Resolver.
func New(runner port.Runner, cfg *config.Config) *Resolver {
	return &Resolver{runner: runner, cfg: cfg}
}

// BumpAuto computes the next SemVer from pre-fetched bare versions and commits.
// tags is sorted newest-first with the prefix already stripped; commits are the
// full commit messages since the latest tag. Implements the VersionCalculator
// interface consumed by internal/versioning/perenv.
func (r *Resolver) BumpAuto(tags []string, commits []string) (string, error) {
	if len(tags) == 0 {
		return r.initialVersion(), nil
	}
	currentVersion := tags[0]
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits since %s — create at least one commit before running heraut release", currentVersion)
	}
	bump := DetermineBump(commits)
	return BumpVersion(currentVersion, bump)
}

// BumpFromDate is not used by the SemVer calculator; it satisfies the
// VersionCalculator interface for internal/versioning/perenv.
func (r *Resolver) BumpFromDate(_ []string) (string, error) {
	return "", fmt.Errorf("BumpFromDate is not supported by the SemVer calculator")
}

// SetVersionOverride sets a manual version override (for bump: manual mode).
func (r *Resolver) SetVersionOverride(v string) {
	r.versionOverride = v
}

// Resolve returns the next version result.
func (r *Resolver) Resolve() (versioning.Result, error) {
	if r.cfg.Versioning.Bump == "manual" {
		return r.resolveManual()
	}
	return r.resolveAuto()
}

func (r *Resolver) resolveManual() (versioning.Result, error) {
	if r.versionOverride == "" {
		return versioning.Result{}, fmt.Errorf("manual bump mode requires --version flag")
	}
	prefix := r.prefix()
	return versioning.Result{
		Version: r.versionOverride,
		Tag:     prefix + r.versionOverride,
		Bump:    versioning.BumpNone,
	}, nil
}

func (r *Resolver) resolveAuto() (versioning.Result, error) {
	prefix := r.prefix()

	stdout, _, err := r.runner.Run("git", "tag", "-l", prefix+"*", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing git tags: %w", err)
	}

	tags := parseTags(stdout)
	if len(tags) == 0 {
		iv := r.initialVersion()
		return versioning.Result{
			Version: iv,
			Tag:     prefix + iv,
			Bump:    versioning.BumpNone,
		}, nil
	}

	currentTag := tags[0]
	currentVersion := strings.TrimPrefix(currentTag, prefix)

	// %B gives the full commit message; %x00 is a null-byte separator between commits.
	stdout, _, err = r.runner.Run("git", "log", currentTag+"..HEAD", "--format=%B%x00")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("reading git log: %w", err)
	}

	commits := parseCommits(stdout)
	if len(commits) == 0 {
		return versioning.Result{}, fmt.Errorf("no commits since %s — create at least one commit before running heraut release", currentTag)
	}

	bump := DetermineBump(commits)
	nextVersion, err := BumpVersion(currentVersion, bump)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("bumping version: %w", err)
	}

	return versioning.Result{
		Version:    nextVersion,
		Tag:        prefix + nextVersion,
		CurrentTag: currentTag,
		Bump:       bump,
	}, nil
}

func (r *Resolver) prefix() string {
	if r.cfg.Versioning.Prefix != nil {
		return *r.cfg.Versioning.Prefix
	}
	return defaultPrefix
}

func (r *Resolver) initialVersion() string {
	if r.cfg.Versioning.InitialVersion != "" {
		return r.cfg.Versioning.InitialVersion
	}
	return defaultInitialVersion
}

func parseTags(stdout string) []string {
	var tags []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tags = append(tags, line)
		}
	}
	return tags
}

func parseCommits(stdout string) []string {
	var commits []string
	for _, chunk := range strings.Split(stdout, "\x00") {
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			commits = append(commits, chunk)
		}
	}
	return commits
}
