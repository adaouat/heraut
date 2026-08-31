package native

import (
	"errors"
	"regexp"
	"strings"
)

// ErrNoAnchors reports that a non-empty changelog has no heraut-release anchors — it was produced
// by another tool and cannot be safely spliced incrementally (the caller directs to --regenerate).
var ErrNoAnchors = errors.New("no heraut-release anchors")

const (
	anchorOpen  = "<!-- heraut-release: "
	anchorClose = " -->"
	// footerAnchor marks the start of the document-level footer region (ADR-0050) — the trailing
	// counterpart to the preamble, structural and non-overridable like the section anchors, so
	// parseChangelog can find and discard a stale on-disk footer before a fresh one is written.
	footerAnchor = "<!-- heraut-footer -->"
)

// anchorRe matches a section anchor line and captures the release tag.
var anchorRe = regexp.MustCompile(`(?m)^<!-- heraut-release: (.+) -->$`)

// footerAnchorRe matches the footer anchor line and everything after it, so parseChangelog can
// strip the whole trailing footer region before locating section boundaries.
var footerAnchorRe = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(footerAnchor) + `\n?`)

// anchorLine returns the render-invisible section anchor for a release tag.
func anchorLine(tag string) string { return anchorOpen + tag + anchorClose }

// anchoredSection is one release section: its tag plus the full block text (anchor line + body).
type anchoredSection struct {
	tag  string
	text string
}

// parseChangelog splits content into the preamble (everything before the first anchor) and the
// ordered anchored sections, having first discarded any trailing document-footer region (marked by
// footerAnchor, ADR-0050) so it never ends up folded into the last section's captured text.
// hasAnchors is false when content contains no anchor line — in which case preamble is the whole
// content (footer region included) and sections is nil.
func parseChangelog(content string) (preamble string, sections []anchoredSection, hasAnchors bool) {
	body := content
	if loc := footerAnchorRe.FindStringIndex(content); loc != nil {
		body = content[:loc[0]]
	}
	locs := anchorRe.FindAllStringSubmatchIndex(body, -1)
	if len(locs) == 0 {
		return content, nil, false
	}
	preamble = body[:locs[0][0]]
	for i, m := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		sections = append(sections, anchoredSection{
			tag:  body[m[2]:m[3]],
			text: strings.TrimRight(body[m[0]:end], "\n"),
		})
	}
	return preamble, sections, true
}

// spliceSection inserts a freshly rendered section (newBody, without its anchor) for newTag into
// the ordered sections parsed from existing changelog content, preserving the rest verbatim. If
// the top section already carries newTag it is replaced (idempotent); otherwise the new section is
// inserted above it. preamble and postamble are always the freshly rendered current-config output
// (ADR-0050) — never whatever was previously on disk, which parseChangelog discards along with the
// stale footer region. ErrNoAnchors is returned when existing is non-empty but anchorless.
func spliceSection(existing, newBody, newTag, preamble, postamble string) (string, error) {
	_, sections, hasAnchors := parseChangelog(existing)
	if !hasAnchors {
		return "", ErrNoAnchors
	}
	block := anchoredSection{tag: newTag, text: anchorLine(newTag) + "\n" + newBody}
	if len(sections) > 0 && sections[0].tag == newTag {
		sections[0] = block
	} else {
		sections = append([]anchoredSection{block}, sections...)
	}
	texts := make([]string, len(sections))
	for i, s := range sections {
		texts[i] = s.text
	}
	body := preamble + strings.Join(texts, "\n\n") + "\n"
	if postamble != "" {
		body += footerAnchor + postamble
	}
	return body, nil
}
