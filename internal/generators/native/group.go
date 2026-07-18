package native

import (
	"regexp"
	"sort"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
)

// parsedCommit pairs a rawCommit with its optional conventional-commit metadata.
// parsed is nil when the subject does not conform to the Conventional Commits grammar.
type parsedCommit struct {
	raw    rawCommit
	parsed *conventionalcommit.Commit
}

// group is one classified commit bucket produced by groupCommits for rendering. Groups are
// sorted by order ascending. Commits within each group are scoped-first (scope ascending),
// then unscoped; oldest-first (input order) is the stable tiebreak.
type group struct {
	name    string
	order   int
	commits []parsedCommit
}

// Built-in rendering-only group orders, placed after the config-derived type groups (which
// use commits.types order, or unorderedTypeOrder when a type has no order). These three are
// not commits.types entries — adding them as types would change the verify allow-list — so
// the renderer owns them: the security body-rule, revert, and the catch-all for unmatched /
// non-conventional commits (ADR-0033, T132/T134).
const (
	unorderedTypeOrder = 100
	orderSecurity      = 101
	orderRevert        = 102
	orderOther         = 103

	securityGroup = "🛡️ Security"
	revertGroup   = "◀️ Revert"
	otherGroup    = "💼 Other"
)

// groupCommits classifies commits into ordered, scope-sorted groups, driven by config
// (ADR-0033): type groups come from config.EffectiveTypes(userTypes); commits matching
// config.EffectiveExcludes(userExcludes), merge commits, and fixup commits are dropped. A
// commit whose type is not a configured type falls to the built-in security body-rule,
// revert, or catch-all "Other" group, so nothing is silently dropped. The input is expected
// oldest-first; that order is the stable within-group tiebreak.
func groupCommits(commits []rawCommit, userTypes []config.TypeRule, userExcludes []config.Exclude) []group {
	typeIndex := buildTypeIndex(config.EffectiveTypes(userTypes))
	excTypes, excRes := buildExcludes(config.EffectiveExcludes(userExcludes))

	byName := make(map[string]int)
	var groups []group

	for _, raw := range commits {
		msg := commitMessage(raw)
		if conventionalcommit.IsMergeCommit(msg) || conventionalcommit.IsFixupCommit(msg) {
			continue
		}
		parsed, _ := conventionalcommit.Parse(msg)

		if isExcluded(raw, parsed, excTypes, excRes) {
			continue
		}

		name, order := classify(raw, parsed, typeIndex)
		pc := parsedCommit{raw: raw, parsed: parsed}
		if idx, ok := byName[name]; ok {
			groups[idx].commits = append(groups[idx].commits, pc)
		} else {
			byName[name] = len(groups)
			groups = append(groups, group{name: name, order: order, commits: []parsedCommit{pc}})
		}
	}

	sort.SliceStable(groups, func(i, j int) bool { return groups[i].order < groups[j].order })
	for i := range groups {
		sortWithinGroup(groups[i].commits)
	}
	return groups
}

type typeInfo struct {
	label string
	order int
}

// buildTypeIndex maps each effective type name to its section label and display order. A type
// with no render label joins the catch-all "Other" group (matching git-cliff's `.*` parser —
// e.g. the default `build` type), rather than forming a bare capitalized section. An unset order
// sorts a labelled type after the ordered ones (but before the built-in fallbacks).
func buildTypeIndex(types []config.TypeRule) map[string]typeInfo {
	idx := make(map[string]typeInfo, len(types))
	for _, t := range types {
		if t.Render == "" {
			idx[t.Name] = typeInfo{label: otherGroup, order: orderOther}
			continue
		}
		order := unorderedTypeOrder
		if t.Order != nil {
			order = *t.Order
		}
		idx[t.Name] = typeInfo{label: t.Render, order: order}
	}
	return idx
}

// buildExcludes splits the effective excludes into a type set and compiled subject regexes.
// The config validator rejects invalid regexes upstream; compiling defensively (skip on error,
// like resolveTickets) keeps the generator from panicking if it is ever run on unvalidated
// config — e.g. heraut embedded as a library.
func buildExcludes(excludes []config.Exclude) (map[string]bool, []*regexp.Regexp) {
	types := make(map[string]bool)
	var res []*regexp.Regexp
	for _, e := range excludes {
		if e.Type != "" {
			types[e.Type] = true
		}
		if e.Regex != "" {
			re, err := regexp.Compile(e.Regex)
			if err != nil {
				continue
			}
			res = append(res, re)
		}
	}
	return types, res
}

// isExcluded reports whether a commit is dropped by rendering.excludes: a {type} match on the
// parsed type, or a {regex} match on the subject.
func isExcluded(raw rawCommit, parsed *conventionalcommit.Commit, excTypes map[string]bool, excRes []*regexp.Regexp) bool {
	if parsed != nil && excTypes[parsed.Type] {
		return true
	}
	for _, re := range excRes {
		if re.MatchString(raw.Subject) {
			return true
		}
	}
	return false
}

// classify returns the group name and order for a commit: its configured type group, or one
// of the built-in fallbacks. The fallback priority — security body-rule, then revert, then
// the catch-all "Other" — mirrors the previous (git-cliff-derived) taxonomy ordering.
func classify(raw rawCommit, parsed *conventionalcommit.Commit, typeIndex map[string]typeInfo) (string, int) {
	if parsed != nil {
		if ti, ok := typeIndex[parsed.Type]; ok {
			return ti.label, ti.order
		}
	}
	if strings.Contains(raw.Body, "security") {
		return securityGroup, orderSecurity
	}
	if parsed != nil && parsed.Type == "revert" {
		return revertGroup, orderRevert
	}
	return otherGroup, orderOther
}

// commitMessage builds the full commit message from a rawCommit: subject as line 1, body
// after a blank line when present.
func commitMessage(raw rawCommit) string {
	if raw.Body == "" {
		return raw.Subject
	}
	return raw.Subject + "\n\n" + raw.Body
}

// sortWithinGroup sorts commits in place: scoped commits first (scope ascending), then
// unscoped; the input (oldest-first) order is the stable tiebreak within each bucket.
func sortWithinGroup(commits []parsedCommit) {
	sort.SliceStable(commits, func(i, j int) bool {
		si := commitScope(commits[i])
		sj := commitScope(commits[j])
		hasI := si != ""
		hasJ := sj != ""
		if hasI != hasJ {
			return hasI
		}
		if hasI {
			return si < sj
		}
		return false
	})
}

// commitScope returns the conventional-commit scope for pc, or "" when unavailable.
func commitScope(pc parsedCommit) string {
	if pc.parsed != nil {
		return pc.parsed.Scope
	}
	return ""
}

// overlayAuthorHandles stamps each commit's resolved author handle (sha → handle) onto the grouped
// commits, so the renderer can credit the commit author independently of any associated PR.
func overlayAuthorHandles(groups []group, authors map[string]string) {
	if len(authors) == 0 {
		return
	}
	for gi := range groups {
		for ci := range groups[gi].commits {
			if h, ok := authors[groups[gi].commits[ci].raw.Hash]; ok {
				groups[gi].commits[ci].raw.AuthorHandle = h
			}
		}
	}
}
