package native

import (
	"errors"
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

// herautProjectURL is heraut's public repository, exposed to templates as .Heraut.URL.
const herautProjectURL = "https://github.com/adaouat/heraut"

// Generator is heraut's built-in, zero-external-dependency content generator (ADR-0032). It
// walks git history, classifies commits against the effective commits.types / rendering.excludes
// (propagated onto the ContentDriver by the app layer), and renders Markdown with internal
// templates. With an injected port.Forge (ADR-0043) it enriches commits with PR metadata,
// honouring the remote_metadata policy; Degraded reports an optional fetch failure.
type Generator struct {
	runner         port.Runner
	cfg            *config.ContentDriver
	mode           Mode
	forge          port.Forge
	degraded       bool
	degradedReason string
	now            func() time.Time // injected clock for .Heraut.GeneratedAt; defaults to time.Now
}

var _ port.Generator = (*Generator)(nil)

// New constructs a native Generator.
func New(runner port.Runner, cfg *config.ContentDriver, mode Mode, opts ...Option) *Generator {
	g := &Generator{
		runner: runner,
		cfg:    cfg,
		mode:   mode,
		now:    time.Now,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Option customizes a Generator at construction.
type Option func(*Generator)

// WithForge injects the resolved enrichment forge (ADR-0043) the generator uses to fetch PR/MR
// metadata.
func WithForge(f port.Forge) Option {
	return func(g *Generator) { g.forge = f }
}

// WithDegraded seeds a degraded state at construction time — used when the pipeline could not
// resolve an enrichment forge under a non-required policy (T175): the run proceeds without PR
// attribution, the same outcome as a post-resolution fetch failure under "optional", rather than
// failing outright.
func WithDegraded(reason string) Option {
	return func(g *Generator) {
		g.degraded = true
		g.degradedReason = reason
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

// DegradedReason returns the human-readable reason enrichment was skipped (the underlying fetch
// failure), or "" when the run did not degrade. The pipeline surfaces it as a step sub-result
// rather than writing it to os.Stderr mid-step, where it collided with the live spinner line.
func (g *Generator) DegradedReason() string { return g.degradedReason }

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
	// tag is already a pushed ref by the time release notes are generated (the release pipeline
	// tags and pushes before this step), so it anchors enrichment directly — no HEAD resolution
	// needed (T153).
	er, err := g.enrichForRelease(commits, tag)
	if err != nil {
		return "", err
	}
	before, err := authorsBefore(g.runner, prev)
	if err != nil {
		return "", err
	}
	groups := groupCommits(commits, g.cfg.Types, g.cfg.Excludes)
	overlayAuthorHandles(groups, er.authors)
	contributors := collectContributors(toParsedCommits(renderedCommits(commits, groups)), before, er.prs)
	return renderReleaseNotes(tag, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, prevDate, g.cfg.TypesHeadingLevel, er.prs, contributors, g.herautMeta(), g.cfg.EffectiveTemplates, g.cfg.Template)
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

// newSectionBound returns the previous-tag bound for the newest (unreleased) release section:
// cfg.PreviousTagOverride when set, else the newest of the scoped tags. The override exists for a
// rotating changelog.output (T247): tag-scoping a bucket (e.g. "^2026\.") correctly excludes
// prior-bucket tags from the historical walk below, but that also leaves a brand-new bucket's
// first release with no in-scope previous tag — defaulting to "since the beginning of history"
// and duplicating every prior bucket's entries into the new file. The app layer sets the override
// to the true previous tag (regardless of bucket) to bound it correctly instead.
func (g *Generator) newSectionBound(scopedTags []string) string {
	if g.cfg.PreviousTagOverride != "" {
		return g.cfg.PreviousTagOverride
	}
	if len(scopedTags) > 0 {
		return scopedTags[0]
	}
	return ""
}

// buildAllSections renders every release section (newest-first), each prefixed with its anchor.
// The newest (unreleased) section is always enriched; historical sections are enriched only when
// enrichAll is true (--regenerate). Matches the pre-incremental full-regen layout plus anchors.
func (g *Generator) buildAllSections(tag string, lc *port.LinkContext, enrichAll bool) (string, error) {
	tags, err := g.scopedTags()
	if err != nil {
		return "", err
	}

	preamble, err := renderPreamble(changelogTmpl, g.cfg.EffectiveTemplates, g.cfg.Template, g.herautMeta())
	if err != nil {
		return "", fmt.Errorf("rendering changelog preamble: %w", err)
	}

	var blocks []string

	latest := g.newSectionBound(tags)
	if sec, err := g.renderRelease(tag, latest, "HEAD", lc, true); err != nil {
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
		if sec, err := g.renderRelease(t, prev, t, lc, enrichAll); err != nil {
			return "", err
		} else if sec != "" {
			blocks = append(blocks, anchorLine(t)+"\n"+sec)
		}
	}

	footer, err := renderPostamble(changelogTmpl, g.cfg.EffectiveTemplates, g.cfg.Template, g.herautMeta())
	if err != nil {
		return "", fmt.Errorf("rendering changelog postamble: %w", err)
	}

	body := preamble + strings.Join(blocks, "\n\n") + "\n"
	return appendFooter(body, footer), nil
}

// generateIncremental splices only the new release's section into the existing changelog,
// preserving all historical sections (and their attribution) — but always re-renders the
// preamble/postamble (title/subtitle/footer) fresh from current config, discarding whatever was
// previously on disk (ADR-0050; supersedes the "preamble preserved verbatim" line in ADR-0038). It
// bootstraps a full build when the file is missing/empty, and errors when the file is non-empty
// but has no anchors.
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
	latest := g.newSectionBound(tags)
	newBody, err := g.renderRelease(tag, latest, "HEAD", lc, true)
	if err != nil {
		return "", err
	}
	if newBody == "" {
		return existing, nil // nothing new to add; leave the file untouched
	}
	preamble, err := renderPreamble(changelogTmpl, g.cfg.EffectiveTemplates, g.cfg.Template, g.herautMeta())
	if err != nil {
		return "", fmt.Errorf("rendering changelog preamble: %w", err)
	}
	postamble, err := renderPostamble(changelogTmpl, g.cfg.EffectiveTemplates, g.cfg.Template, g.herautMeta())
	if err != nil {
		return "", fmt.Errorf("rendering changelog postamble: %w", err)
	}
	body, err := spliceSection(existing, newBody, tag, preamble, postamble)
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
// fetch per historical release (ADR-0034 §5). tip is the range's git-resolvable upper bound — a
// tag name for a historical release, or the literal "HEAD" for the unreleased section — used both
// to collect the range's commits and, when enrichment runs, as the enrichment ref (T153).
func (g *Generator) renderRelease(version, prev, tip string, lc *port.LinkContext, enrichEnabled bool) (string, error) {
	commits, err := collectCommits(g.runner, commitRange(prev, tip))
	if err != nil {
		return "", err
	}
	groups := groupCommits(commits, g.cfg.Types, g.cfg.Excludes)
	if len(groups) == 0 {
		return "", nil
	}
	var prs map[string]PullRequest
	if enrichEnabled {
		er, err := g.enrichForRelease(commits, tip)
		if err != nil {
			return "", err
		}
		prs = er.prs
		overlayAuthorHandles(groups, er.authors)
	}
	return renderChangelogSection(version, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, g.cfg.HeadingVersionPattern, g.cfg.TypesHeadingLevel, prs, g.herautMeta(), g.cfg.EffectiveTemplates, g.cfg.Template)
}
