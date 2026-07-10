package native

import (
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rc builds a rawCommit fixture for groupCommits tests. The hash identifies commits in
// ordering assertions.
func rc(hash, subject, body string) rawCommit {
	return rawCommit{
		Hash:    hash,
		Author:  "Test Author",
		Email:   "test@example.com",
		Date:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Subject: subject,
		Body:    body,
	}
}

func ptr(n int) *int { return &n }

// TestGroupCommits_DefaultTaxonomy exercises the default type set (config.EffectiveTypes(nil))
// plus the built-in security / revert / catch-all fallbacks. Under ADR-0033 classification is
// by exact parsed type: `build` has no render label so it joins the catch-all "💼 Other" group
// (matching git-cliff's `.*` parser), and a non-default type (e.g. `doc`, `wip`) falls to the
// fallbacks rather than matching a `^doc` prefix.
func TestGroupCommits_DefaultTaxonomy(t *testing.T) {
	tests := []struct {
		name      string
		subject   string
		body      string
		wantGroup string
		wantOrder int
	}{
		{"feat → Features", "feat: add login", "", "🚀 Features", 0},
		{"feat(scope) → Features", "feat(auth): add login", "", "🚀 Features", 0},
		{"fix → Bug Fixes", "fix: correct typo", "", "🐛 Bug Fixes", 1},
		{"refactor → Refactor", "refactor: simplify logic", "", "🚜 Refactor", 2},
		{"docs → Documentation", "docs: add API guide", "", "📚 Documentation", 3},
		{"perf → Performance", "perf: speed up query", "", "⚡ Performance", 4},
		{"style → Styling", "style: format code", "", "🎨 Styling", 5},
		{"test → Testing", "test: add unit tests", "", "🧪 Testing", 6},
		{"chore → Miscellaneous Tasks", "chore: update lock", "", "⚙️ Miscellaneous Tasks", 7},
		{"ci → Miscellaneous Tasks", "ci: update pipeline", "", "⚙️ Miscellaneous Tasks", 7},
		{"build (no render) → Other", "build: compile", "", "💼 Other", 103},
		{
			// The security body-rule fires only for a commit whose type is NOT a configured
			// type (here "wip"), so it is not grouped as a type first.
			name:      "non-type subject + security body → Security",
			subject:   "wip: experiment",
			body:      "this addresses a security vulnerability",
			wantGroup: "🛡️ Security",
			wantOrder: 101,
		},
		{"revert → Revert", "revert: revert feat x", "", "◀️ Revert", 102},
		{"unknown type → Other", "wip: half-done", "", "💼 Other", 103},
		{"non-conventional → Other", "just a plain commit message", "", "💼 Other", 103},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			commit := rc("abc1234", tc.subject, tc.body)
			groups := groupCommits([]rawCommit{commit}, nil, nil)
			require.Len(t, groups, 1, "expected exactly one group")
			assert.Equal(t, tc.wantGroup, groups[0].name)
			assert.Equal(t, tc.wantOrder, groups[0].order)
			require.Len(t, groups[0].commits, 1)
			assert.Equal(t, commit, groups[0].commits[0].raw)
		})
	}
}

// TestGroupCommits_DefaultExcludes verifies the built-in exclude set (heraut release commits
// + dependency / PR-merge noise) drops commits before classification.
func TestGroupCommits_DefaultExcludes(t *testing.T) {
	tests := []struct {
		name    string
		subject string
	}{
		{"chore(release): excluded", "chore(release): v1.2.3"},
		{"chore(deps): excluded", "chore(deps): bump foo from 1.0 to 1.1"},
		{"chore(deps-dev): excluded", "chore(deps-dev): bump bar"},
		{"chore(pr): excluded", "chore(pr): merge pr"},
		{"chore(pull): excluded", "chore(pull): merge pull request"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groups := groupCommits([]rawCommit{rc("skip1", tc.subject, "")}, nil, nil)
			assert.Empty(t, groups, "excluded commit should produce no groups")
		})
	}
}

func TestGroupCommits_MergeAndFixupExcluded(t *testing.T) {
	tests := []struct {
		name    string
		subject string
	}{
		{"merge branch", "Merge branch 'feature/foo' into main"},
		{"merge pull request", "Merge pull request #123 from user/branch"},
		{"merge remote-tracking branch", "Merge remote-tracking branch 'origin/main'"},
		{"fixup commit", "fixup! feat: something"},
		{"squash commit", "squash! fix: the thing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groups := groupCommits([]rawCommit{rc("m1", tc.subject, "")}, nil, nil)
			assert.Empty(t, groups)
		})
	}
}

// TestGroupCommits_ExcludesBeforeClassification verifies excluded commits are dropped even
// though their type (chore) is otherwise a valid group.
func TestGroupCommits_ExcludesBeforeClassification(t *testing.T) {
	commits := []rawCommit{
		rc("s1", "chore(release): 1.2.3", ""),   // excluded (default)
		rc("s2", "chore(deps): bump x", ""),     // excluded (default)
		rc("k1", "chore: update toolchain", ""), // kept → Miscellaneous Tasks
	}
	groups := groupCommits(commits, nil, nil)
	require.Len(t, groups, 1)
	assert.Equal(t, "⚙️ Miscellaneous Tasks", groups[0].name)
	require.Len(t, groups[0].commits, 1)
	assert.Equal(t, "k1", groups[0].commits[0].raw.Hash)
}

// TestGroupCommits_SecurityBodyOnlyForUnmatchedType verifies a commit whose type IS a
// configured type is grouped by type, not reclassified by the security body-rule.
func TestGroupCommits_SecurityBodyOnlyForUnmatchedType(t *testing.T) {
	commit := rc("ci1", "ci: update pipeline", "fix a security issue")
	groups := groupCommits([]rawCommit{commit}, nil, nil)
	require.Len(t, groups, 1)
	assert.Equal(t, "⚙️ Miscellaneous Tasks", groups[0].name,
		"security body-rule must not override a configured type")
}

// TestGroupCommits_GroupDisplayOrder verifies groups sort by display order ascending.
func TestGroupCommits_GroupDisplayOrder(t *testing.T) {
	commits := []rawCommit{
		rc("r1", "revert: something", ""),    // order 102
		rc("f1", "feat: something", ""),      // order 0
		rc("x1", "fix: something", ""),       // order 1
		rc("rf1", "refactor: something", ""), // order 2
	}
	groups := groupCommits(commits, nil, nil)
	require.Len(t, groups, 4)
	assert.Equal(t, "🚀 Features", groups[0].name)
	assert.Equal(t, "🐛 Bug Fixes", groups[1].name)
	assert.Equal(t, "🚜 Refactor", groups[2].name)
	assert.Equal(t, "◀️ Revert", groups[3].name)
}

// TestGroupCommits_DocRefactorDisplayOrder verifies the default order inversion: Refactor
// (order 2) before Documentation (order 3).
func TestGroupCommits_DocRefactorDisplayOrder(t *testing.T) {
	commits := []rawCommit{
		rc("d1", "docs: update guide", ""),
		rc("rf1", "refactor: clean up", ""),
	}
	groups := groupCommits(commits, nil, nil)
	require.Len(t, groups, 2)
	assert.Equal(t, "🚜 Refactor", groups[0].name, "Refactor (order 2) before Documentation (order 3)")
	assert.Equal(t, "📚 Documentation", groups[1].name)
}

// TestGroupCommits_WithinGroupOrdering verifies the within-group sort: scoped first (scope
// ascending), then unscoped; oldest-first input order is the stable tiebreak.
func TestGroupCommits_WithinGroupOrdering(t *testing.T) {
	commits := []rawCommit{
		rc("c1", "feat: no scope oldest", ""),
		rc("c2", "feat(zed): z scope", ""),
		rc("c3", "feat(alpha): a scope", ""),
		rc("c4", "feat: no scope newer", ""),
	}
	groups := groupCommits(commits, nil, nil)
	require.Len(t, groups, 1)
	got := groups[0].commits
	require.Len(t, got, 4)
	assert.Equal(t, "c3", got[0].raw.Hash, "alpha scope first")
	assert.Equal(t, "c2", got[1].raw.Hash, "zed scope second")
	assert.Equal(t, "c1", got[2].raw.Hash, "oldest unscoped first")
	assert.Equal(t, "c4", got[3].raw.Hash, "newer unscoped last")
}

func TestGroupCommits_EmptyInput(t *testing.T) {
	assert.Empty(t, groupCommits(nil, nil, nil))
	assert.Empty(t, groupCommits([]rawCommit{}, nil, nil))
}

func TestGroupCommits_ParsedCommitFields(t *testing.T) {
	t.Run("conventional commit populates parsed", func(t *testing.T) {
		commit := rc("a1", "feat(auth)!: add SSO", "")
		groups := groupCommits([]rawCommit{commit}, nil, nil)
		require.Len(t, groups, 1)
		require.Len(t, groups[0].commits, 1)
		pc := groups[0].commits[0]
		assert.Equal(t, commit, pc.raw)
		require.NotNil(t, pc.parsed)
		assert.Equal(t, "feat", pc.parsed.Type)
		assert.Equal(t, "auth", pc.parsed.Scope)
		assert.True(t, pc.parsed.Breaking)
	})

	t.Run("non-conventional commit has nil parsed", func(t *testing.T) {
		commit := rc("b1", "just a plain commit message", "")
		groups := groupCommits([]rawCommit{commit}, nil, nil)
		require.Len(t, groups, 1)
		pc := groups[0].commits[0]
		assert.Equal(t, commit, pc.raw)
		assert.Nil(t, pc.parsed)
	})
}

// ── config-driven customization (T132) ───────────────────────────────────────

// TestGroupCommits_CustomTypeRenderAndOrder verifies commits.types overrides a default type's
// label/order and adds a new type, both merged over the defaults.
func TestGroupCommits_CustomTypeRenderAndOrder(t *testing.T) {
	userTypes := []config.TypeRule{
		{Name: "feat", Render: "✨ Cool Stuff", Order: ptr(0)},
		{Name: "deps", Render: "📦 Dependencies", Order: ptr(5)},
	}
	commits := []rawCommit{
		rc("f1", "feat: a feature", ""),
		rc("d1", "deps: bump x", ""),
	}
	groups := groupCommits(commits, userTypes, nil)
	require.Len(t, groups, 2)
	assert.Equal(t, "✨ Cool Stuff", groups[0].name)
	assert.Equal(t, "📦 Dependencies", groups[1].name)
}

// TestGroupCommits_RemovedTypeFallsToOther verifies a removed default type's commits fall to
// the catch-all (no longer a recognized type group).
func TestGroupCommits_RemovedTypeFallsToOther(t *testing.T) {
	userTypes := []config.TypeRule{{Name: "docs", Remove: true}}
	groups := groupCommits([]rawCommit{rc("d1", "docs: a doc", "")}, userTypes, nil)
	require.Len(t, groups, 1)
	assert.Equal(t, "💼 Other", groups[0].name)
}

// TestGroupCommits_UserExcludeByType drops commits of an excluded type, on top of the defaults.
func TestGroupCommits_UserExcludeByType(t *testing.T) {
	excludes := []config.Exclude{{Type: "docs"}}
	commits := []rawCommit{
		rc("d1", "docs: drop me", ""),
		rc("f1", "feat: keep me", ""),
	}
	groups := groupCommits(commits, nil, excludes)
	require.Len(t, groups, 1)
	assert.Equal(t, "🚀 Features", groups[0].name)
}

// TestGroupCommits_UserExcludeByRegex drops commits whose subject matches a regex exclude,
// while the default excludes still apply.
func TestGroupCommits_UserExcludeByRegex(t *testing.T) {
	excludes := []config.Exclude{{Regex: "^wip"}}
	commits := []rawCommit{
		rc("w1", "wip: drop me", ""),
		rc("dp1", "chore(deps): also dropped by default", ""),
		rc("f1", "feat: keep me", ""),
	}
	groups := groupCommits(commits, nil, excludes)
	require.Len(t, groups, 1)
	assert.Equal(t, "🚀 Features", groups[0].name)
}
