// Package conventionalcommit parses and inspects commit messages against the
// Conventional Commits 1.0.0 grammar. It validates structure only — it does not enforce a
// type allow-list (that policy decision lives in internal/app.VerifyCommit) and it does
// not replicate the broader commitlint-style rule catalog (casing, length limits,
// signed-off-by, etc.) — see ADR-0027's "Explicitly still out of scope" note.
package conventionalcommit

import (
	"fmt"
	"regexp"
	"strings"
)

// headerPattern matches a conventional-commit header: type, optional (scope), optional
// breaking "!", then ": " and a non-empty description. Anchored and bounded — no
// nested-quantifier patterns — so it stays linear-time on the hot paths documented in
// ADR-0027 (the commit-msg hook and DetermineBump).
var headerPattern = regexp.MustCompile(`^(\w+)(\(([^)]*)\))?(!)?: (.+)$`)

// footerLinePattern matches one footer line: a token — either the literal, case-sensitive
// "BREAKING CHANGE" exception, or a generic hyphenated word-token per the spec — followed
// by ": " or " #", then the value. Reused both to decide whether a trailing paragraph is a
// footer block and to parse each line within it.
var footerLinePattern = regexp.MustCompile(`^(BREAKING CHANGE|[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)*)(: | #)(.*)$`)

// mergeCommitPattern matches git's own merge-commit subject shapes.
var mergeCommitPattern = regexp.MustCompile(`^Merge (branch |pull request |remote-tracking branch )`)

// Footer is one trailer in a commit message's footer block, e.g. "Acked-by: Alice" or
// "BREAKING CHANGE: removes the old flag".
type Footer struct {
	Token string
	Value string
}

// Commit is the structural result of parsing a conventional-commit message.
type Commit struct {
	Type        string
	Scope       string
	Breaking    bool
	Description string
	Body        string
	Footers     []Footer
}

// Parse validates message against the Conventional Commits grammar and returns its
// structural components. It enforces grammar only — callers needing a type allow-list
// apply that policy themselves (see internal/app.VerifyCommit).
func Parse(message string) (*Commit, error) {
	lines := strings.Split(message, "\n")
	header := lines[0]

	m := headerPattern.FindStringSubmatch(header)
	if m == nil {
		return nil, fmt.Errorf(`invalid conventional commit header %q: expected "type(scope)!: description"`, header)
	}

	c := &Commit{
		Type:        m[1],
		Scope:       m[3],
		Breaking:    m[4] == "!",
		Description: m[5],
	}

	if len(lines) > 1 {
		rest := lines[1:]
		if rest[0] != "" {
			return nil, fmt.Errorf("invalid conventional commit: body/footer must be separated from the header by a blank line")
		}
		rest = rest[1:]

		body, footers := parseBodyAndFooters(rest)
		c.Body = body
		c.Footers = footers
		for _, f := range footers {
			if f.Token == "BREAKING CHANGE" || f.Token == "BREAKING-CHANGE" {
				c.Breaking = true
			}
		}
	}

	return c, nil
}

// IsMergeCommit reports whether message is a git-generated merge commit
// ("Merge branch ...", "Merge pull request ...", "Merge remote-tracking branch ...").
func IsMergeCommit(message string) bool {
	return mergeCommitPattern.MatchString(firstLine(message))
}

// IsFixupCommit reports whether message is a git "fixup!"/"squash!" autosquash commit.
func IsFixupCommit(message string) bool {
	line := firstLine(message)
	return strings.HasPrefix(line, "fixup! ") || strings.HasPrefix(line, "squash! ")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// parseBodyAndFooters splits the lines following the header (and its separating blank
// line) into a body string and a footer list. Paragraphs are groups of lines separated by
// blank lines; only the LAST paragraph is a candidate footer block, and only if its first
// line itself looks like a footer token — otherwise the whole remainder is body. This
// mirrors the spec's footer-placement rule and rejects a body paragraph that merely
// mentions footer-shaped text without being structurally a footer.
func parseBodyAndFooters(lines []string) (string, []Footer) {
	paragraphs := splitParagraphs(lines)
	if len(paragraphs) == 0 {
		return "", nil
	}

	last := paragraphs[len(paragraphs)-1]
	if !footerLinePattern.MatchString(last[0]) {
		return strings.Join(joinParagraphs(paragraphs), "\n\n"), nil
	}

	footers := parseFooterBlock(last)
	body := strings.Join(joinParagraphs(paragraphs[:len(paragraphs)-1]), "\n\n")
	return body, footers
}

func splitParagraphs(lines []string) [][]string {
	var paragraphs [][]string
	var current []string
	for _, l := range lines {
		if l == "" {
			if len(current) > 0 {
				paragraphs = append(paragraphs, current)
				current = nil
			}
			continue
		}
		current = append(current, l)
	}
	if len(current) > 0 {
		paragraphs = append(paragraphs, current)
	}
	return paragraphs
}

func joinParagraphs(paragraphs [][]string) []string {
	out := make([]string, len(paragraphs))
	for i, p := range paragraphs {
		out[i] = strings.Join(p, "\n")
	}
	return out
}

// Format renders a Commit back to its canonical Conventional Commits message text.
// It round-trips with Parse: Parse(c.Format()) reproduces c's structural fields.
// Empty scope, body, and footer list are omitted; Breaking adds "!" to the header.
func (c *Commit) Format() string {
	var b strings.Builder
	b.WriteString(c.Type)
	if c.Scope != "" {
		b.WriteString("(" + c.Scope + ")")
	}
	if c.Breaking {
		b.WriteString("!")
	}
	b.WriteString(": " + c.Description)
	if c.Body != "" {
		b.WriteString("\n\n" + c.Body)
	}
	if len(c.Footers) > 0 {
		b.WriteString("\n\n")
		for i, f := range c.Footers {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(f.Token + ": " + f.Value)
		}
	}
	return b.String()
}

// ParseFooterLine parses a single footer trailer line for message construction
// (the commit wizard). ok is false when line is not a valid footer. Unlike the
// internal footer-block parser used by Parse, it preserves the leading "#" of the
// "Token #value" form so the result round-trips through Format.
func ParseFooterLine(line string) (Footer, bool) {
	m := footerLinePattern.FindStringSubmatch(line)
	if m == nil {
		return Footer{}, false
	}
	value := m[3]
	if m[2] == " #" {
		value = "#" + value
	}
	return Footer{Token: m[1], Value: value}, true
}

func parseFooterBlock(lines []string) []Footer {
	var footers []Footer
	for _, line := range lines {
		if m := footerLinePattern.FindStringSubmatch(line); m != nil {
			footers = append(footers, Footer{Token: m[1], Value: m[3]})
			continue
		}
		if len(footers) > 0 {
			footers[len(footers)-1].Value += "\n" + line
		}
	}
	return footers
}
