package native

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// changelogHeader is the fixed file-level header for a CHANGELOG.md.
// Release-notes have no header (the platform renders the version heading).
const changelogHeader = "# Changelog\n\n"

//go:embed changelog.tmpl
var changelogTmpl string

//go:embed release_notes.tmpl
var releaseNotesTmpl string

// ─── view model types ─────────────────────────────────────────────────────────

// commitView is a fully pre-rendered commit line for the changelog template.
// The Line field holds everything after the leading "- ": scope, breaking flag,
// description, commit link, and ticket links. Templates iterate over these and
// prefix "- " themselves.
type commitView struct {
	Line string // e.g. `*(api)* Add endpoint - ([abc1234](URL)) ([PROJ-1](url))`
}

// commitNoteView holds the pre-formatted block for one commit in the release-notes
// variant. Block contains the commit line plus optional body and footer lines,
// separated by blank lines — ready to embed directly in the template between "\n"
// delimiters. Templates never branch on its contents.
type commitNoteView struct {
	Block string // full formatted commit section, no leading/trailing "\n"
}

// contributorView is a pre-rendered "New Contributors" line for the release-notes variant,
// e.g. `* @octocat made their first contribution in [#42](url)`. Templates only iterate.
type contributorView struct {
	Line string
}

type ticketLink struct {
	Text string
	Href string
}

type statsTicketLink struct {
	Text  string
	Href  string
	Count int
}

type groupView struct {
	Name    string
	Commits []commitView
}

type groupNoteView struct {
	Name    string
	Commits []commitNoteView
}

// changelogView is the template data for renderChangelogSection.
type changelogView struct {
	Heading       string      // full heading line, e.g. `## [1.2.3](compareURL) - 2024-01-15`
	HeadingPrefix string      // "#"×types_heading_level — the depth of each group section heading
	Groups        []groupView // display-ordered; commits already scope-sorted within each group
}

// notesView is the template data for renderReleaseNotes.
type notesView struct {
	HeadingPrefix     string // "#"×types_heading_level — depth of the group + statistics headings
	Groups            []groupNoteView
	Contributors      []contributorView // first-time contributors (enrichment); empty omits the block
	CommitCount       int
	CommitsTimespan   int // days between oldest and newest commit date
	ConventionalCount int
	StatTicketLinks   []statsTicketLink
	TicketLinkCount   int  // len(StatTicketLinks), pre-computed for templates
	DaysSincePrev     int  // 0 when prevReleaseDate is zero
	HasDaysSincePrev  bool // true only when a previous-release date was supplied
}

// ─── render entry points ─────────────────────────────────────────────────────

// renderChangelogSection renders a single release section for a CHANGELOG.md.
// The returned string is trimmed of leading/trailing whitespace so T125 can join
// multiple sections with "\n\n" and prepend changelogHeader.
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
) (string, error) {
	v := buildChangelogView(version, previousVersion, releaseDate, groups, lc, tickets, typesHeadingLevel, enrichment)
	out, err := execTemplate("changelog", changelogTmpl, v)
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
	enrichment map[string]PullRequest,
) (string, error) {
	v := buildNotesView(version, previousVersion, releaseDate, groups, lc, tickets, prevReleaseDate, typesHeadingLevel, enrichment)
	out, err := execTemplate("release_notes", releaseNotesTmpl, v)
	if err != nil {
		return "", fmt.Errorf("rendering release notes: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// ─── view model builders ──────────────────────────────────────────────────────

func buildChangelogView(
	version, previousVersion string,
	releaseDate time.Time,
	groups []group,
	lc *port.LinkContext,
	tickets []config.Ticket,
	typesHeadingLevel int,
	enrichment map[string]PullRequest,
) changelogView {
	cuBase := buildCommitURL(lc)
	compareURL := buildCompareURL(lc, previousVersion, version)

	ver := strings.TrimPrefix(version, "v")
	var heading string
	if compareURL != "" {
		heading = fmt.Sprintf("## [%s](%s) - %s", ver, compareURL, releaseDate.Format("2006-01-02"))
	} else {
		heading = fmt.Sprintf("## [%s] - %s", ver, releaseDate.Format("2006-01-02"))
	}

	gviews := make([]groupView, 0, len(groups))
	for _, g := range groups {
		gv := groupView{Name: g.name}
		for _, pc := range g.commits {
			gv.Commits = append(gv.Commits, commitView{Line: buildCommitLine(pc, cuBase, tickets, enrichment)})
		}
		gviews = append(gviews, gv)
	}

	return changelogView{Heading: heading, HeadingPrefix: headingPrefix(typesHeadingLevel), Groups: gviews}
}

func buildNotesView(
	version, previousVersion string,
	releaseDate time.Time,
	groups []group,
	lc *port.LinkContext,
	tickets []config.Ticket,
	prevReleaseDate time.Time,
	typesHeadingLevel int,
	enrichment map[string]PullRequest,
) notesView {
	_ = version
	_ = previousVersion
	cuBase := buildCommitURL(lc)

	var allCommits []parsedCommit
	for _, g := range groups {
		allCommits = append(allCommits, g.commits...)
	}

	// Stats: commit count, timespan, conventional count.
	commitCount := len(allCommits)
	conventionalCount := 0
	var oldest, newest time.Time
	for _, pc := range allCommits {
		if pc.parsed != nil {
			conventionalCount++
		}
		d := pc.raw.Date
		if oldest.IsZero() || d.Before(oldest) {
			oldest = d
		}
		if newest.IsZero() || d.After(newest) {
			newest = d
		}
	}
	timespan := 0
	if !oldest.IsZero() && !newest.IsZero() {
		timespan = int(newest.Sub(oldest).Hours() / 24)
	}

	// Distinct ticket links across all commits (counted per unique href).
	statTickets := buildStatTicketLinks(allCommits, tickets)

	daysSincePrev := 0
	hasDaysSincePrev := !prevReleaseDate.IsZero()
	if hasDaysSincePrev {
		diff := releaseDate.Sub(prevReleaseDate)
		daysSincePrev = int(diff.Hours() / 24)
	}

	// Build group note views.
	gnviews := make([]groupNoteView, 0, len(groups))
	for _, g := range groups {
		gnv := groupNoteView{Name: g.name}
		for _, pc := range g.commits {
			block := buildCommitBlock(pc, cuBase, tickets, enrichment)
			gnv.Commits = append(gnv.Commits, commitNoteView{Block: block})
		}
		gnviews = append(gnviews, gnv)
	}

	return notesView{
		HeadingPrefix:     headingPrefix(typesHeadingLevel),
		Groups:            gnviews,
		Contributors:      buildContributors(allCommits, enrichment),
		CommitCount:       commitCount,
		CommitsTimespan:   timespan,
		ConventionalCount: conventionalCount,
		StatTicketLinks:   statTickets,
		TicketLinkCount:   len(statTickets),
		DaysSincePrev:     daysSincePrev,
		HasDaysSincePrev:  hasDaysSincePrev,
	}
}

// ─── commit line helpers ──────────────────────────────────────────────────────

// buildCommitLine assembles the full commit line content (everything after "- "):
//
//	{*(scope)* }{[**breaking**] }Description - {([hash7](URL))|hash7}{ ([ticket](href))...}
//
// When cuBase is "" (lc==nil), a bare 7-char hash is used with no Markdown link.
func buildCommitLine(pc parsedCommit, cuBase string, tickets []config.Ticket, enrichment map[string]PullRequest) string {
	scope, breaking, desc := commitLineDetails(pc)
	hash7 := pc.raw.Hash
	if len(hash7) > 7 {
		hash7 = hash7[:7]
	}

	var sb strings.Builder
	if scope != "" {
		sb.WriteString("*(")
		sb.WriteString(scope)
		sb.WriteString(")* ")
	}
	if breaking {
		sb.WriteString("[**breaking**] ")
	}
	sb.WriteString(desc)
	sb.WriteString(" - ")
	if cuBase != "" {
		sb.WriteString("([")
		sb.WriteString(hash7)
		sb.WriteString("](")
		sb.WriteString(cuBase)
		sb.WriteString(pc.raw.Hash)
		sb.WriteString("))")
	} else {
		sb.WriteString(hash7)
	}

	// Remote enrichment: PR author + number (ADR-0034). Placed after the hash link and before
	// ticket links, mirroring the git-cliff release-notes template.
	if pr, ok := enrichment[pc.raw.Hash]; ok {
		if pr.AuthorLogin != "" {
			sb.WriteString(" by @")
			sb.WriteString(pr.AuthorLogin)
		}
		if pr.Number > 0 {
			fmt.Fprintf(&sb, " in [%s](%s)", prRef(pr), pr.URL)
		}
	}

	// Ticket links: search the full commit text (subject + body).
	text := pc.raw.Subject
	if pc.raw.Body != "" {
		text += "\n" + pc.raw.Body
	}
	for _, tl := range resolveTickets(text, tickets) {
		sb.WriteString(" ([")
		sb.WriteString(tl.Text)
		sb.WriteString("](")
		sb.WriteString(tl.Href)
		sb.WriteString("))")
	}
	return sb.String()
}

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

// resolveTickets finds all ticket matches in text for each configured ticket pattern
// (in config order) and returns the corresponding ticketLinks. Multiple matches of
// the same pattern generate multiple links. Patterns with no capture group use the
// full match as the URL substitution value (mirroring gitcliff.injectLinkParsers).
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

// buildContributors returns the distinct first-time contributors across commits (in first-seen
// order) as pre-rendered "New Contributors" lines. Empty when there is no enrichment or no
// first-timer, which omits the block entirely.
func buildContributors(commits []parsedCommit, enrichment map[string]PullRequest) []contributorView {
	if len(enrichment) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []contributorView
	for _, pc := range commits {
		pr, ok := enrichment[pc.raw.Hash]
		if !ok || !pr.FirstTimer || pr.AuthorLogin == "" || seen[pr.AuthorLogin] {
			continue
		}
		seen[pr.AuthorLogin] = true
		line := "* @" + pr.AuthorLogin + " made their first contribution"
		if pr.Number > 0 {
			line += fmt.Sprintf(" in [%s](%s)", prRef(pr), pr.URL)
		}
		out = append(out, contributorView{Line: line})
	}
	return out
}

// ─── body / footer helpers ────────────────────────────────────────────────────

// buildCommitBlock assembles the full commit section string for the release-notes
// variant. The block starts with the commit line and, when body or footers are
// present, appends them separated by blank lines:
//
//   - line\n\n    body\n\n  Token: Value
//
// Body lines are indented 4 spaces; footer lines are indented 2 spaces with
// "Token: Value" format. The block has no leading or trailing "\n" — those are
// added by the release_notes.tmpl per-commit iteration.
func buildCommitBlock(pc parsedCommit, cuBase string, tickets []config.Ticket, enrichment map[string]PullRequest) string {
	line := "- " + buildCommitLine(pc, cuBase, tickets, enrichment)

	bodyText := ""
	var footerLines []string

	if pc.parsed != nil && (pc.parsed.Body != "" || len(pc.parsed.Footers) > 0) {
		bodyText = pc.parsed.Body
		for _, f := range pc.parsed.Footers {
			footerLines = append(footerLines, f.Token+": "+f.Value)
		}
	} else if pc.raw.Body != "" && pc.parsed == nil {
		// Non-conventional commit: treat the whole body as plain text.
		bodyText = pc.raw.Body
	}

	if bodyText == "" && len(footerLines) == 0 {
		return line
	}

	var sb strings.Builder
	sb.WriteString(line)

	if bodyText != "" {
		sb.WriteString("\n\n")
		for i, l := range strings.Split(bodyText, "\n") {
			if i > 0 {
				sb.WriteString("\n")
			}
			if l != "" {
				sb.WriteString("    ") // 4-space indent
			}
			sb.WriteString(l)
		}
	}

	if len(footerLines) > 0 {
		sb.WriteString("\n\n")
		for i, fl := range footerLines {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("  ") // 2-space indent
			sb.WriteString(fl)
		}
	}

	return sb.String()
}

// ─── heading post-processing ──────────────────────────────────────────────────

// applyHeadingPattern applies headingVersionPattern (a regex) to the first line
// of content, replacing the bracketed version match with [$1]. This mirrors
// injectHeadingPostprocessor in the gitcliff package — native receives the derived
// pattern as a string so it does not need to import tagfmt.
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

func execTemplate(name, tmplStr string, data any) (string, error) {
	t, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing template %q: %w", name, err)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("executing template %q: %w", name, err)
	}
	return sb.String(), nil
}
