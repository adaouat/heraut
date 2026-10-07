package app

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

var (
	ErrUnlistedBranch   = errors.New("branch matches no versioning.branches entry")
	ErrAmbiguousBranch  = errors.New("branch matches more than one versioning.branches entry")
	ErrUnderivableRange = errors.New("cannot derive a maintenance range from the branch name")
)

type BranchKind int

const (
	BranchRelease     BranchKind = iota // exact name, no range: today's behaviour
	BranchMaintenance                   // has a Range
	BranchUnlisted                      // no entry matches, or the branch is unknown
)

type BranchMatch struct {
	Branch string // "" when unknown
	Kind   BranchKind
	Range  config.BranchRange // set when Kind == BranchMaintenance
	Rule   int                // index into cfg.Versioning.Branches; -1 when unlisted
}

// CurrentBranch reads the checked-out branch. A detached HEAD (the norm in CI checkouts) falls
// back to the CI providers' branch variables; GitHub's ref name and Azure's source ref only
// count when the ref is a branch, since a tag-triggered run exposes the tag there. ok is false when no source
// yields a branch; err is set only when git itself fails.
func CurrentBranch(runner port.Runner) (branch string, ok bool, err error) {
	out, _, err := runner.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", false, fmt.Errorf("determining current git branch: %w", err)
	}
	branch = strings.TrimSpace(out)
	if branch != "" && branch != "HEAD" {
		return branch, true, nil
	}
	if v := os.Getenv("CI_COMMIT_BRANCH"); v != "" {
		return v, true, nil
	}
	if v := os.Getenv("GITHUB_REF_NAME"); v != "" && os.Getenv("GITHUB_REF_TYPE") == "branch" {
		return v, true, nil
	}
	// Azure's BUILD_SOURCEBRANCHNAME is only the ref's last path segment (release/1.3 → "1.3"),
	// so read the full ref and accept branch refs only — a tag build carries refs/tags/….
	if v, ok := strings.CutPrefix(os.Getenv("BUILD_SOURCEBRANCH"), "refs/heads/"); ok && v != "" {
		return v, true, nil
	}
	return "", false, nil
}

// MatchBranchRule matches branch against cfg.Versioning.Branches. needRange == false skips
// ErrUnderivableRange (the --set-version path): the match is then BranchMaintenance with a zero
// Range, which callers on that path never read.
func MatchBranchRule(cfg *config.Config, branch string, known, needRange bool) (BranchMatch, error) {
	unlisted := BranchMatch{Branch: branch, Kind: BranchUnlisted, Rule: -1}
	if !known {
		unlisted.Branch = ""
		return unlisted, nil
	}

	rules := cfg.Versioning.Branches
	var matched []int
	for i, rule := range rules {
		if branchRuleMatches(rule, branch) {
			matched = append(matched, i)
		}
	}
	switch len(matched) {
	case 0:
		return unlisted, nil
	case 1:
	default:
		quoted := make([]string, len(matched))
		for i, idx := range matched {
			quoted[i] = fmt.Sprintf("%q", rules[idx].Name)
		}
		return BranchMatch{}, fmt.Errorf("%w: entries %s all match branch %q",
			ErrAmbiguousBranch, strings.Join(quoted, ", "), branch)
	}

	idx := matched[0]
	rule := rules[idx]
	match := BranchMatch{Branch: branch, Kind: BranchMaintenance, Rule: idx}
	switch {
	case rule.Range != "":
		r, err := config.ParseBranchRange(rule.Range)
		if err != nil {
			return BranchMatch{}, fmt.Errorf("versioning.branches[%d] (%q): %w", idx, rule.Name, err)
		}
		match.Range = r
	case rule.IsGlob():
		r, ok := config.DeriveBranchRange(branch)
		switch {
		case ok:
			match.Range = r
		case needRange:
			return BranchMatch{}, fmt.Errorf("%w: branch %q matches versioning.branches[%d] (%q) but its name carries no N.x / N.N.x / N.N version — add range: to the entry",
				ErrUnderivableRange, branch, idx, rule.Name)
		}
	default:
		match.Kind = BranchRelease
	}
	return match, nil
}

func branchRuleMatches(rule config.BranchRule, branch string) bool {
	if !rule.IsGlob() {
		return rule.Name == branch
	}
	ok, err := path.Match(rule.Name, branch)
	return err == nil && ok
}

// CheckReleaseBranch refuses publishing from an unlisted/unknown branch when
// cfg.Versioning.Branches is set. No-op when the block is absent or force is true.
func CheckReleaseBranch(runner port.Runner, cfg *config.Config, force bool) error {
	if len(cfg.Versioning.Branches) == 0 || force {
		return nil
	}
	branch, known, err := CurrentBranch(runner)
	if err != nil {
		return err
	}
	if !known {
		return fmt.Errorf("%w: cannot determine the current branch (detached HEAD, no CI branch variable)",
			ErrUnlistedBranch)
	}
	match, err := MatchBranchRule(cfg, branch, true, false)
	if err != nil {
		return err
	}
	if match.Kind != BranchUnlisted {
		return nil
	}
	names := make([]string, len(cfg.Versioning.Branches))
	for i, rule := range cfg.Versioning.Branches {
		names[i] = fmt.Sprintf("%q", rule.Name)
	}
	return fmt.Errorf("%w: branch %q (pass --force to release anyway)\nconfigured versioning.branches: %s",
		ErrUnlistedBranch, branch, strings.Join(names, ", "))
}
