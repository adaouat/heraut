package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
)

// DefaultCommitTypes is the type allow-list applied when commits.types adds no overrides:
// the names of the built-in default type set (config.EffectiveTypes(nil)).
var DefaultCommitTypes = commitTypeNames(config.EffectiveTypes(nil))

func commitTypeNames(types []config.TypeRule) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.Name
	}
	return names
}

// AllowedCommitTypes returns the effective commit-type allow-list: the names of
// config.EffectiveTypes(commits.types) — the built-in defaults with the user's commits.types
// merged over them. Single source of truth shared by VerifyCommit and the commit wizard.
func AllowedCommitTypes(cfg *config.Config) []string {
	var user []config.TypeRule
	if cfg != nil && cfg.Commits != nil {
		user = cfg.Commits.Types
	}
	return commitTypeNames(config.EffectiveTypes(user))
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
		return fmt.Errorf("validating commit message: %w", err)
	}

	types := AllowedCommitTypes(cfg)
	if !slices.Contains(types, c.Type) {
		return fmt.Errorf("commit type %q is not allowed (allowed: %s)", c.Type, strings.Join(types, ", "))
	}
	return verifyScope(cfg, c.Scope)
}

// verifyScope rejects a scope outside commits.scopes when scopes_restricted is true. A commit
// with no scope is always allowed — the restriction applies only to scopes that are present.
func verifyScope(cfg *config.Config, scope string) error {
	if cfg == nil || cfg.Commits == nil || !cfg.Commits.ScopesRestricted {
		return nil
	}
	if scope == "" {
		return nil
	}
	names := config.ScopeNames(config.EffectiveScopes(cfg.Commits.Scopes))
	if slices.Contains(names, scope) {
		return nil
	}
	return fmt.Errorf("commit scope %q is not allowed (allowed: %s)", scope, strings.Join(names, ", "))
}
