package native

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rc builds a rawCommit fixture for groupCommits tests.
// The hash is used to identify commits in ordering assertions.
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

// TestGroupCommits_Taxonomy exercises every taxonomy entry that produces a named group,
// including the non-conventional catch-all and the security body rule.
func TestGroupCommits_Taxonomy(t *testing.T) {
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
		{"doc → Documentation", "doc: update readme", "", "📚 Documentation", 3},
		{"docs → Documentation", "docs: add API guide", "", "📚 Documentation", 3},
		{"perf → Performance", "perf: speed up query", "", "⚡ Performance", 4},
		{"style → Styling", "style: format code", "", "🎨 Styling", 5},
		{"test → Testing", "test: add unit tests", "", "🧪 Testing", 6},
		{"chore → Miscellaneous Tasks", "chore: update lock", "", "⚙️ Miscellaneous Tasks", 7},
		{"chore(scope) → Miscellaneous Tasks", "chore(tooling): update", "", "⚙️ Miscellaneous Tasks", 7},
		{"ci → Miscellaneous Tasks", "ci: update pipeline", "", "⚙️ Miscellaneous Tasks", 7},
		{
			// The security body rule (isBody=true) sits after ^chore|^ci in the table.
			// It fires only when the subject did not match any earlier rule.
			name:      "security body with unmatched subject → Security",
			subject:   "build: misc change",
			body:      "this addresses a security vulnerability",
			wantGroup: "🛡️ Security",
			wantOrder: 8,
		},
		{"revert → Revert", "revert: revert feat: x", "", "◀️ Revert", 9},
		{"unknown type → Other", "build: compile binaries", "", "💼 Other", 10},
		{"non-conventional → Other (catch-all)", "just a plain commit message", "", "💼 Other", 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			commit := rc("abc1234", tc.subject, tc.body)
			groups := groupCommits([]rawCommit{commit})
			require.Len(t, groups, 1, "expected exactly one group")
			assert.Equal(t, tc.wantGroup, groups[0].name)
			assert.Equal(t, tc.wantOrder, groups[0].order)
			require.Len(t, groups[0].commits, 1)
			assert.Equal(t, commit, groups[0].commits[0].raw)
		})
	}
}

// TestGroupCommits_SkipRules verifies that commits matching skip patterns produce no output.
func TestGroupCommits_SkipRules(t *testing.T) {
	tests := []struct {
		name    string
		subject string
	}{
		{"chore(release): is skipped", "chore(release): v1.2.3"},
		{"chore(deps): is skipped", "chore(deps): bump foo from 1.0 to 1.1"},
		{"chore(deps-dev): is skipped", "chore(deps-dev): bump bar"},
		{"chore(pr) is skipped", "chore(pr): merge pr"},
		{"chore(pull) is skipped", "chore(pull): merge pull request"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groups := groupCommits([]rawCommit{rc("skip1", tc.subject, "")})
			assert.Empty(t, groups, "skipped commit should produce no groups")
		})
	}
}

// TestGroupCommits_MergeAndFixupExcluded verifies merge and fixup commits are excluded
// before any taxonomy classification.
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
			groups := groupCommits([]rawCommit{rc("m1", tc.subject, "")})
			assert.Empty(t, groups)
		})
	}
}

// TestGroupCommits_FirstMatchWins ensures skip rules take priority over the generic
// chore/ci rule: chore(release) and chore(deps*) must be excluded, not grouped.
func TestGroupCommits_FirstMatchWins(t *testing.T) {
	commits := []rawCommit{
		rc("s1", "chore(release): 1.2.3", ""),   // must be skipped
		rc("s2", "chore(deps): bump x", ""),     // must be skipped
		rc("k1", "chore: update toolchain", ""), // must land in Miscellaneous Tasks
	}

	groups := groupCommits(commits)

	require.Len(t, groups, 1, "only the generic chore survives the skip rules")
	assert.Equal(t, "⚙️ Miscellaneous Tasks", groups[0].name)
	require.Len(t, groups[0].commits, 1)
	assert.Equal(t, "k1", groups[0].commits[0].raw.Hash)
}

// TestGroupCommits_SecurityBodyAfterSubjectRules verifies that a commit whose subject
// matches an earlier rule (here: ^chore|^ci) is NOT reclassified by the security body
// rule — first-match-wins holds even when later rules are body-based.
func TestGroupCommits_SecurityBodyAfterSubjectRules(t *testing.T) {
	commit := rc("ci1", "ci: update pipeline", "fix a security issue")
	groups := groupCommits([]rawCommit{commit})

	require.Len(t, groups, 1)
	assert.Equal(t, "⚙️ Miscellaneous Tasks", groups[0].name,
		"security body rule must not override an earlier subject match")
}

// TestGroupCommits_GroupDisplayOrder verifies that groups are returned sorted by their
// display order field (ascending), regardless of the input order.
func TestGroupCommits_GroupDisplayOrder(t *testing.T) {
	commits := []rawCommit{
		rc("r1", "revert: something", ""),    // display order 9
		rc("f1", "feat: something", ""),      // display order 0
		rc("x1", "fix: something", ""),       // display order 1
		rc("rf1", "refactor: something", ""), // display order 2
	}

	groups := groupCommits(commits)
	require.Len(t, groups, 4)
	assert.Equal(t, 0, groups[0].order, "Features must be first")
	assert.Equal(t, 1, groups[1].order, "Bug Fixes second")
	assert.Equal(t, 2, groups[2].order, "Refactor third")
	assert.Equal(t, 9, groups[3].order, "Revert last")
}

// TestGroupCommits_DocRefactorDisplayOrder verifies that Refactor (display order 2) is
// returned before Documentation (display order 3), even though the doc rule has higher
// match priority than refactor in the taxonomy table (mirroring the TOML array order).
func TestGroupCommits_DocRefactorDisplayOrder(t *testing.T) {
	commits := []rawCommit{
		rc("d1", "doc: update guide", ""),
		rc("rf1", "refactor: clean up", ""),
	}

	groups := groupCommits(commits)
	require.Len(t, groups, 2)
	assert.Equal(t, "🚜 Refactor", groups[0].name, "Refactor (order 2) before Documentation (order 3)")
	assert.Equal(t, "📚 Documentation", groups[1].name)
}

// TestGroupCommits_WithinGroupOrdering verifies the within-group sort: scoped commits
// first (sorted by scope ascending), then unscoped; oldest-first input order is the stable
// tiebreak within each bucket.
func TestGroupCommits_WithinGroupOrdering(t *testing.T) {
	// Input is oldest-first: c1 < c2 < c3 < c4.
	commits := []rawCommit{
		rc("c1", "feat: no scope oldest", ""), // unscoped (oldest)
		rc("c2", "feat(zed): z scope", ""),    // scoped, scope "zed"
		rc("c3", "feat(alpha): a scope", ""),  // scoped, scope "alpha"
		rc("c4", "feat: no scope newer", ""),  // unscoped (newer)
	}

	groups := groupCommits(commits)
	require.Len(t, groups, 1)

	got := groups[0].commits
	require.Len(t, got, 4)

	// Scoped first, sorted by scope ascending: alpha < zed.
	assert.Equal(t, "c3", got[0].raw.Hash, "alpha scope first")
	assert.Equal(t, "c2", got[1].raw.Hash, "zed scope second")
	// Unscoped last, original (oldest-first) order preserved by stable sort.
	assert.Equal(t, "c1", got[2].raw.Hash, "oldest unscoped commit first")
	assert.Equal(t, "c4", got[3].raw.Hash, "newer unscoped commit last")
}

// TestGroupCommits_EmptyInput returns empty output without panicking.
func TestGroupCommits_EmptyInput(t *testing.T) {
	assert.Empty(t, groupCommits(nil))
	assert.Empty(t, groupCommits([]rawCommit{}))
}

// TestGroupCommits_ParsedCommitFields verifies that parsedCommit.parsed is populated for
// valid conventional commits and nil for non-conventional ones.
func TestGroupCommits_ParsedCommitFields(t *testing.T) {
	t.Run("conventional commit populates parsed", func(t *testing.T) {
		commit := rc("a1", "feat(auth)!: add SSO", "")
		groups := groupCommits([]rawCommit{commit})
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
		groups := groupCommits([]rawCommit{commit})
		require.Len(t, groups, 1)
		pc := groups[0].commits[0]
		assert.Equal(t, commit, pc.raw)
		assert.Nil(t, pc.parsed)
	})
}
