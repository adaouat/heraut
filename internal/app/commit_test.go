package app_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestVerifyCommit_DefaultTypes_Valid(t *testing.T) {
	err := app.VerifyCommit(nil, "feat: add x")
	assert.NoError(t, err)
}

func TestVerifyCommit_DefaultTypes_RejectsUnknownType(t *testing.T) {
	err := app.VerifyCommit(nil, "wip: not a real type")
	assert.Error(t, err)
}

func TestVerifyCommit_ScopesNotEnforcedByDefault(t *testing.T) {
	// commits.scopes gates verify only when scopes_restricted is true (added in T131). With it
	// unset (the default), VerifyCommit accepts any scope — one outside the list, and no scope
	// at all.
	cfg := &config.Config{Commits: &config.Commits{
		Types:  []config.TypeRule{{Name: "feat"}, {Name: "fix"}},
		Scopes: []config.ScopeRule{{Name: "cmd"}, {Name: "config"}},
	}}
	assert.NoError(t, app.VerifyCommit(cfg, "feat(elsewhere): outside the scope list"))
	assert.NoError(t, app.VerifyCommit(cfg, "feat: no scope at all"))
}

func TestVerifyCommit_ScopesRestricted_RejectsOutsideList(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Scopes:           []config.ScopeRule{{Name: "cmd"}, {Name: "config"}},
		ScopesRestricted: true,
	}}
	assert.NoError(t, app.VerifyCommit(cfg, "feat(cmd): an allowed scope"))
	assert.NoError(t, app.VerifyCommit(cfg, "feat: no scope is allowed under restriction"))
	assert.Error(t, app.VerifyCommit(cfg, "feat(elsewhere): scope not in the list"))
}

func TestVerifyCommit_DefaultTypes_AllAccepted(t *testing.T) {
	for _, typ := range app.DefaultCommitTypes {
		err := app.VerifyCommit(nil, typ+": something")
		assert.NoError(t, err, "type %q should be accepted by default", typ)
	}
}

func TestVerifyCommit_TypesMergeOverDefaults(t *testing.T) {
	// commits.types MERGES over the built-in defaults (ADR-0033): listing a type adds/overrides
	// it but does NOT restrict the allow-list to only the listed types.
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "deps"}}}}
	assert.NoError(t, app.VerifyCommit(cfg, "deps: bump x"), "a listed custom type is allowed")
	assert.NoError(t, app.VerifyCommit(cfg, "docs: update readme"), "default types remain allowed under merge")
}

func TestVerifyCommit_RemoveDropsDefaultType(t *testing.T) {
	// remove:true drops a default type from the allow-list (ADR-0033).
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "docs", Remove: true}}}}
	assert.NoError(t, app.VerifyCommit(cfg, "feat: still allowed"))
	assert.Error(t, app.VerifyCommit(cfg, "docs: removed from the allow-list"))
}

func TestVerifyCommit_InvalidGrammar_Errors(t *testing.T) {
	err := app.VerifyCommit(nil, "not a conventional commit at all")
	assert.Error(t, err)
}

func TestVerifyCommit_MergeCommit_Skipped(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "docs", Remove: true}}}}
	err := app.VerifyCommit(cfg, "Merge branch 'main' into feature/x")
	assert.NoError(t, err)
}

func TestVerifyCommit_FixupCommit_Skipped(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "docs", Remove: true}}}}
	err := app.VerifyCommit(cfg, "fixup! docs: typo") // docs removed — would reject if not skipped
	assert.NoError(t, err)
}

func TestVerifyCommit_SquashCommit_Skipped(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "docs", Remove: true}}}}
	err := app.VerifyCommit(cfg, "squash! docs: typo")
	assert.NoError(t, err)
}

func TestVerifyCommit_EmptyTypes_UsesDefaults(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{}}}
	err := app.VerifyCommit(cfg, "feat: add x")
	assert.NoError(t, err)
}

func TestAllowedCommitTypes(t *testing.T) {
	t.Run("nil config returns defaults", func(t *testing.T) {
		assert.Equal(t, app.DefaultCommitTypes, app.AllowedCommitTypes(nil))
	})
	t.Run("empty commits returns defaults", func(t *testing.T) {
		cfg := &config.Config{Commits: &config.Commits{}}
		assert.Equal(t, app.DefaultCommitTypes, app.AllowedCommitTypes(cfg))
	})
	t.Run("custom type merges in", func(t *testing.T) {
		cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "deps"}}}}
		assert.Contains(t, app.AllowedCommitTypes(cfg), "deps")
		assert.Contains(t, app.AllowedCommitTypes(cfg), "feat")
	})
	t.Run("remove drops a default", func(t *testing.T) {
		cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "build", Remove: true}}}}
		assert.NotContains(t, app.AllowedCommitTypes(cfg), "build")
		assert.Contains(t, app.AllowedCommitTypes(cfg), "feat")
	})
}
