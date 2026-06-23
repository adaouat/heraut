package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
)

// DefaultCommitTypes is the type allow-list VerifyCommit applies when no
// commit_lint.types override is configured — the 10 types documented in
// workflow.md's commit-type table.
var DefaultCommitTypes = []string{
	"feat", "fix", "docs", "chore", "refactor", "test", "style", "perf", "ci", "build",
}

// VerifyCommit validates message against the conventional-commit grammar and the
// configured (or default) type allow-list. Merge and fixup commits are always skipped,
// unconditionally. cfg may be nil (no .heraut.yml present) — the default type list applies.
func VerifyCommit(cfg *config.Config, message string) error {
	if conventionalcommit.IsMergeCommit(message) || conventionalcommit.IsFixupCommit(message) {
		return nil
	}

	c, err := conventionalcommit.Parse(message)
	if err != nil {
		return err
	}

	types := DefaultCommitTypes
	if cfg != nil && cfg.CommitLint != nil && len(cfg.CommitLint.Types) > 0 {
		types = cfg.CommitLint.Types
	}
	for _, t := range types {
		if c.Type == t {
			return nil
		}
	}
	return fmt.Errorf("commit type %q is not allowed (allowed: %s)", c.Type, strings.Join(types, ", "))
}
