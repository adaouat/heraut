package native

import (
	"regexp"
	"sort"

	"github.com/adaouat/heraut/internal/conventionalcommit"
)

// parsedCommit pairs a rawCommit with its optional conventional-commit metadata.
// parsed is nil when the subject does not conform to the Conventional Commits grammar.
// T124 uses raw for hash/author/date and parsed for type/scope/breaking/description.
type parsedCommit struct {
	raw    rawCommit
	parsed *conventionalcommit.Commit
}

// group is one classified commit bucket produced by groupCommits for rendering by T124.
// Groups are sorted by order ascending. Commits within each group are scoped-first
// (scope sorted ascending), then unscoped; oldest-first (input order) is the stable tiebreak.
type group struct {
	name    string
	order   int // display sort position (0 = first), mirrors <!-- N --> in git-cliff TOML
	commits []parsedCommit
}

// matchRule is one entry in the ordered taxonomy table (evaluated top-to-bottom,
// first-match-wins).
type matchRule struct {
	re     *regexp.Regexp
	isBody bool   // if true, re is matched against rawCommit.Body instead of Subject
	name   string // group display name; meaningless when skip is true
	order  int    // display sort position; meaningless when skip is true
	skip   bool   // if true, the matching commit is excluded entirely
}

// commitRules is the taxonomy table, evaluated in array order (first-match-wins).
// It mirrors internal/generators/gitcliff/cliff.changelog.toml commit_parsers exactly:
//   - array position = match priority (first match wins, same as git-cliff's array)
//   - order field    = display sort position (the <!-- N --> prefix stripped at render time)
//
// Two rule kinds:
//   - message rule (isBody=false): re matched against rawCommit.Subject
//   - body rule    (isBody=true):  re matched against rawCommit.Body  (the security rule)
var commitRules = []matchRule{
	{re: regexp.MustCompile(`^feat`), name: "🚀 Features", order: 0},
	{re: regexp.MustCompile(`^fix`), name: "🐛 Bug Fixes", order: 1},
	{re: regexp.MustCompile(`^doc`), name: "📚 Documentation", order: 3},
	{re: regexp.MustCompile(`^perf`), name: "⚡ Performance", order: 4},
	{re: regexp.MustCompile(`^refactor`), name: "🚜 Refactor", order: 2},
	{re: regexp.MustCompile(`^style`), name: "🎨 Styling", order: 5},
	{re: regexp.MustCompile(`^test`), name: "🧪 Testing", order: 6},
	{re: regexp.MustCompile(`^chore\(release\):`), skip: true},
	{re: regexp.MustCompile(`^chore\(deps.*\)`), skip: true},
	{re: regexp.MustCompile(`^chore\(pr\)`), skip: true},
	{re: regexp.MustCompile(`^chore\(pull\)`), skip: true},
	{re: regexp.MustCompile(`^chore|^ci`), name: "⚙️ Miscellaneous Tasks", order: 7},
	{re: regexp.MustCompile(`.*security`), isBody: true, name: "🛡️ Security", order: 8},
	{re: regexp.MustCompile(`^revert`), name: "◀️ Revert", order: 9},
	{re: regexp.MustCompile(`.*`), name: "💼 Other", order: 10},
}

// groupCommits classifies commits into ordered, scope-sorted groups, excluding merge
// commits, fixup commits, and subjects matching skip patterns. The input slice is
// expected to be oldest-first; that ordering is preserved as the stable tiebreak within
// each group. Returns an empty slice when no classifiable commits are present.
func groupCommits(commits []rawCommit) []group {
	byName := make(map[string]int) // group name → index in groups
	var groups []group

	for _, raw := range commits {
		msg := commitMessage(raw)
		if conventionalcommit.IsMergeCommit(msg) || conventionalcommit.IsFixupCommit(msg) {
			continue
		}

		rule := classifyCommit(raw)
		if rule == nil || rule.skip {
			continue
		}

		// Parse the conventional commit; non-conventional subjects return (nil, err).
		// Parse errors are intentional here: the commit still lands in its group with
		// parsed=nil, and T124 falls back to raw.Subject for rendering.
		parsed, _ := conventionalcommit.Parse(msg)
		pc := parsedCommit{raw: raw, parsed: parsed}

		if idx, ok := byName[rule.name]; ok {
			groups[idx].commits = append(groups[idx].commits, pc)
		} else {
			byName[rule.name] = len(groups)
			groups = append(groups, group{
				name:    rule.name,
				order:   rule.order,
				commits: []parsedCommit{pc},
			})
		}
	}

	// Sort groups by display order (ascending).
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].order < groups[j].order
	})

	// Within each group: scoped commits first (scope ascending), then unscoped.
	for i := range groups {
		sortWithinGroup(groups[i].commits)
	}

	return groups
}

// commitMessage builds the full commit message string from a rawCommit for passing to
// conventionalcommit functions. Subject is line 1; body follows after a blank line when present.
func commitMessage(raw rawCommit) string {
	if raw.Body == "" {
		return raw.Subject
	}
	return raw.Subject + "\n\n" + raw.Body
}

// classifyCommit walks commitRules top-to-bottom and returns the first matching rule.
// For message rules (isBody=false), the regex is matched against raw.Subject.
// For body rules (isBody=true), the regex is matched against raw.Body.
// Returns nil only if no rule matches (unreachable in practice due to the catch-all).
func classifyCommit(raw rawCommit) *matchRule {
	for i := range commitRules {
		r := &commitRules[i]
		target := raw.Subject
		if r.isBody {
			target = raw.Body
		}
		if r.re.MatchString(target) {
			return r
		}
	}
	return nil
}

// sortWithinGroup sorts commits in place: scoped commits first (by scope ascending),
// then unscoped. sort.SliceStable preserves the input (oldest-first) order as the
// tiebreak within each bucket, matching git-cliff's sort_commits = "oldest" behaviour.
func sortWithinGroup(commits []parsedCommit) {
	sort.SliceStable(commits, func(i, j int) bool {
		si := commitScope(commits[i])
		sj := commitScope(commits[j])
		hasI := si != ""
		hasJ := sj != ""
		if hasI != hasJ {
			return hasI // scoped before unscoped
		}
		if hasI {
			return si < sj // both scoped: sort by scope name ascending
		}
		return false // both unscoped: equal → stable sort preserves input order
	})
}

// commitScope returns the conventional-commit scope for pc, or "" when unavailable
// (non-conventional commit or no scope on an otherwise conventional commit).
func commitScope(pc parsedCommit) string {
	if pc.parsed != nil {
		return pc.parsed.Scope
	}
	return ""
}
