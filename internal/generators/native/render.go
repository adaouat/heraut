package native

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

//go:embed blocks.tmpl
var blocksTmpl string

//go:embed changelog.tmpl
var changelogTmpl string

//go:embed release_notes.tmpl
var releaseNotesTmpl string

type ticketLink struct {
	Text string
	Href string
}

type statsTicketLink struct {
	Text  string
	Href  string
	Count int
}

// ─── render entry points ─────────────────────────────────────────────────────

// renderChangelogSection renders a single release section for a CHANGELOG.md.
// The returned string is trimmed of leading/trailing whitespace so buildAllSections can join
// multiple sections and prepend the rendered title/subtitle preamble (renderPreamble).
//
// version and previousVersion are raw tag strings (e.g. "v1.2.3"). When lc is nil,
// commit lines show bare 7-char hashes (no Markdown link) and the heading omits the
// compare URL — this is the intentional --offline equivalent behaviour. When
// headingVersionPattern is non-empty, the heading's [version] bracket is regex-replaced
// (replacement `[$1]`) to strip env prefix / build ID.
func renderChangelogSection(
	version, previousVersion string,
	releaseDate time.Time,
	groups []group,
	lc *port.LinkContext,
	tickets []config.Ticket,
	headingVersionPattern string,
	typesHeadingLevel int,
	enrichment map[string]PullRequest,
	heraut tplHeraut,
	snippets map[string]string,
	templateFile string,
) (string, error) {
	rel := buildRelease(version, previousVersion, releaseDate, time.Time{}, groups, lc, tickets, typesHeadingLevel, enrichment, nil, heraut)
	out, err := execBlocks("changelog", changelogTmpl, snippets, templateFile, rel)
	if err != nil {
		return "", fmt.Errorf("rendering changelog section: %w", err)
	}
	out = strings.TrimSpace(out)
	if headingVersionPattern != "" {
		out, err = applyHeadingPattern(out, headingVersionPattern)
		if err != nil {
			return "", fmt.Errorf("applying heading pattern: %w", err)
		}
	}
	return out, nil
}

// renderReleaseNotes renders the release-notes body for one release.
// The body starts with the group sections followed by the commit-statistics block.
// prevReleaseDate being zero causes the "days passed between releases" stat to be omitted.
func renderReleaseNotes(
	version, previousVersion string,
	releaseDate time.Time,
	groups []group,
	lc *port.LinkContext,
	tickets []config.Ticket,
	prevReleaseDate time.Time,
	typesHeadingLevel int,
	prs map[string]PullRequest,
	contributors []Contributor,
	heraut tplHeraut,
	snippets map[string]string,
	templateFile string,
) (string, error) {
	rel := buildRelease(version, previousVersion, releaseDate, prevReleaseDate, groups, lc, tickets, typesHeadingLevel, prs, contributors, heraut)
	preamble, err := renderPreamble(releaseNotesTmpl, snippets, templateFile, heraut)
	if err != nil {
		return "", fmt.Errorf("rendering release notes preamble: %w", err)
	}
	out, err := execBlocks("release_notes", releaseNotesTmpl, snippets, templateFile, rel)
	if err != nil {
		return "", fmt.Errorf("rendering release notes: %w", err)
	}
	footer, err := renderPostamble(releaseNotesTmpl, snippets, templateFile, heraut)
	if err != nil {
		return "", fmt.Errorf("rendering release notes postamble: %w", err)
	}
	body := preamble + out
	if footer != "" {
		body += footerSeparator + footer
	}
	return strings.TrimSpace(body), nil
}

// ─── commit-line helpers ──────────────────────────────────────────────────────

// commitLineDetails returns the scope, breaking flag, and upper-first description
// for rendering. When pc.parsed is non-nil, uses the conventional-commit fields;
// otherwise falls back to the raw subject upper-first.
func commitLineDetails(pc parsedCommit) (scope string, breaking bool, desc string) {
	if pc.parsed != nil {
		return pc.parsed.Scope, pc.parsed.Breaking, upperFirst(pc.parsed.Description)
	}
	return "", false, upperFirst(pc.raw.Subject)
}

// prRef renders a PR/MR reference label: "#42" for GitHub, "!42" for GitLab (per RefPrefix),
// defaulting to "#".
func prRef(pr PullRequest) string {
	prefix := pr.RefPrefix
	if prefix == "" {
		prefix = "#"
	}
	return fmt.Sprintf("%s%d", prefix, pr.Number)
}

// upperFirst returns s with its first Unicode rune upper-cased.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// ─── ticket helpers ───────────────────────────────────────────────────────────

// TicketMatch is one ticket reference found in commit text: the matched display text and
// its resolved URL. Exported so callers outside native (internal/app is allowed to import
// internal/generators/* per the Layer rules table in .claude/rules/coding.md) can verify
// commits.tickets patterns against real commit text — heraut commit tickets (T241) — using
// the exact same matching logic that drives changelog/release-notes ticket-link rendering.
type TicketMatch = ticketLink

// MatchTickets finds every match of each configured ticket pattern in text, in config
// order. It is resolveTickets exported under a stable name for callers outside native.
func MatchTickets(text string, tickets []config.Ticket) []TicketMatch {
	return resolveTickets(text, tickets)
}

// resolveTickets finds all ticket matches in text for each configured ticket pattern
// (in config order) and returns the corresponding ticketLinks. Multiple matches of
// the same pattern generate multiple links. Patterns with no capture group use the
// full match as the URL substitution value.
func resolveTickets(text string, tickets []config.Ticket) []ticketLink {
	if len(tickets) == 0 {
		return nil
	}
	var links []ticketLink
	for _, t := range tickets {
		re, err := regexp.Compile(t.Pattern)
		if err != nil {
			continue
		}
		matches := re.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			label := m[0] // full match as display text
			var sub string
			if re.NumSubexp() > 0 {
				sub = m[1] // capture group 1 as URL substitution
			} else {
				sub = m[0]
			}
			href := strings.ReplaceAll(t.URL, "{ticket}", sub)
			links = append(links, ticketLink{Text: label, Href: href})
		}
	}
	return links
}

// buildStatTicketLinks collects distinct ticket links across all commits, counting
// how many times each href is referenced. Used for the release-notes statistics block.
func buildStatTicketLinks(commits []parsedCommit, tickets []config.Ticket) []statsTicketLink {
	type entry struct {
		text  string
		count int
	}
	byHref := make(map[string]*entry)
	var order []string

	for _, pc := range commits {
		text := pc.raw.Subject
		if pc.raw.Body != "" {
			text += "\n" + pc.raw.Body
		}
		seen := make(map[string]bool)
		for _, tl := range resolveTickets(text, tickets) {
			if seen[tl.Href] {
				continue
			}
			seen[tl.Href] = true
			if byHref[tl.Href] == nil {
				byHref[tl.Href] = &entry{text: tl.Text}
				order = append(order, tl.Href)
			}
			byHref[tl.Href].count++
		}
	}

	result := make([]statsTicketLink, 0, len(order))
	for _, href := range order {
		e := byHref[href]
		result = append(result, statsTicketLink{Text: e.text, Href: href, Count: e.count})
	}
	return result
}

// ─── heading post-processing ──────────────────────────────────────────────────

// applyHeadingPattern applies headingVersionPattern (a regex) to the first line
// of content, replacing the bracketed version match with [$1]. native receives the
// derived pattern as a string so it does not need to import tagfmt.
func applyHeadingPattern(content, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid heading version pattern %q: %w", pattern, err)
	}
	lines := strings.SplitN(content, "\n", 2)
	lines[0] = re.ReplaceAllString(lines[0], "[$1]")
	return strings.Join(lines, "\n"), nil
}

// headingPrefix returns the Markdown heading prefix ("#" repeated) for type section headings,
// from commits.types_heading_level. A non-positive level defaults to 3 ("###").
func headingPrefix(level int) string {
	if level <= 0 {
		level = 3
	}
	return strings.Repeat("#", level)
}

// ─── template execution ───────────────────────────────────────────────────────

// execBlocks parses the shared built-in blocks plus the doc-specific root, then layers the user's
// inline snippets and optional template file on top (buildTemplateSet), and executes the named
// root block over data. The built-in changelog / release-notes output is produced entirely through
// this block set (ADR-0037) — the same path a user template extends.
func execBlocks(rootName, rootTmpl string, snippets map[string]string, templateFile string, data any) (string, error) {
	ts, err := buildTemplateSet(blocksTmpl+"\n"+rootTmpl, snippets, templateFile)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := ts.ExecuteTemplate(&sb, rootName, data); err != nil {
		return "", fmt.Errorf("executing %q template: %w", rootName, err)
	}
	return sb.String(), nil
}

// renderPreamble renders a driver's document-level title+subtitle blocks (ADR-0048): unlike every
// other block, these fire once per document, not once per rendered release, so they execute
// against a bare tplHeraut (not a Release) — snippets write .Version/.URL/.GeneratedAt directly,
// not .Heraut.Version. Each is independently trimmed and blank-line-joined: an unset block
// contributes nothing, so no stray blank line survives whether both, one, or neither is set. An
// empty-or-whitespace snippet override nulls a block via buildTemplateSet's null-body encoding —
// the same mechanism every other block's null override uses (T251), so no special-casing here.
func renderPreamble(rootTmpl string, snippets map[string]string, templateFile string, heraut tplHeraut) (string, error) {
	title, err := execBlocks("title", rootTmpl, snippets, templateFile, heraut)
	if err != nil {
		return "", err
	}
	subtitle, err := execBlocks("subtitle", rootTmpl, snippets, templateFile, heraut)
	if err != nil {
		return "", err
	}
	var parts []string
	if t := strings.TrimSpace(title); t != "" {
		parts = append(parts, t)
	}
	if s := strings.TrimSpace(subtitle); s != "" {
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, "\n\n") + "\n\n", nil
}

// renderPostamble renders a driver's document-level footer block (ADR-0049), the trailing
// counterpart to renderPreamble's title/subtitle: it fires once per document, not once per
// rendered release, so it executes against a bare tplHeraut — same context, same null-override
// mechanism, same reasoning as renderPreamble. Returns the trimmed footer text (empty when unset)
// — callers own the surrounding presentation: buildAllSections/spliceSection add the anchor and
// footerSeparator (ADR-0051) before appending it once after every joined section; renderReleaseNotes
// adds just footerSeparator before appending it to the single rendered release. The per-release
// trailing block (release_footer, unchanged from the original "footer" block pre-ADR-0049) still
// fires from within changelog.tmpl/release_notes.tmpl's root — this is a separate, document-scoped
// block layered on top.
func renderPostamble(rootTmpl string, snippets map[string]string, templateFile string, heraut tplHeraut) (string, error) {
	footer, err := execBlocks("footer", rootTmpl, snippets, templateFile, heraut)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(footer), nil
}
