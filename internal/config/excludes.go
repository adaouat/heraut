package config

import "regexp"

// ExcludeMatcher decides whether a commit is dropped by rendering.excludes: a {type} entry matches
// the parsed conventional-commit type, a {regex} entry matches the subject. It is the one
// definition of "excluded" shared by the changelog renderer and the version resolver's
// empty-changelog warning.
type ExcludeMatcher struct {
	types map[string]bool
	res   []*regexp.Regexp
}

// NewExcludeMatcher compiles excludes once. An entry whose regex does not compile is skipped: the
// validator rejects it upstream, so this is only a defensive guard.
func NewExcludeMatcher(excludes []Exclude) *ExcludeMatcher {
	m := &ExcludeMatcher{types: make(map[string]bool)}
	for _, e := range excludes {
		if e.Type != "" {
			m.types[e.Type] = true
		}
		if e.Regex != "" {
			re, err := regexp.Compile(e.Regex)
			if err != nil {
				continue
			}
			m.res = append(m.res, re)
		}
	}
	return m
}

// Excluded reports whether a commit with this subject line is dropped. commitType is the parsed
// conventional-commit type, or "" when the message is not a conventional commit.
func (m *ExcludeMatcher) Excluded(subject, commitType string) bool {
	if commitType != "" && m.types[commitType] {
		return true
	}
	for _, re := range m.res {
		if re.MatchString(subject) {
			return true
		}
	}
	return false
}
