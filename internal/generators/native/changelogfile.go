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
)

// anchorRe matches a section anchor line and captures the release tag.
var anchorRe = regexp.MustCompile(`(?m)^<!-- heraut-release: (.+) -->$`)

// anchorLine returns the render-invisible section anchor for a release tag.
func anchorLine(tag string) string { return anchorOpen + tag + anchorClose }

// anchoredSection is one release section: its tag plus the full block text (anchor line + body).
type anchoredSection struct {
	tag  string
	text string
}

// parseChangelog splits content into the preamble (everything before the first anchor) and the
// ordered anchored sections. hasAnchors is false when content contains no anchor line — in which
// case preamble is the whole content and sections is nil.
func parseChangelog(content string) (preamble string, sections []anchoredSection, hasAnchors bool) {
	locs := anchorRe.FindAllStringSubmatchIndex(content, -1)
	if len(locs) == 0 {
		return content, nil, false
	}
	preamble = content[:locs[0][0]]
	for i, m := range locs {
		end := len(content)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		sections = append(sections, anchoredSection{
			tag:  content[m[2]:m[3]],
			text: strings.TrimRight(content[m[0]:end], "\n"),
		})
	}
	return preamble, sections, true
}

// spliceSection inserts a freshly rendered section (newBody, without its anchor) for newTag into
// existing changelog content. If the top section already carries newTag it is replaced
// (idempotent); otherwise the new section is inserted above it, preserving the rest verbatim.
// ErrNoAnchors is returned when existing is non-empty but anchorless.
func spliceSection(existing, newBody, newTag string) (string, error) {
	preamble, sections, hasAnchors := parseChangelog(existing)
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
	return preamble + strings.Join(texts, "\n\n") + "\n", nil
}
