package native

import (
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// The tpl* types are the public template contract: the data a user template (inline
// rendering.templates.<block> snippet or a <driver>.template file) sees as its root and
// nested values. Field names are the stable/experimental public API per ADR-0037 — additive
// changes are free, renames/removals are avoided and would be called out.

// tplRelease is the root passed to both the release-notes and the changelog-section templates.
type tplRelease struct {
	Version       string
	Tag           string
	PreviousTag   string
	CompareURL    string
	HeadingPrefix string // leading "#"s for the contributors/stats headings
	Date          time.Time
	Groups        []tplGroup
	Contributors  []tplContributor
	Stats         tplStats
	Heraut        tplHeraut
}

// tplHeraut is document meta reachable from header/footer/root blocks.
type tplHeraut struct {
	Version     string
	URL         string
	GeneratedAt time.Time
}

// tplGroup is one commit-type group with its heading and commits.
type tplGroup struct {
	Name          string
	HeadingPrefix string // "#" × types_heading_level
	Commits       []tplCommit
}

// tplCommit is one rendered commit.
type tplCommit struct {
	Type        string
	Scope       string
	Breaking    bool
	Description string
	Subject     string
	Body        string
	Hash        string
	ShortHash   string
	CommitURL   string
	Date        time.Time
	Author      Author
	PR          *tplPR // nil when the commit has no associated PR/MR
	Tickets     []tplLink
	Footers     []tplFooter
}

// tplPR is a commit's associated pull/merge request. All fields are remote-only (empty offline).
type tplPR struct {
	Number    int
	URL       string
	Title     string
	Ref       string // "#42" / "!42", precomputed per platform
	Labels    []string
	Author    Author
	CreatedAt time.Time
	MergedAt  time.Time
	MergedBy  Author
	Approvers []Author
}

// tplContributor is one entry in the "New Contributors" block.
type tplContributor struct {
	Author Author
	PR     *tplPR // their first PR in this release; nil offline
}

// tplStats is the release statistics block.
type tplStats struct {
	CommitCount       int
	ConventionalCount int
	TimespanDays      int
	DaysSincePrev     int
	HasDaysSincePrev  bool
	Tickets           []tplStatTicket
}

// tplLink is a text/href pair (e.g. a ticket reference in a commit line).
type tplLink struct {
	Text string
	Href string
}

// tplStatTicket is one aggregated ticket reference in the stats block.
type tplStatTicket struct {
	Text  string
	Href  string
	Count int
}

// tplFooter is one git trailer parsed from a commit body.
type tplFooter struct {
	Token string
	Value string
}

// ─── template-model builders ──────────────────────────────────────────────────

// buildRelease maps the internal render data (grouped commits + enrichment + tickets + stats +
// previous-release date + heraut meta) into the public tplRelease contract. It reuses the
// render.go helpers (buildCommitURL/buildCompareURL/commitLineDetails/prRef/resolveTickets/
// buildStatTicketLinks/headingPrefix) so the mapping and the built-in output share one code path.
func buildRelease(
	version, previousVersion string,
	releaseDate, prevReleaseDate time.Time,
	groups []group,
	lc *port.LinkContext,
	tickets []config.Ticket,
	typesHeadingLevel int,
	enrichment map[string]PullRequest,
	contributors []Contributor,
	heraut tplHeraut,
) tplRelease {
	cuBase := buildCommitURL(lc)
	prefix := headingPrefix(typesHeadingLevel)

	var allCommits []parsedCommit
	tplGroups := make([]tplGroup, 0, len(groups))
	for _, g := range groups {
		allCommits = append(allCommits, g.commits...)
		tg := tplGroup{Name: g.name, HeadingPrefix: prefix}
		for _, pc := range g.commits {
			tg.Commits = append(tg.Commits, buildCommit(pc, cuBase, tickets, enrichment))
		}
		tplGroups = append(tplGroups, tg)
	}

	return tplRelease{
		Version:       strings.TrimPrefix(version, "v"),
		Tag:           version,
		PreviousTag:   previousVersion,
		CompareURL:    buildCompareURL(lc, previousVersion, version),
		HeadingPrefix: prefix,
		Date:          releaseDate,
		Groups:        tplGroups,
		Contributors:  buildContributors(contributors),
		Stats:         buildStats(allCommits, tickets, releaseDate, prevReleaseDate),
		Heraut:        heraut,
	}
}

// buildCommit maps one parsedCommit into a tplCommit, resolving its conventional-commit fields,
// commit URL, associated PR, ticket links, and footers.
func buildCommit(pc parsedCommit, cuBase string, tickets []config.Ticket, enrichment map[string]PullRequest) tplCommit {
	scope, breaking, desc := commitLineDetails(pc)
	shortHash := pc.raw.Hash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	commitURL := ""
	if cuBase != "" {
		commitURL = cuBase + pc.raw.Hash
	}

	var commitType, body string
	var footers []tplFooter
	if pc.parsed != nil {
		commitType = pc.parsed.Type
		body = pc.parsed.Body
		for _, f := range pc.parsed.Footers {
			footers = append(footers, tplFooter{Token: f.Token, Value: f.Value})
		}
	} else {
		body = pc.raw.Body
	}

	text := pc.raw.Subject
	if pc.raw.Body != "" {
		text += "\n" + pc.raw.Body
	}
	var links []tplLink
	for _, tl := range resolveTickets(text, tickets) {
		links = append(links, tplLink(tl))
	}

	var pr *tplPR
	if p, ok := enrichment[pc.raw.Hash]; ok {
		pr = tplPRFrom(p)
	}

	return tplCommit{
		Type:        commitType,
		Scope:       scope,
		Breaking:    breaking,
		Description: desc,
		Subject:     pc.raw.Subject,
		Body:        body,
		Hash:        pc.raw.Hash,
		ShortHash:   shortHash,
		CommitURL:   commitURL,
		Date:        pc.raw.Date,
		Author:      Author{Name: pc.raw.Author, Email: pc.raw.Email, Username: pc.raw.AuthorHandle},
		PR:          pr,
		Tickets:     links,
		Footers:     footers,
	}
}

// tplPRFrom maps a normalized PullRequest into the public tplPR (Ref precomputed via prRef).
func tplPRFrom(pr PullRequest) *tplPR {
	return &tplPR{
		Number:    pr.Number,
		URL:       pr.URL,
		Title:     pr.Title,
		Ref:       prRef(pr),
		Labels:    pr.Labels,
		Author:    Author{Username: pr.AuthorLogin},
		CreatedAt: pr.CreatedAt,
		MergedAt:  pr.MergedAt,
		MergedBy:  pr.MergedBy,
		Approvers: pr.Approvers,
	}
}

// buildContributors maps local-tier Contributors into the public tplContributor slice.
// Contributors without a platform handle are dropped: the built-in New Contributors block is
// remote-handle-gated (ADR-0036), so offline the slice is empty and the block is omitted.
func buildContributors(contributors []Contributor) []tplContributor {
	var out []tplContributor
	for _, c := range contributors {
		if c.Author.Username == "" {
			continue
		}
		var pr *tplPR
		if c.PR != nil {
			pr = tplPRFrom(*c.PR)
		}
		out = append(out, tplContributor{Author: c.Author, PR: pr})
	}
	return out
}

// buildStats computes the release statistics block from all commits, reusing the render.go
// stat logic (commit/conventional counts, commit timespan, days-since-previous, ticket links).
func buildStats(allCommits []parsedCommit, tickets []config.Ticket, releaseDate, prevReleaseDate time.Time) tplStats {
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

	daysSincePrev := 0
	hasDaysSincePrev := !prevReleaseDate.IsZero()
	if hasDaysSincePrev {
		daysSincePrev = int(releaseDate.Sub(prevReleaseDate).Hours() / 24)
	}

	var statTickets []tplStatTicket
	for _, s := range buildStatTicketLinks(allCommits, tickets) {
		statTickets = append(statTickets, tplStatTicket(s))
	}

	return tplStats{
		CommitCount:       len(allCommits),
		ConventionalCount: conventionalCount,
		TimespanDays:      timespan,
		DaysSincePrev:     daysSincePrev,
		HasDaysSincePrev:  hasDaysSincePrev,
		Tickets:           statTickets,
	}
}
