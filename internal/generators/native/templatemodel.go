package native

import "time"

// The tpl* types are the public template contract: the data a user template (inline
// rendering.templates.<block> snippet or a <driver>.template file) sees as its root and
// nested values. Field names are the stable/experimental public API per ADR-0037 — additive
// changes are free, renames/removals are avoided and would be called out.

// tplChangelog is the root passed to a changelog template.
type tplChangelog struct {
	Releases []tplRelease
	Heraut   tplHeraut
}

// tplRelease is the root passed to a release-notes template, and one entry in a changelog.
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
