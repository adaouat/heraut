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

func TestVerifyCommit_SecurityIsADefaultType(t *testing.T) {
	_, err := app.VerifyCommit(nil, "security(auth): rotate the signing key")
	assert.NoError(t, err)

	cfg := &config.Config{Commits: &config.Commits{Types: []config.TypeRule{{Name: "security", Remove: true}}}}
	_, err = app.VerifyCommit(cfg, "security: patch the token leak")
	assert.Error(t, err, "remove:true takes the built-in type out of the allow-list")
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

// ── commits.rules (ADR-0056) ──────────────────────────────────────────────────

func TestVerifyCommit_Rules_Deny_RejectsMatch(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "no-wip", Deny: `(?i)\bwip\b`, Message: "no WIP allowed"}},
	}}
	_, err := app.VerifyCommit(cfg, "feat: still WIP work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no WIP allowed")
}

func TestVerifyCommit_Rules_Deny_AllowsNonMatch(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "no-wip", Deny: `(?i)\bwip\b`, Message: "no WIP allowed"}},
	}}
	_, err := app.VerifyCommit(cfg, "feat: finished work")
	assert.NoError(t, err)
}

func TestVerifyCommit_Rules_Require_RejectsMissing(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "needs-x", Require: `x`, Message: "must mention x"}},
	}}
	_, err := app.VerifyCommit(cfg, "feat: add y")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must mention x")
}

func TestVerifyCommit_Rules_Require_AllowsMatch(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "needs-x", Require: `x`, Message: "must mention x"}},
	}}
	_, err := app.VerifyCommit(cfg, "feat: add x")
	assert.NoError(t, err)
}

func TestVerifyCommit_Rules_RequireTicket_UsesConfiguredTicketPatterns(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Tickets: []config.Ticket{{Pattern: `[A-Z]{2,}-\d+`, URL: "https://example.com/{ticket}"}},
		Rules:   []config.CommitRule{{Name: "needs-ticket", RequireTicket: true, Message: "reference a ticket"}},
	}}
	_, err := app.VerifyCommit(cfg, "fix: resolve JIRA-42")
	assert.NoError(t, err)

	_, err = app.VerifyCommit(cfg, "fix: no ticket here")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reference a ticket")
}

func TestVerifyCommit_Rules_RequireTicket_DefaultMessage(t *testing.T) {
	// No Message set — evaluateRule synthesizes one naming the rule, since require_ticket
	// alone (unlike deny/require) is common enough to not force a message on every project.
	cfg := &config.Config{Commits: &config.Commits{
		Tickets: []config.Ticket{{Pattern: `[A-Z]{2,}-\d+`, URL: "https://example.com/{ticket}"}},
		Rules:   []config.CommitRule{{Name: "needs-ticket", RequireTicket: true}},
	}}
	_, err := app.VerifyCommit(cfg, "fix: no ticket here")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs-ticket")
}

func TestVerifyCommit_Rules_RefuseBreaking(t *testing.T) {
	tests := []struct {
		name    string
		types   []string
		message string
		refused bool
	}{
		{"plain commit", nil, "feat: add x", false},
		{"bang without scope", nil, "feat!: drop x", true},
		{"bang with scope", nil, "fix(cmd)!: drop x", true},
		{"BREAKING CHANGE footer", nil, "feat: x\n\nBREAKING CHANGE: x is gone", true},
		{"BREAKING-CHANGE footer", nil, "feat: x\n\nBREAKING-CHANGE: x is gone", true},
		{"breaking in body prose only", nil, "feat: x\n\nnot a breaking change", false},
		{"scoped to feat, breaking fix passes", []string{"feat"}, "fix!: drop x", false},
		{"scoped to feat, breaking feat refused", []string{"feat"}, "feat!: drop x", true},
		{"merge commit", nil, "Merge branch 'x'", false},
		{"fixup commit", nil, "fixup! feat!: drop x", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Commits: &config.Commits{
				Rules: []config.CommitRule{{Name: "no-breaking", IsBreaking: true, Refuse: true, Types: tc.types, Message: "cut a major on purpose"}},
			}}
			_, err := app.VerifyCommit(cfg, tc.message)
			if !tc.refused {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cut a major on purpose")
		})
	}
}

func TestVerifyCommit_Rules_Refuse_DefaultMessage(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "no-breaking", IsBreaking: true, Refuse: true}},
	}}
	_, err := app.VerifyCommit(cfg, "feat!: drop x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `commit refused (rule "no-breaking")`)
}

func TestVerifyCommit_Rules_IsBreakingFilter(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "breaking-needs-ok", IsBreaking: true, Require: "x-ok", Message: "needs x-ok"}},
	}}
	_, err := app.VerifyCommit(cfg, "feat: plain")
	assert.NoError(t, err, "a non-breaking commit is not selected")
	_, err = app.VerifyCommit(cfg, "feat!: drop")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs x-ok")
	_, err = app.VerifyCommit(cfg, "feat!: drop x-ok")
	assert.NoError(t, err)
}

func TestVerifyCommit_Rules_RefuseByType(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "no-chore", Types: []string{"chore"}, Refuse: true}},
	}}
	_, err := app.VerifyCommit(cfg, "chore: x")
	require.Error(t, err)
	_, err = app.VerifyCommit(cfg, "fix: x")
	assert.NoError(t, err)
}

func TestVerifyCommit_Rules_Target_Header(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "no-wip-header", Deny: `wip`, Message: "no wip in header", Target: "header"}},
	}}
	_, err := app.VerifyCommit(cfg, "feat: add x\n\nstill wip in the details")
	assert.NoError(t, err, "wip only in the body — header-scoped deny doesn't trigger")
	_, err = app.VerifyCommit(cfg, "feat: wip work")
	require.Error(t, err, "wip in the header triggers")
}

func TestVerifyCommit_Rules_Target_Body(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "no-wip-body", Deny: `wip`, Message: "no wip in body", Target: "body"}},
	}}
	_, err := app.VerifyCommit(cfg, "feat: wip in header only")
	assert.NoError(t, err, "header-only mention doesn't trigger a body-scoped rule")
	_, err = app.VerifyCommit(cfg, "feat: add x\n\nstill wip in the details")
	require.Error(t, err, "wip in the body triggers")
}

func TestVerifyCommit_Rules_Target_Footer_ScopesTicketToTrailer(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Tickets: []config.Ticket{{Pattern: `[A-Z]{2,}-\d+`, URL: "https://example.com/{ticket}"}},
		Rules: []config.CommitRule{{
			Name: "ticket-in-footer", RequireTicket: true, Target: "footer",
			Message: "reference a ticket in a footer trailer",
		}},
	}}
	_, err := app.VerifyCommit(cfg, "fix: resolve JIRA-42")
	require.Error(t, err, "ticket only in the description — footer-scoped rule still fails")

	_, err = app.VerifyCommit(cfg, "fix: resolve the bug\n\nRefs: JIRA-42")
	assert.NoError(t, err, "ticket in an actual footer trailer passes")
}

func TestVerifyCommit_Rules_ScopedByType(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "fix-only", Deny: `wip`, Message: "no wip", Types: []string{"fix"}}},
	}}
	_, err := app.VerifyCommit(cfg, "feat: wip work")
	assert.NoError(t, err, "not a fix commit — rule doesn't apply")
	_, err = app.VerifyCommit(cfg, "fix: wip work")
	require.Error(t, err)
}

func TestVerifyCommit_Rules_ScopedByScope(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{{Name: "cmd-only", Deny: `wip`, Message: "no wip", Scopes: []string{"cmd"}}},
	}}
	_, err := app.VerifyCommit(cfg, "feat(other): wip work")
	assert.NoError(t, err, "scope not in the list — rule doesn't apply")
	_, err = app.VerifyCommit(cfg, "feat(cmd): wip work")
	require.Error(t, err)
}

func TestVerifyCommit_Rules_ViolationsAggregate(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{
		Rules: []config.CommitRule{
			{Name: "no-wip", Deny: `wip`, Message: "no wip"},
			{Name: "no-todo", Deny: `todo`, Message: "no todo"},
		},
	}}
	_, err := app.VerifyCommit(cfg, "feat: wip and todo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no wip")
	assert.Contains(t, err.Error(), "no todo")
}

func TestVerifyCommit_Rules_NoRulesConfigured_NoError(t *testing.T) {
	cfg := &config.Config{Commits: &config.Commits{}}
	_, err := app.VerifyCommit(cfg, "feat: anything goes")
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
