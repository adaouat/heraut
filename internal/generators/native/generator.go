package native

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// Mode selects which content the native generator produces.
type Mode int

const (
	ModeChangelog Mode = iota
	ModeReleaseNotes
)

// Generator is heraut's built-in, zero-external-dependency content generator (ADR-0032). It
// walks git history, classifies commits against the effective commits.types / rendering.excludes
// (propagated onto the ContentDriver by the app layer), and renders Markdown with internal
// templates. Phase 1 performs no remote enrichment, so Degraded always reports false.
type Generator struct {
	runner port.Runner
	cfg    *config.ContentDriver
	mode   Mode
}

var _ port.Generator = (*Generator)(nil)

// New constructs a native Generator.
func New(runner port.Runner, cfg *config.ContentDriver, mode Mode) *Generator {
	return &Generator{runner: runner, cfg: cfg, mode: mode}
}

// Check verifies the generator is usable. native has no external dependency, so it always
// succeeds.
func (g *Generator) Check() error { return nil }

// Validate checks generator-specific config. native has no required config in Phase 1 (user
// templates are deferred — ADR-0033), so it always succeeds.
func (g *Generator) Validate() error { return nil }

// Degraded reports whether remote metadata was unavailable. Phase 1 performs no enrichment,
// so native is never degraded.
func (g *Generator) Degraded() bool { return false }

// Generate produces the changelog (writing it to cfg.Output when set) or the release-notes
// string for tag, resolving commit/compare links against lc.
func (g *Generator) Generate(tag string, lc *port.LinkContext) (string, error) {
	if g.mode == ModeReleaseNotes {
		return g.generateReleaseNotes(tag, lc)
	}
	return g.generateChangelog(tag, lc)
}

func (g *Generator) generateReleaseNotes(tag string, lc *port.LinkContext) (string, error) {
	prev, err := previousTag(g.runner, tag, "")
	if err != nil {
		return "", err
	}
	commits, err := collectCommits(g.runner, commitRange(prev, tag))
	if err != nil {
		return "", err
	}
	groups := groupCommits(commits, g.cfg.Types, g.cfg.Excludes)
	// prevReleaseDate is omitted in Phase 1 (the "days between releases" stat is a follow-up):
	// it needs the previous tag's date.
	return renderReleaseNotes(tag, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, time.Time{}, g.cfg.TypesHeadingLevel)
}

// generateChangelog regenerates the full CHANGELOG.md: a section for the release being created
// (its commits are everything since the latest existing tag — the changelog runs before the
// tag is created) followed by one section per existing tag, newest-first. Written to cfg.Output
// when set, and also returned.
func (g *Generator) generateChangelog(tag string, lc *port.LinkContext) (string, error) {
	tags, err := listTags(g.runner, "")
	if err != nil {
		return "", err
	}

	var sections []string

	latest := ""
	if len(tags) > 0 {
		latest = tags[0]
	}
	if sec, err := g.renderRelease(tag, latest, commitRange(latest, "HEAD"), lc); err != nil {
		return "", err
	} else if sec != "" {
		sections = append(sections, sec)
	}

	// Existing releases, newest-first. prev is the next-older tag by version refname (listTags
	// is version-sorted); release-notes mode instead resolves prev via git-describe topology.
	// Equivalent for linear history — the common case.
	for i, t := range tags {
		prev := ""
		if i+1 < len(tags) {
			prev = tags[i+1]
		}
		if sec, err := g.renderRelease(t, prev, commitRange(prev, t), lc); err != nil {
			return "", err
		} else if sec != "" {
			sections = append(sections, sec)
		}
	}

	body := changelogHeader + strings.Join(sections, "\n\n") + "\n"
	if g.cfg.Output != "" {
		if err := os.WriteFile(g.cfg.Output, []byte(body), 0o644); err != nil {
			return "", fmt.Errorf("writing changelog %q: %w", g.cfg.Output, err)
		}
	}
	return body, nil
}

// renderRelease renders one changelog release section, or "" when the range has no
// classifiable commits.
func (g *Generator) renderRelease(version, prev, rng string, lc *port.LinkContext) (string, error) {
	commits, err := collectCommits(g.runner, rng)
	if err != nil {
		return "", err
	}
	groups := groupCommits(commits, g.cfg.Types, g.cfg.Excludes)
	if len(groups) == 0 {
		return "", nil
	}
	return renderChangelogSection(version, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, g.cfg.HeadingVersionPattern, g.cfg.TypesHeadingLevel)
}
