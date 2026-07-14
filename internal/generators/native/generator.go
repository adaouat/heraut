package native

import (
	"errors"
	"fmt"
	"net/http"
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

// herautProjectURL is heraut's public repository, exposed to templates as .Heraut.URL.
const herautProjectURL = "https://github.com/adaouat/heraut"

// Generator is heraut's built-in, zero-external-dependency content generator (ADR-0032). It
// walks git history, classifies commits against the effective commits.types / rendering.excludes
// (propagated onto the ContentDriver by the app layer), and renders Markdown with internal
// templates. With a platform LinkContext it enriches commits with PR metadata via gh/glab
// (ADR-0034), honouring the remote_metadata policy; Degraded reports an optional fetch failure.
type Generator struct {
	runner     port.Runner
	httpClient *http.Client
	cfg        *config.ContentDriver
	mode       Mode
	degraded   bool
	now        func() time.Time // injected clock for .Heraut.GeneratedAt; defaults to time.Now
}

var _ port.Generator = (*Generator)(nil)

// New constructs a native Generator. The httpClient is used only by the Azure DevOps
// enrichment path (ADR-0035); GitHub/GitLab enrichment rides the runner via gh/glab.
func New(runner port.Runner, cfg *config.ContentDriver, mode Mode) *Generator {
	return &Generator{
		runner:     runner,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cfg:        cfg,
		mode:       mode,
		now:        time.Now,
	}
}

// herautMeta builds the document-meta value passed to templates as .Heraut.
func (g *Generator) herautMeta() tplHeraut {
	return tplHeraut{Version: g.cfg.HerautVersion, URL: herautProjectURL, GeneratedAt: g.now()}
}

// Check verifies the generator is usable. native has no external dependency, so it always
// succeeds.
func (g *Generator) Check() error { return nil }

// Validate checks generator-specific config. native has no required config in Phase 1 (user
// templates are deferred — ADR-0033), so it always succeeds.
func (g *Generator) Validate() error { return nil }

// Degraded reports whether an optional remote-enrichment fetch failed during the last run.
func (g *Generator) Degraded() bool { return g.degraded }

// Generate produces the changelog (writing it to cfg.Output when set) or the release-notes
// string for tag, resolving commit/compare links against lc.
func (g *Generator) Generate(tag string, lc *port.LinkContext) (string, error) {
	if g.mode == ModeReleaseNotes {
		return g.generateReleaseNotes(tag, lc)
	}
	return g.generateChangelog(tag, lc)
}

// scopedTags returns the release tags for the active scope, newest-first: the env glob (per-env
// auto, T138) takes precedence, else an explicit tag_pattern regex filter (T139), else all tags.
func (g *Generator) scopedTags() ([]string, error) {
	if g.cfg.TagGlob != "" {
		return listTags(g.runner, g.cfg.TagGlob)
	}
	all, err := listTags(g.runner, "")
	if err != nil {
		return nil, err
	}
	return filterByTagPattern(all, g.cfg.TagPattern)
}

// scopedPreviousTag resolves the tag preceding tag within the active scope. An explicit
// tag_pattern (regex) resolves from the Go-filtered list; the glob / unscoped cases delegate to
// git describe (--match <glob> for per-env auto).
func (g *Generator) scopedPreviousTag(tag string) (string, error) {
	if g.cfg.TagGlob == "" && g.cfg.TagPattern != "" {
		tags, err := g.scopedTags()
		if err != nil {
			return "", err
		}
		return previousInList(tag, tags), nil
	}
	return previousTag(g.runner, tag, g.cfg.TagGlob)
}

func (g *Generator) generateReleaseNotes(tag string, lc *port.LinkContext) (string, error) {
	prev, err := g.scopedPreviousTag(tag)
	if err != nil {
		return "", err
	}
	var prevDate time.Time
	if prev != "" {
		if prevDate, err = tagDate(g.runner, prev); err != nil {
			return "", err
		}
	}
	commits, err := collectCommits(g.runner, commitRange(prev, tag))
	if err != nil {
		return "", err
	}
	enrichment, err := g.enrichForRelease(lc, commits)
	if err != nil {
		return "", err
	}
	before, err := authorsBefore(g.runner, prev)
	if err != nil {
		return "", err
	}
	groups := groupCommits(commits, g.cfg.Types, g.cfg.Excludes)
	contributors := collectContributors(toParsedCommits(renderedCommits(commits, groups)), before, enrichment)
	return renderReleaseNotes(tag, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, prevDate, g.cfg.TypesHeadingLevel, enrichment, contributors, g.herautMeta(), g.cfg.EffectiveTemplates, g.cfg.Template)
}

// generateChangelog produces CHANGELOG.md: incrementally, splicing only the new release's
// section into the existing file (preserving history and its attribution verbatim), or as a
// full rebuild when cfg.RegenerateChangelog is set (the --regenerate flag).
func (g *Generator) generateChangelog(tag string, lc *port.LinkContext) (string, error) {
	if g.cfg.RegenerateChangelog {
		body, err := g.buildAllSections(tag, lc, true) // enrich every section
		if err != nil {
			return "", err
		}
		return g.writeChangelog(body)
	}
	return g.generateIncremental(tag, lc)
}

// buildAllSections renders every release section (newest-first), each prefixed with its anchor.
// The newest (unreleased) section is always enriched; historical sections are enriched only when
// enrichAll is true (--regenerate). Matches the pre-incremental full-regen layout plus anchors.
func (g *Generator) buildAllSections(tag string, lc *port.LinkContext, enrichAll bool) (string, error) {
	tags, err := g.scopedTags()
	if err != nil {
		return "", err
	}

	var blocks []string

	latest := ""
	if len(tags) > 0 {
		latest = tags[0]
	}
	if sec, err := g.renderRelease(tag, latest, commitRange(latest, "HEAD"), lc, true); err != nil {
		return "", err
	} else if sec != "" {
		blocks = append(blocks, anchorLine(tag)+"\n"+sec)
	}

	// Existing releases, newest-first. prev is the next-older tag by version refname (listTags
	// is version-sorted); release-notes mode instead resolves prev via git-describe topology.
	// Equivalent for linear history — the common case.
	for i, t := range tags {
		prev := ""
		if i+1 < len(tags) {
			prev = tags[i+1]
		}
		if sec, err := g.renderRelease(t, prev, commitRange(prev, t), lc, enrichAll); err != nil {
			return "", err
		} else if sec != "" {
			blocks = append(blocks, anchorLine(t)+"\n"+sec)
		}
	}

	return changelogHeader + strings.Join(blocks, "\n\n") + "\n", nil
}

// generateIncremental splices only the new release's section into the existing changelog,
// preserving all historical sections (and their attribution). It bootstraps a full build when the
// file is missing/empty, and errors when the file is non-empty but has no anchors.
func (g *Generator) generateIncremental(tag string, lc *port.LinkContext) (string, error) {
	var existing string
	if g.cfg.Output != "" {
		b, err := os.ReadFile(g.cfg.Output)
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("reading changelog %q: %w", g.cfg.Output, err)
		}
		existing = string(b)
	}
	if strings.TrimSpace(existing) == "" {
		body, err := g.buildAllSections(tag, lc, false) // bootstrap: enrich newest only
		if err != nil {
			return "", err
		}
		return g.writeChangelog(body)
	}
	if _, _, hasAnchors := parseChangelog(existing); !hasAnchors {
		// Checked before any rendering/enrichment: a wasted `gh`/`glab` call on a run that will
		// abort anyway, and under remote_metadata: required an enrichment failure would otherwise
		// mask this actionable error (enrichForRelease returns a hard error first).
		return "", g.foreignFileError(ErrNoAnchors)
	}

	tags, err := g.scopedTags()
	if err != nil {
		return "", err
	}
	latest := ""
	if len(tags) > 0 {
		latest = tags[0]
	}
	newBody, err := g.renderRelease(tag, latest, commitRange(latest, "HEAD"), lc, true)
	if err != nil {
		return "", err
	}
	if newBody == "" {
		return existing, nil // nothing new to add; leave the file untouched
	}
	body, err := spliceSection(existing, newBody, tag)
	if errors.Is(err, ErrNoAnchors) {
		return "", g.foreignFileError(err) // defensive: the pre-check above already catches this
	}
	if err != nil {
		return "", err
	}
	return g.writeChangelog(body)
}

// foreignFileError wraps err into the actionable message shown when the changelog at cfg.Output
// is non-empty but has no heraut-release anchors — it was produced by another tool and cannot be
// safely spliced incrementally. Both invoking commands' regenerate flags are named since the
// generator cannot tell whether it was invoked by `heraut changelog` (--regenerate) or
// `heraut release` (--regenerate-changelog).
func (g *Generator) foreignFileError(err error) error {
	return fmt.Errorf("changelog %q has no heraut-release anchors (generated by another tool?); "+
		"rebuild it with anchors and full PR attribution by running `heraut changelog --regenerate` "+
		"(or `heraut release --regenerate-changelog`), after which incremental updates apply: %w",
		g.cfg.Output, err)
}

// writeChangelog writes body to cfg.Output when set and returns it.
func (g *Generator) writeChangelog(body string) (string, error) {
	if g.cfg.Output != "" {
		if err := os.WriteFile(g.cfg.Output, []byte(body), 0o644); err != nil {
			return "", fmt.Errorf("writing changelog %q: %w", g.cfg.Output, err)
		}
	}
	return body, nil
}

// renderRelease renders one changelog release section, or "" when the range has no
// classifiable commits. enrichEnabled gates remote PR enrichment: only the unreleased (newest)
// section is enriched, so a full changelog regeneration costs O(1) API calls rather than one
// fetch per historical release (ADR-0034 §5).
func (g *Generator) renderRelease(version, prev, rng string, lc *port.LinkContext, enrichEnabled bool) (string, error) {
	commits, err := collectCommits(g.runner, rng)
	if err != nil {
		return "", err
	}
	groups := groupCommits(commits, g.cfg.Types, g.cfg.Excludes)
	if len(groups) == 0 {
		return "", nil
	}
	var enrichment map[string]PullRequest
	if enrichEnabled {
		if enrichment, err = g.enrichForRelease(lc, commits); err != nil {
			return "", err
		}
	}
	return renderChangelogSection(version, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, g.cfg.HeadingVersionPattern, g.cfg.TypesHeadingLevel, enrichment, g.herautMeta(), g.cfg.EffectiveTemplates, g.cfg.Template)
}
