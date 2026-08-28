package app_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyCommit_DefaultTypes_Valid(t *testing.T) {
	_, err := app.VerifyCommit(nil, "feat: add x")
	assert.NoError(t, err)
}

func TestVerifyCommit_DefaultTypes_RejectsUnknownType(t *testing.T) {
	_, err := app.VerifyCommit(nil, "wip: not a real type")
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
	_, err := app.VerifyCommit(cfg, "feat(elsewhere): outside the scope list")
	assert.NoError(t, err)
	_, err = app.VerifyCommit(cfg, "feat: no scope at all")
	assert.NoError(t, err)
}

func TestVerifyCommit_ScopesRestricted_RejectsOutsideList(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Scopes:           []config.ScopeRule{{Name: "cmd"}, {Name: "config"}},
		ScopesRestricted: true,
	}}
	_, err := app.VerifyCommit(cfg, "feat(cmd): an allowed scope")
	assert.NoError(t, err)
	_, err = app.VerifyCommit(cfg, "feat: no scope is allowed under restriction")
	assert.NoError(t, err)
	_, err = app.VerifyCommit(cfg, "feat(elsewhere): scope not in the list")
	assert.Error(t, err)
}

func TestVerifyCommit_DefaultTypes_AllAccepted(t *testing.T) {
	for _, typ := range app.DefaultCommitTypes {
		_, err := app.VerifyCommit(nil, typ+": something")
		assert.NoError(t, err, "type %q should be accepted by default", typ)
	}
}

func TestVerifyCommit_TypesMergeOverDefaults(t *testing.T) {
	// commits.types MERGES over the built-in defaults (ADR-0033): listing a type adds/overrides
	// it but does NOT restrict the allow-list to only the listed types.
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "deps"}}}}
	_, err := app.VerifyCommit(cfg, "deps: bump x")
	assert.NoError(t, err, "a listed custom type is allowed")
	_, err = app.VerifyCommit(cfg, "docs: update readme")
	assert.NoError(t, err, "default types remain allowed under merge")
}

func TestVerifyCommit_RemoveDropsDefaultType(t *testing.T) {
	// remove:true drops a default type from the allow-list (ADR-0033).
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "docs", Remove: true}}}}
	_, err := app.VerifyCommit(cfg, "feat: still allowed")
	assert.NoError(t, err)
	_, err = app.VerifyCommit(cfg, "docs: removed from the allow-list")
	assert.Error(t, err)
}

func TestVerifyCommit_InvalidGrammar_Errors(t *testing.T) {
	_, err := app.VerifyCommit(nil, "not a conventional commit at all")
	assert.Error(t, err)
}

func TestVerifyCommit_MergeCommit_Skipped(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "docs", Remove: true}}}}
	summary, err := app.VerifyCommit(cfg, "Merge branch 'main' into feature/x")
	assert.NoError(t, err)
	assert.Nil(t, summary, "a skipped merge commit has no summary to show")
}

func TestVerifyCommit_FixupCommit_Skipped(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "docs", Remove: true}}}}
	_, err := app.VerifyCommit(cfg, "fixup! docs: typo") // docs removed — would reject if not skipped
	assert.NoError(t, err)
}

func TestVerifyCommit_SquashCommit_Skipped(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "docs", Remove: true}}}}
	_, err := app.VerifyCommit(cfg, "squash! docs: typo")
	assert.NoError(t, err)
}

func TestVerifyCommit_EmptyTypes_UsesDefaults(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{}}}
	_, err := app.VerifyCommit(cfg, "feat: add x")
	assert.NoError(t, err)
}

// TestVerifyCommit_Summary covers T242: a successfully verified conventional commit
// returns a *CommitSummary breaking down type/scope/breaking/description, so
// `heraut commit verify` can print a cocogitto-style recap instead of exiting silently.
func TestVerifyCommit_Summary(t *testing.T) {
	summary, err := app.VerifyCommit(nil, "feat(cmd)!: add a thing")
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, "feat", summary.Type)
	assert.Equal(t, "cmd", summary.Scope)
	assert.True(t, summary.Breaking)
	assert.Equal(t, "add a thing", summary.Description)
	assert.Empty(t, summary.Tickets)
}

// TestVerifyCommit_Summary_DetectsTickets covers the same recap detecting commits.tickets
// matches, using the exact same matching native.MatchTickets applies elsewhere (T241).
func TestVerifyCommit_Summary_DetectsTickets(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Tickets: []config.Ticket{{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/browse/PROJ-{ticket}"}},
	}}
	summary, err := app.VerifyCommit(cfg, "fix: resolve PROJ-42")
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Len(t, summary.Tickets, 1)
	assert.Equal(t, "PROJ-42", summary.Tickets[0].Text)
	assert.Equal(t, "https://jira.example.com/browse/PROJ-42", summary.Tickets[0].Href)
}

func TestVerifyCommit_Summary_NilOnError(t *testing.T) {
	summary, err := app.VerifyCommit(nil, "wip: not a real type")
	require.Error(t, err)
	assert.Nil(t, summary)
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
